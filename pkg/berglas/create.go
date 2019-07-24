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
	"google.golang.org/api/googleapi"
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
}

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

	// Attempt to get the object first to build the CAS parameters
	var conds storage.Conditions
	attrs, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Attrs(ctx)
	switch err {
	case nil:
		conds.GenerationMatch = attrs.Generation
		conds.MetagenerationMatch = attrs.Metageneration
	case storage.ErrObjectNotExist:
		conds.DoesNotExist = true
	default:
		return nil, errors.Wrap(err, "failed to get object")
	}

	// Write the object with CAS
	iow := c.storageClient.
		Bucket(bucket).
		Object(object).
		If(conds).
		NewWriter(ctx)
	iow.ObjectAttrs.CacheControl = CacheControl
	iow.ChunkSize = 1024

	if iow.Metadata == nil {
		iow.Metadata = make(map[string]string)
	}

	// Mark this as a secret
	iow.Metadata[MetadataIDKey] = "1"

	// If a specific key version was given, only store the key, not the key
	// version, because decrypt calls can't specify a key version.
	iow.Metadata[MetadataKMSKey] = kmsKeyTrimVersion(key)

	if _, err := iow.Write([]byte(blob)); err != nil {
		return nil, errors.Wrap(err, "failed save encrypted ciphertext to storage")
	}

	// Close, handling any errors
	if err := iow.Close(); err != nil {
		if terr, ok := err.(*googleapi.Error); ok {
			switch terr.Code {
			case 404:
				return nil, errors.New("bucket does not exist")
			case 412:
				return nil, errors.New("secret modified between read and write")
			}
		}

		return nil, errors.Wrap(err, "failed to close writer")
	}

	return &Secret{
		Name:       iow.Attrs().Name,
		Generation: iow.Attrs().Generation,
		UpdatedAt:  iow.Attrs().Updated,
	}, nil
}
