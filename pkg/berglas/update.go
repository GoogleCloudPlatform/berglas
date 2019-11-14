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

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Update is a top-level package function for creating a secret. For large
// volumes of secrets, please update a client instead.
func Update(ctx context.Context, i *UpdateRequest) (*Secret, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Update(ctx, i)
}

// UpdateRequest is used as input to a update a secret.
type UpdateRequest struct {
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

	// Plaintext value of the secret (may not be filled in)
	Plaintext []byte

	// CreateIfMissing indicates that the updater should create a secret with the
	// given parameters if one does not already exist.
	CreateIfMissing bool
}

// Update changes the contents of an existing secret. If the secret does not
// exist, an error is returned.
func (c *Client) Update(ctx context.Context, i *UpdateRequest) (*Secret, error) {
	if i == nil {
		return nil, errors.New("missing request")
	}

	bucket := i.Bucket
	if bucket == "" {
		return nil, errors.New("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return nil, errors.New("missing object name")
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
				return nil, errors.Wrap(err, "failed to get plaintext")
			}
		}

		// Get existing IAM policies
		logger.Debug("getting iam policies")

		storageHandle := c.storageIAM(bucket, object)
		storageP, err := getIAMPolicy(ctx, storageHandle)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get IAM policy")
		}

		// Update the secret
		logger.Debug("updating secret")

		secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext,
			generation, metageneration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to update secret")
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
			return nil, errors.Wrapf(err, "failed to update Storage IAM policy for %s", object)
		}
		return secret, nil
	case storage.ErrObjectNotExist:
		logger.Debug("secret does not exist")

		if !createIfMissing {
			return nil, errSecretDoesNotExist
		}

		if key == "" {
			return nil, errors.New("missing key name")
		}

		if plaintext == nil {
			return nil, errors.New("missing plaintext")
		}

		logger.Debug("creating secret")

		// Update the secret.
		secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext,
			generation, metageneration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to update secret")
		}
		return secret, nil
	default:
		return nil, errors.Wrap(err, "failed to fetch existing secret")
	}
}
