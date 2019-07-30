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

// Access is a top-level package function for accessing a secret. For large
// volumes of secrets, please create a client instead.
func Access(ctx context.Context, i *AccessRequest) ([]byte, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Access(ctx, i)
}

// AccessRequest is used as input to a get secret request.
type AccessRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Generation of the object to fetch
	Generation int64
}

// Access reads the contents of the secret from the bucket, decrypting the
// ciphertext using Cloud KMS.
func (c *Client) Access(ctx context.Context, i *AccessRequest) ([]byte, error) {
	secret, err := c.Read(ctx, &ReadRequest{
		Bucket:     i.Bucket,
		Object:     i.Object,
		Generation: i.Generation,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to access secret")
	}
	return secret.Plaintext, nil
}
