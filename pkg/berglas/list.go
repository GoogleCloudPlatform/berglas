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
	"cloud.google.com/go/storage"
	"context"
	"sort"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

// Secret represents a specific secret stored in Google Cloud Storage
// The attributes on this object should ideally map 1:1 with storage.ObjectAttrs
type Secret struct {
	// Name of the secret
	Name string

	// UpdatedAt indicates when a secret was last updated
	UpdatedAt time.Time

	// Generation indicates a secret's version
	Generation int64
}

// SecretSlice is a list of secrets
type SecretSlice []Secret

// Len is the number of elements in the collection.
func (s SecretSlice) Len() int {
	return len(s)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (s SecretSlice) Less(i, j int) bool {
	if s[i].Name == s[j].Name {
		return s[i].Generation > s[j].Generation
	}
	return s[i].Name > s[j].Name
}

// Swap swaps the elements with indexes i and j.
func (s SecretSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// List is a top-level package function for listing secrets.
func List(ctx context.Context, i *ListRequest) (SecretSlice, error) {
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

	// Prefix matches secret names to filter
	Prefix string

	// Generations indicates that all generations of secrets should be listed
	Generations bool
}

// List lists all secrets in the bucket.
func (c *Client) List(ctx context.Context, i *ListRequest) (SecretSlice, error) {
	if i == nil {
		return nil, errors.New("missing request")
	}

	bucket := i.Bucket
	if bucket == "" {
		return nil, errors.New("missing bucket name")
	}

	var result SecretSlice

	query := &storage.Query{
		Prefix:   i.Prefix,
		Versions: i.Generations,
	}

	// List all objects
	it := c.storageClient.
		Bucket(bucket).
		Objects(ctx, query)
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
			secret := Secret{
				Name:       obj.Name,
				UpdatedAt:  obj.Updated,
				Generation: obj.Generation,
			}
			result = append(result, secret)
		}
	}

	sort.Sort(result)

	return result, nil
}
