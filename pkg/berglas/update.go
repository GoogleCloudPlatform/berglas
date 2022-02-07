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

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	secretspb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type updateRequest interface {
	isUpdateRequest()
}

// StorageUpdateRequest is used as input to update a secret from Cloud Storage
// encrypted with Cloud KMS.
type StorageUpdateRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Generation indicates a secret's version.
	Generation int64

	// Key is the fully qualified KMS key id.
	Key string

	// Metageneration indicates a secret's metageneration.
	Metageneration int64

	// Plaintext value of the secret.
	Plaintext []byte

	// CreateIfMissing indicates that the updater should create a secret with the
	// given parameters if one does not already exist.
	CreateIfMissing bool
}

func (r *StorageUpdateRequest) isUpdateRequest() {}

// UpdateRequest is an alias for StorageUpdateRequest for
// backwards-compatibility. New clients should use StorageUpdateRequest.
type UpdateRequest = StorageUpdateRequest

// SecretManagerUpdateRequest is used as input to update a secret using Secret Manager.
type SecretManagerUpdateRequest struct {
	// Project is the ID or number of the project from which to update the secret.
	Project string

	// Name is the name of the secret to update.
	Name string

	// Plaintext is the plaintext to store.
	Plaintext []byte

	// CreateIfMissing indicates that the updater should create a secret with the
	// given parameters if one does not already exist.
	CreateIfMissing bool
}

func (r *SecretManagerUpdateRequest) isUpdateRequest() {}

// Update is a top-level package function for updating a secret. For large
// volumes of secrets, please update a client instead.
func Update(ctx context.Context, i updateRequest) (*Secret, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Update(ctx, i)
}

// Update updates a secret. When given a SecretManagerUpdateRequest, this
// updates a secret in Secret Manager. When given a StorageUpdateRequest, this
// updates a secret stored in Cloud Storage encrypted with Cloud KMS.
func (c *Client) Update(ctx context.Context, i updateRequest) (*Secret, error) {
	if i == nil {
		return nil, fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerUpdateRequest:
		return c.secretManagerUpdate(ctx, t)
	case *StorageUpdateRequest:
		return c.storageUpdate(ctx, t)
	default:
		return nil, fmt.Errorf("unknown update type %T", t)
	}
}

