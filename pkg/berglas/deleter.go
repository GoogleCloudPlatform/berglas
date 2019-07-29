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

	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
)

// Delete is a top-level package function for creating a secret. For large
// volumes of secrets, please create a client instead.
func Delete(ctx context.Context, i *DeleteRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Delete(ctx, i)
}

// DeleteRequest is used as input to a get secret request.
type DeleteRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the secret in Cloud Storage.
	Object string

	// Permanently delete the secret (instead of archiving)
	Permanently bool
}

// Delete reads the contents of the secret from the bucket, decrypting the
// ciphertext using Cloud KMS.
func (c *Client) Delete(ctx context.Context, i *DeleteRequest) error {
	if i == nil {
		return errors.New("missing request")
	}

	bucket := i.Bucket
	if bucket == "" {
		return errors.New("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return errors.New("missing object name")
	}

	if i.Permanently {
		it := c.storageClient.
			Bucket(bucket).
			Objects(ctx, &storage.Query{
				Prefix:   object,
				Versions: true,
			})

		for {
			obj, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return errors.Wrap(err, "failed to list secrets")
			}
			if err := c.storageClient.
				Bucket(bucket).
				Object(object).
				Generation(obj.Generation).
				Delete(ctx); err != nil {
				return errors.Wrap(err, "failed to delete")
			}
		}
		return nil
	}

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
		return nil
	default:
		return errors.Wrap(err, "failed to get object")
	}

	// Delete the object with CAS
	if err := c.storageClient.
		Bucket(bucket).
		Object(object).
		If(conds).
		Delete(ctx); err != nil {
		if terr, ok := err.(*googleapi.Error); ok && terr.Code == 412 {
			return errors.New("secret modified between read and delete")
		}
		return errors.Wrap(err, "failed to delete")
	}

	return nil
}
