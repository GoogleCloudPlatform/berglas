// Copyright 2019 The Berglas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package berglas

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	secretspb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type readRequest interface {
	isReadRequest()
}

// StorageReadRequest is used as input to read a secret from Cloud Storage
// encrypted with Cloud KMS.
type StorageReadRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Generation of the object to fetch.
	Generation int64
}

func (r *StorageReadRequest) isReadRequest() {}

// ReadRequest is an alias for StorageReadRequest for backwards-compatibility.
// New clients should use StorageReadRequest.
type ReadRequest = StorageReadRequest

// SecretManagerReadRequest is used as input to read a secret from Secret
// Manager.
type SecretManagerReadRequest struct {
	// Project is the ID or number of the project from which to read secrets.
	Project string

	// Name is the name of the secret to read.
	Name string

	// Version is the version of the secret to read.
	Version string
}

func (r *SecretManagerReadRequest) isReadRequest() {}

// Read is a top-level package function for reading an entire secret object. It
// returns attributes about the secret object, including the plaintext.
func Read(ctx context.Context, i readRequest) (*Secret, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Read(ctx, i)
}

// Read read a secret. When given a SecretManagerReadRequest, this reads a
// secret from Secret Manager. When given a StorageReadRequest, this reads a
// secret stored in Cloud Storage.
func (c *Client) Read(ctx context.Context, i readRequest) (*Secret, error) {
	if i == nil {
		return nil, fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerReadRequest:
		return c.secretManagerRead(ctx, t)
	case *StorageReadRequest:
		return c.storageRead(ctx, t)
	default:
		return nil, fmt.Errorf("unknown read type %T", t)
	}
}

func (c *Client) secretManagerRead(ctx context.Context, i *SecretManagerReadRequest) (*Secret, error) {
	project := i.Project
	if project == "" {
		return nil, fmt.Errorf("missing project")
	}

	name := i.Name
	if name == "" {
		return nil, fmt.Errorf("missing secret name")
	}

	version := i.Version
	if version == "" {
		version = "latest"
	}

	logger := c.Logger().WithFields(logrus.Fields{
		"project": project,
		"name":    name,
		"version": version,
	})

	logger.Debug("read.start")
	defer logger.Debug("read.finish")

	logger.Debug("reading secret version")

	versionResp, err := c.secretManagerClient.GetSecretVersion(ctx, &secretspb.GetSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, name, version),
	})
	if err != nil {
		terr, ok := grpcstatus.FromError(err)
		if ok && terr.Code() == grpccodes.NotFound {
			return nil, errSecretDoesNotExist
		}
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	logger.Debug("accessing secret data")

	accessResp, err := c.secretManagerClient.AccessSecretVersion(ctx, &secretspb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, name, version),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access secret: %w", err)
	}

	logger.Debug("parsing location")

	var locations []string
	replication := versionResp.ReplicationStatus.GetUserManaged()
	if replication != nil {
		locations = make([]string, len(replication.Replicas))
		for i, r := range replication.Replicas {
			locations[i] = r.Location
		}
	}
	sort.Strings(locations)

	return &Secret{
		Parent:    project,
		Name:      name,
		Version:   path.Base(versionResp.Name),
		Plaintext: accessResp.Payload.Data,
		UpdatedAt: timestampToTime(versionResp.CreateTime),
		Locations: locations,
	}, nil
}

func (c *Client) storageRead(ctx context.Context, i *StorageReadRequest) (*Secret, error) {
	bucket := i.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return nil, fmt.Errorf("missing object name")
	}

	generation := i.Generation
	if generation == 0 {
		generation = -1
	}

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket":     bucket,
		"object":     object,
		"generation": generation,
	})

	logger.Debug("read.start")
	defer logger.Debug("read.finish")

	// Get attributes to find the KMS key
	logger.Debug("reading attributes from storage")

	attrs, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Generation(generation).
		Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, errSecretDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read secret metadata: %w", err)
	}
	if attrs.Metadata == nil || attrs.Metadata[MetadataKMSKey] == "" {
		return nil, fmt.Errorf("missing kms key in secret metadata")
	}
	key := attrs.Metadata[MetadataKMSKey]

	logger = logger.WithField("key", key)
	logger.Debug("found kms key")

	// Download the file from GCS
	logger.Debug("downloading file from storage")

	ior, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Generation(generation).
		NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, fmt.Errorf("secret object not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	// Read the entire response into memory
	logger.Debug("reading object into memory")

	data, err := ioutil.ReadAll(ior)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret into string: %w", err)
	}
	if err := ior.Close(); err != nil {
		return nil, fmt.Errorf("failed to close reader: %w", err)
	}

	// Split into parts
	logger.Debug("deconstructing and decoding ciphertext into parts")

	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid ciphertext: not enough parts")
	}

	encDEK, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext: failed to parse dek")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext: failed to parse ciphertext")
	}

	// Decrypt the DEK using a KMS key
	logger.Debug("decrypting dek using kms")

	kmsResp, err := c.kmsClient.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        key,
		Ciphertext:                  encDEK,
		AdditionalAuthenticatedData: []byte(object),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt dek: %w", err)
	}
	dek := kmsResp.Plaintext

	// Decrypt with the local key
	logger.Debug("decrypting data with deck locally")

	plaintext, err := envelopeDecrypt(dek, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt envelope: %w", err)
	}
	return secretFromAttrs(bucket, attrs, plaintext), nil
}
