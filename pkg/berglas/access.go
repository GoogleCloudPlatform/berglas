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
	"fmt"

	"github.com/sirupsen/logrus"

	secretspb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type accessRequest interface {
	isAccessRequest()
}

// StorageAccessRequest is used as input to access a secret from Cloud Storage
// encrypted with Cloud KMS.
type StorageAccessRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Generation of the object to fetch
	Generation int64
}

func (r *StorageAccessRequest) isAccessRequest() {}

// AccessRequest is an alias for StorageAccessRequest for
// backwards-compatibility. New clients should use StorageAccessRequest.
type AccessRequest = StorageAccessRequest

// SecretManagerAccessRequest is used as input to access a secret from Secret
// Manager.
type SecretManagerAccessRequest struct {
	// Project is the ID or number of the project from which to access secrets.
	Project string

	// Name is the name of the secret to access.
	Name string

	// Version is the version of the secret to access.
	Version string
}

func (r *SecretManagerAccessRequest) isAccessRequest() {}

// Access is a top-level package function for accessing a secret. For large
// volumes of secrets, please create a client instead.
func Access(ctx context.Context, i accessRequest) ([]byte, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Access(ctx, i)
}

// Access accesses a secret. When given a SecretManagerAccessRequest, this
// accesses a secret from Secret Manager. When given a StorageAccessRequest,
// this accesses a secret stored in Cloud Storage encrypted with Cloud KMS.
func (c *Client) Access(ctx context.Context, i accessRequest) ([]byte, error) {
	if i == nil {
		return nil, fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerAccessRequest:
		return c.secretManagerAccess(ctx, t)
	case *StorageAccessRequest:
		return c.storageAccess(ctx, t)
	default:
		return nil, fmt.Errorf("unknown access type %T", t)
	}
}

func (c *Client) secretManagerAccess(ctx context.Context, i *SecretManagerAccessRequest) ([]byte, error) {
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

	logger.Debug("access.start")
	defer logger.Debug("access.finish")

	resp, err := c.secretManagerClient.AccessSecretVersion(ctx, &secretspb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, name, version),
	})
	if err != nil {
		terr, ok := grpcstatus.FromError(err)
		if ok && terr.Code() == grpccodes.NotFound {
			return nil, errSecretDoesNotExist
		}
		return nil, fmt.Errorf("failed to access secret: %w", err)
	}

	return resp.Payload.Data, nil
}

func (c *Client) storageAccess(ctx context.Context, i *StorageAccessRequest) ([]byte, error) {
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

	logger.Debug("access.start")
	defer logger.Debug("access.finish")

	secret, err := c.Read(ctx, &ReadRequest{
		Bucket:     bucket,
		Object:     object,
		Generation: generation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to access secret: %w", err)
	}
	return secret.Plaintext, nil
}
