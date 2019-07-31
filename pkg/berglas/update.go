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

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
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

	// If no specific generations were given, lookup the latest generation to make
	// sure we don't conflict with another write.
	attrs, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Attrs(ctx)
	switch err {
	case nil:
		if generation == 0 {
			generation = attrs.Generation
		}

		if metageneration == 0 {
			metageneration = attrs.Metageneration
		}

		if key == "" {
			key = attrs.Metadata[MetadataKMSKey]
		}

		if plaintext == nil {
			plaintext, err = c.Access(ctx, &AccessRequest{
				Bucket:     bucket,
				Object:     object,
				Generation: generation,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to get plaintext")
			}
		}

		// Get existing IAM policies.
		storageHandle, err := c.storageIAM(bucket, object)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create IAM client")
		}
		storageP, err := getIAMPolicyWithRetries(ctx, storageHandle)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get IAM policy")
		}

		// Update the secret.
		secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext,
			generation, metageneration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to update secret")
		}

		// Copy over the existing IAM memberships, if any.
		if err := setIAMPolicyWithRetries(ctx, storageHandle, storageP); err != nil {
			return nil, errors.Wrap(err, "secret updated, but failed to update IAM")
		}
		return secret, nil
	case storage.ErrObjectNotExist:
		if !i.CreateIfMissing {
			return nil, errSecretDoesNotExist
		}

		if key == "" {
			return nil, errors.New("missing key name")
		}

		if plaintext == nil {
			return nil, errors.New("missing plaintext")
		}

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
