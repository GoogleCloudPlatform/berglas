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
	"path"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	secretspb "google.golang.org/genproto/googleapis/cloud/secrets/v1beta1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type createRequest interface {
	isCreateRequest()
}

// StorageCreateRequest is used as input to create a secret using Cloud Storage
// encrypted with Cloud KMS.
type StorageCreateRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Key is the fully qualified KMS key id.
	Key string

	// Plaintext is the plaintext secret to encrypt and store.
	Plaintext []byte
}

func (r *StorageCreateRequest) isCreateRequest() {}

// CreateRequest is an alias for StorageCreateRequest for
// backwards-compatability. New clients should use StorageCreateRequest.
type CreateRequest = StorageCreateRequest

// SecretManagerCreateRequest is used as input to create a secret using Secret
// Manager.
type SecretManagerCreateRequest struct {
	// Project is the ID or number of the project from which to create the secret.
	Project string

	// Name is the name of the secret to create.
	Name string

	// Plaintext is the plaintext to store.
	Plaintext []byte
}

func (r *SecretManagerCreateRequest) isCreateRequest() {}

// Create is a top-level package function for creating a secret. For large
// volumes of secrets, please create a client instead.
func Create(ctx context.Context, i createRequest) (*Secret, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Create(ctx, i)
}

// Create creates a secret. When given a SecretManagerCreateRequest, this
// creates a secret using Secret Manager. When given a StorageCreateRequest,
// this creates a secret stored in Cloud Storage encrypted with Cloud KMS.
//
// If the secret already exists, an error is returned. Use Update to update an
// existing secret.
func (c *Client) Create(ctx context.Context, i createRequest) (*Secret, error) {
	if i == nil {
		return nil, errors.New("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerCreateRequest:
		return c.secretManagerCreate(ctx, t)
	case *StorageCreateRequest:
		return c.storageCreate(ctx, t)
	default:
		return nil, errors.Errorf("unknown create type %T", t)
	}
}

func (c *Client) secretManagerCreate(ctx context.Context, i *SecretManagerCreateRequest) (*Secret, error) {
	project := i.Project
	if project == "" {
		return nil, errors.New("missing project")
	}

	name := i.Name
	if name == "" {
		return nil, errors.New("missing secret name")
	}

	plaintext := i.Plaintext
	if plaintext == nil {
		return nil, errors.New("missing plaintext")
	}

	logger := c.Logger().WithFields(logrus.Fields{
		"project": project,
		"name":    name,
	})

	logger.Debug("create.start")
	defer logger.Debug("create.finish")

	logger.Debug("creating secret")

	secretResp, err := c.secretManagerClient.CreateSecret(ctx, &secretspb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", project),
		SecretId: name,
		Secret: &secretspb.Secret{
			Replication: &secretspb.Replication{
				Replication: &secretspb.Replication_Automatic_{
					Automatic: &secretspb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		terr, ok := grpcstatus.FromError(err)
		if ok && terr.Code() == grpccodes.AlreadyExists {
			return nil, errSecretAlreadyExists
		}
		return nil, errors.Wrapf(err, "failed to create secret")
	}

	logger.Debug("creating secret version")

	versionResp, err := c.secretManagerClient.AddSecretVersion(ctx, &secretspb.AddSecretVersionRequest{
		Parent: secretResp.Name,
		Payload: &secretspb.SecretPayload{
			Data: plaintext,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secret version")
	}

	return &Secret{
		Parent:    project,
		Name:      name,
		Version:   path.Base(versionResp.Name),
		Plaintext: plaintext,
		UpdatedAt: timestampToTime(versionResp.CreateTime),
	}, nil
}

func (c *Client) storageCreate(ctx context.Context, i *StorageCreateRequest) (*Secret, error) {
	bucket := i.Bucket
	if bucket == "" {
		return nil, errors.New("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return nil, errors.New("missing object name")
	}

	key := i.Key
	if key == "" {
		return nil, errors.New("missing key name")
	}

	plaintext := i.Plaintext
	if plaintext == nil {
		return nil, errors.New("missing plaintext")
	}

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket": bucket,
		"object": object,
		"key":    key,
	})

	logger.Debug("create.start")
	defer logger.Debug("create.finish")

	secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext, 0, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secret")
	}
	return secret, nil
}
