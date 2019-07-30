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
	"io/ioutil"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// Read is a top-level package function for reading an entire secret object. It
// returns attributes about the secret object, including the plaintext.
func Read(ctx context.Context, i *ReadRequest) (*Secret, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Read(ctx, i)
}

// ReadRequest is used as input to a get secret request.
type ReadRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Generation of the object to fetch
	Generation int64
}

// Read reads the contents of the secret from the bucket.
func (c *Client) Read(ctx context.Context, i *ReadRequest) (*Secret, error) {
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
	if generation == 0 {
		generation = -1
	}

	// Get attributes to find the KMS key
	attrs, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Generation(generation).
		Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, errSecretDoesNotExist
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to read secret metadata")
	}
	if attrs.Metadata == nil || attrs.Metadata[MetadataKMSKey] == "" {
		return nil, errors.New("missing kms key in secret metadata")
	}
	key := attrs.Metadata[MetadataKMSKey]

	// Download the file from GCS
	ior, err := c.storageClient.
		Bucket(bucket).
		Object(object).
		Generation(generation).
		NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, errors.New("secret object not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to read secret")
	}

	// Read the entire response into memory
	data, err := ioutil.ReadAll(ior)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read secret into string")
	}
	if err := ior.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close reader")
	}

	// Split into parts
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) < 2 {
		return nil, errors.New("invalid ciphertext: not enough parts")
	}

	encDEK, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid ciphertext: failed to parse dek")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid ciphertext: failed to parse ciphertext")
	}

	// Decrypt the DEK using a KMS key
	kmsResp, err := c.kmsClient.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:                        key,
		Ciphertext:                  encDEK,
		AdditionalAuthenticatedData: []byte(object),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt dek")
	}
	dek := kmsResp.Plaintext

	// Decrypt with the local key
	plaintext, err := envelopeDecrypt(dek, ciphertext)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt envelope")
	}
	return secretFromAttrs(attrs, plaintext), nil
}
