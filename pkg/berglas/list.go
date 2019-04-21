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
	"sort"

	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

// List is a top-level package function for listing secrets.
func List(ctx context.Context, i *ListRequest) ([]string, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.List(ctx, i)
}

// ListRequest is used as input to a list all secrets in a bucket.
type ListRequest struct {
	// Bucket is the name of the bucket where the secrets live.
	Bucket string
}

// List lists all secrets in the bucket.
func (c *Client) List(ctx context.Context, i *ListRequest) ([]string, error) {
	if i == nil {
		return nil, errors.New("missing request")
	}

	bucket := i.Bucket
	if bucket == "" {
		return nil, errors.New("missing bucket name")
	}

	var result []string

	// List all objects
	it := c.storageClient.
		Bucket(bucket).
		Objects(ctx, nil)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to list secrets")
		}

		// Only include items with metadata marking them as a secret
		if obj.Metadata != nil && obj.Metadata[MetadataIDKey] == "1" {
			result = append(result, obj.Name)
		}
	}

	sort.Strings(result)

	return result, nil
}