func (c *Client) secretManagerUpdate(ctx context.Context, i *SecretManagerUpdateRequest) (*Secret, error) {
	project := i.Project
	if project == "" {
		return nil, fmt.Errorf("missing project")
	}

	name := i.Name
	if name == "" {
		return nil, fmt.Errorf("missing secret name")
	}

	plaintext := i.Plaintext
	if plaintext == nil {
		return nil, fmt.Errorf("missing plaintext")
	}

	createIfMissing := i.CreateIfMissing

	logger := c.Logger().WithFields(logrus.Fields{
		"project":           project,
		"name":              name,
		"create_if_missing": createIfMissing,
	})

	logger.Debug("update.start")
	defer logger.Debug("update.finish")

	logger.Debug("reading existing secret")

	secretResp, err := c.secretManagerClient.GetSecret(ctx, &secretspb.GetSecretRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s", project, name),
	})
	if err != nil {
		terr, ok := grpcstatus.FromError(err)
		if !ok || terr.Code() != grpccodes.NotFound {
			return nil, fmt.Errorf("failed to read secret for updating: %w", err)
		}

		logger.Debug("secret does not exist")

		if !createIfMissing {
			return nil, errSecretDoesNotExist
		}

		logger.Debug("creating secret")

		secretResp, err = c.secretManagerClient.CreateSecret(ctx, &secretspb.CreateSecretRequest{
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
			if !ok || terr.Code() != grpccodes.AlreadyExists {
				return nil, fmt.Errorf("failed to create secret: %w", err)
			}
		}
	}

	logger.Debug("creating secret version")

	versionResp, err := c.secretManagerClient.AddSecretVersion(ctx, &secretspb.AddSecretVersionRequest{
		Parent: secretResp.Name,
		Payload: &secretspb.SecretPayload{
			Data: plaintext,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create secret version: %w", err)
	}

	return &Secret{
		Parent:    project,
		Name:      name,
		Version:   path.Base(versionResp.Name),
		Plaintext: plaintext,
		UpdatedAt: timestampToTime(versionResp.CreateTime),
	}, nil
}

func (c *Client) storageUpdate(ctx context.Context, i *StorageUpdateRequest) (*Secret, error) {
	bucket := i.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return nil, fmt.Errorf("missing object name")
	}

	// Key and Plaintext may be required depending on whether the object exists.
	key := i.Key
	plaintext := i.Plaintext

	generation := i.Generation
	metageneration := i.Metageneration
	createIfMissing := i.CreateIfMissing

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket":            bucket,
		"object":            object,
		"key":               key,
		"generation":        generation,
		"metageneration":    metageneration,
		"create_if_missing": createIfMissing,
	})

	logger.Debug("update.start")
	defer logger.Debug("update.finish")

	// If no specific generations were given, lookup the latest generation to make
	// sure we don't conflict with another write.
	attrs, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Attrs(ctx)
	switch err {
	case nil:
		logger = logger.WithFields(logrus.Fields{
			"existing.bucket":         attrs.Bucket,
			"existing.name":           attrs.Name,
			"existing.size":           attrs.Size,
			"existing.metadata":       attrs.Metadata,
			"existing.generation":     attrs.Generation,
			"existing.metageneration": attrs.Metageneration,
			"existing.created":        attrs.Created,
			"existing.deleted":        attrs.Deleted,
			"existing.updated":        attrs.Updated,
		})
		logger.Debug("found existing storage object")

		if generation == 0 {
			generation = attrs.Generation
			logger = logger.WithField("generation", generation)
			logger.Debug("setting generation")
		}

		if metageneration == 0 {
			metageneration = attrs.Metageneration
			logger = logger.WithField("metageneration", metageneration)
			logger.Debug("setting metageneration")
		}

		if key == "" {
			key = attrs.Metadata[MetadataKMSKey]
			logger = logger.WithField("key", key)
			logger.Debug("setting key")
		}

		if plaintext == nil {
			logger.Debug("attempting to access plaintext")

			plaintext, err = c.Access(ctx, &AccessRequest{
				Bucket:     bucket,
				Object:     object,
				Generation: generation,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get plaintext: %w", err)
			}
		}

		// Get existing IAM policies
		logger.Debug("getting iam policies")

		storageHandle := c.storageIAM(bucket, object)
		storageP, err := getIAMPolicy(ctx, storageHandle)
		if err != nil {
			return nil, fmt.Errorf("failed to get IAM policy: %w", err)
		}

		// Update the secret
		logger.Debug("updating secret")

		secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext,
			generation, metageneration)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret: %w", err)
		}

		// Copy over the existing IAM memberships, if any
		logger.Debug("updating iam policies")

		if err := updateIAMPolicy(ctx, storageHandle, func(p *iam.Policy) *iam.Policy {
			// Copy any IAM permissions from the old object over to the new object.
			for _, m := range storageP.Members(iamObjectReader) {
				p.Add(m, iamObjectReader)
			}
			return p
		}); err != nil {
			return nil, fmt.Errorf("failed to update Storage IAM policy for %s: %w", object, err)
		}
		return secret, nil
	case storage.ErrObjectNotExist:
		logger.Debug("secret does not exist")

		if !createIfMissing {
			return nil, errSecretDoesNotExist
		}

		if key == "" {
			return nil, fmt.Errorf("missing key name")
		}

		if plaintext == nil {
			return nil, fmt.Errorf("missing plaintext")
		}

		logger.Debug("creating secret")

		// Update the secret.
		secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext,
			generation, metageneration)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret: %w", err)
		}
		return secret, nil
	default:
		return nil, fmt.Errorf("failed to fetch existing secret: %w", err)
	}
}
