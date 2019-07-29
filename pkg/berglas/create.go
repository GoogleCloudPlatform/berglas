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

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
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

	// Overwrite the existing secret
	Overwrite bool
}

var alreadyExistsError = "secret already exists"

// Create reads the contents of the secret from the bucket, decrypting the
// ciphertext using Cloud KMS.
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

	_, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Attrs(ctx)
	switch err {
	case nil:
		if !i.Overwrite {
			return nil, errors.New(alreadyExistsError)
		}
	case storage.ErrObjectNotExist:
		break
	default:
		return nil, errors.Wrap(err, "failed to get object")
	}

	// Generate a unique DEK and encrypt the plaintext locally (useful for large
	// pieces of data).
	dek, ciphertext, err := envelopeEncrypt(plaintext)
	if err != nil {
		return nil, errors.Wrap(err, "failed to perform envelope encryption")
	}

	// Encrypt the plaintext using a KMS key
	kmsResp, err := c.kmsClient.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        key,
		Plaintext:                   dek,
		AdditionalAuthenticatedData: []byte(object),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt secret")
	}
	encDEK := kmsResp.Ciphertext

	// Build the storage object contents. Contents will be of the format:
	//
	//    b64(kms_encrypted_dek):b64(dek_encrypted_plaintext)
	blob := fmt.Sprintf("%s:%s",
		base64.StdEncoding.EncodeToString(encDEK),
		base64.StdEncoding.EncodeToString(ciphertext))

	conds := &storage.Conditions{
		DoesNotExist: !i.Overwrite,
	}

	return c.write(ctx, bucket, object, key, blob, conds, plaintext, alreadyExistsError)
}
