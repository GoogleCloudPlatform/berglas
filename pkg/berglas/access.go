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

// Access is a top-level package function for accessing a secret. For large
// volumes of secrets, please create a client instead.
func Access(ctx context.Context, i *AccessRequest) (*Secret, error) {
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

var doesNotExistError = "secret object not found"

// IsDoesNotExist returns true if the error returned by this package indicates
// that a secret does not exist
func IsDoesNotExist(err error) bool {
	return errors.Cause(err).Error() == "secret object not found"
}

// Access reads the contents of the secret from the bucket, decrypting the
// ciphertext using Cloud KMS.
func (c *Client) Access(ctx context.Context, i *AccessRequest) (*Secret, error) {
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

	// Get attributes to find the KMS key
	h := c.storageClient.
		Bucket(bucket).
		Object(object)
	if i.Generation != 0 {
		h = h.Generation(i.Generation)
	}
	attrs, err := h.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, errors.New(doesNotExistError)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to read secret metadata")
	}
	if attrs.Metadata == nil || attrs.Metadata[MetadataKMSKey] == "" {
		return nil, errors.New("missing kms key in secret metadata")
	}
	key := attrs.Metadata[MetadataKMSKey]

	// Download the file from GCS
	ior, err := h.NewReader(ctx)
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
		return nil, errors.Wrap(err, "xxx")
	}
	return &Secret{
		Name:           attrs.Name,
		Generation:     attrs.Generation,
		KMSKey:         key,
		Metageneration: attrs.Metageneration,
		Plaintext:      plaintext,
		UpdatedAt:      attrs.Updated,
	}, nil
}
