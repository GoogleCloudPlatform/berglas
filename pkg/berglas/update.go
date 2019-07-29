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

	// Generation indicates a secret's version
	Generation int64

	// Key is the fully qualified KMS key id.
	Key string

	// Metageneration indicates a secret's metageneration
	Metageneration int64

	// Plaintext value of the secret (may not be filled in)
	Plaintext []byte
}

var secretUpdatedError = "secret modified between read and write"

// Update reads the contents of the secret from the bucket, decrypting the
// ciphertext using Cloud KMS.
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

	generation := i.Generation
	if generation <= 0 {
		return nil, errors.New("missing secret generation")
	}

	metageneration := i.Metageneration
	if metageneration <= 0 {
		return nil, errors.New("missing secret metageneration")
	}

	key := i.Key
	if key == "" {
		return nil, errors.New("missing key name")
	}

	plaintext := i.Plaintext
	if plaintext == nil {
		return nil, errors.New("missing plaintext")
	}

	attrs, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Attrs(ctx)
	switch err {
	case nil:
		if attrs.Generation != generation || attrs.Metageneration != metageneration {
			return nil, errors.New(secretUpdatedError)
		}
	case storage.ErrObjectNotExist:
		return nil, errors.Wrap(err, "secret does not exist")
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
		GenerationMatch:     generation,
		MetagenerationMatch: metageneration,
	}

	return c.write(ctx, bucket, object, key, blob, conds, plaintext, secretUpdatedError)
}
