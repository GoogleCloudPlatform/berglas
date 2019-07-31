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

	"github.com/pkg/errors"
)

// Create is a top-level package function for creating a secret. For large
// volumes of secrets, please create a client instead.
func Create(ctx context.Context, i *CreateRequest) (*Secret, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Create(ctx, i)
}

// CreateRequest is used as input to a create a secret.
type CreateRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Key is the fully qualified KMS key id.
	Key string

	// Plaintext is the plaintext secret to encrypt and store.
	Plaintext []byte
}

// Create creates a new encrypted secret on GCS. If the secret already exists,
// an error is returned. Use Update to update an existing secret.
func (c *Client) Create(ctx context.Context, i *CreateRequest) (*Secret, error) {
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

	key := i.Key
	if key == "" {
		return nil, errors.New("missing key name")
	}

	plaintext := i.Plaintext
	if plaintext == nil {
		return nil, errors.New("missing plaintext")
	}

	secret, err := c.encryptAndWrite(ctx, bucket, object, key, plaintext, 0, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secret")
	}
	return secret, nil
}
