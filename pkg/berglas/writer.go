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
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
)

var emptyCondition = storage.Conditions{}

func (c *Client) write(
	ctx context.Context, bucket, object, key, blob string, conds *storage.Conditions, plaintext []byte,
	preconditionFailureError string) (*Secret, error) {
	oh := c.storageClient.
		Bucket(bucket).
		Object(object)
	// Write the object with CAS
	if conds != nil && *conds != emptyCondition {
		oh = oh.If(*conds)
	}
	iow := oh.NewWriter(ctx)
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
			case http.StatusNotFound:
				return nil, errors.New("bucket does not exist")
			case http.StatusPreconditionFailed:
				return nil, errors.New(preconditionFailureError)
			}
		}

		return nil, errors.Wrap(err, "failed to close writer")
	}

	return &Secret{
		Name:           iow.Attrs().Name,
		Generation:     iow.Attrs().Generation,
		KMSKey:         iow.Attrs().Metadata[MetadataKMSKey],
		Metageneration: iow.Attrs().Metageneration,
		Plaintext:      plaintext,
		UpdatedAt:      iow.Attrs().Updated,
	}, nil
}
