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
	"fmt"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	secretspb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

type listRequest interface {
	isListRequest()
}

// StorageListRequest is used as input to list secrets from Cloud Storage.
type StorageListRequest struct {
	// Bucket is the name of the bucket where the secrets live.
	Bucket string

	// Prefix matches secret names to filter.
	Prefix string

	// Generations indicates that all generations of secrets should be listed.
	Generations bool
}

func (r *StorageListRequest) isListRequest() {}

// ListRequest is an alias for StorageListRequest for backwards-compatibility.
// New clients should use StorageListRequest.
type ListRequest = StorageListRequest

// SecretManagerListRequest is used as input to list secrets from Secret
// Manager.
type SecretManagerListRequest struct {
	// Project is the ID or number of the project from which to list secrets.
	Project string

	// Prefix matches secret names to filter.
	Prefix string

	// Versions indicates that all versions of secrets should be listed.
	Versions bool
}

func (r *SecretManagerListRequest) isListRequest() {}

// ListResponse is the response from a list call.
type ListResponse struct {
	// Secrets are the list of secrets in the response.
	Secrets []*Secret
}

// secretList is a list of secrets
type secretList []*Secret

// Len is the number of elements in the collection.
func (s secretList) Len() int {
	return len(s)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (s secretList) Less(i, j int) bool {
	if s[i].Name == s[j].Name {
		if s[i].Generation == s[j].Generation {
			return s[i].Version > s[j].Version
		}
		return s[i].Generation > s[j].Generation
	}
	return s[i].Name > s[j].Name
}

// Swap swaps the elements with indexes i and j.
func (s secretList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// List is a top-level package function for listing secrets. This doesn't
// fetch the plaintext value of secrets.
func List(ctx context.Context, i listRequest) (*ListResponse, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.List(ctx, i)
}

// List lists all secrets in the bucket. This doesn't fetch the plaintext value
// of secrets.
func (c *Client) List(ctx context.Context, i listRequest) (*ListResponse, error) {
	if i == nil {
		return nil, fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerListRequest:
		return c.secretManagerList(ctx, t)
	case *StorageListRequest:
		return c.storageList(ctx, t)
	default:
		return nil, fmt.Errorf("unknown list type %T", t)
	}
}

func (c *Client) secretManagerList(ctx context.Context, i *SecretManagerListRequest) (*ListResponse, error) {
	project := i.Project
	if project == "" {
		return nil, fmt.Errorf("missing project")
	}

	prefix := i.Prefix
	versions := i.Versions

	logger := c.Logger().WithFields(logrus.Fields{
		"project":  project,
		"prefix":   prefix,
		"versions": versions,
	})

	logger.Debug("list.start")
	defer logger.Debug("list.finish")

	allSecrets := []*Secret{}

	it := c.secretManagerClient.ListSecrets(ctx, &secretspb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", project),
	})
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			logger.Debug("out of secrets")
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		if strings.HasPrefix(path.Base(resp.Name), prefix) {
			allSecrets = append(allSecrets, &Secret{
				Parent:    project,
				Name:      path.Base(resp.Name),
				UpdatedAt: timestampToTime(resp.CreateTime),
			})
		}
	}

	if !versions {
		sort.Sort(secretList(allSecrets))
		return &ListResponse{
			Secrets: allSecrets,
		}, nil
	}

	allSecretVersions := make([]*Secret, 0, len(allSecrets)*2)

	for _, s := range allSecrets {
		logger = logger.WithFields(logrus.Fields{
			"project": s.Parent,
			"name":    s.Name,
		})
		logger.Debug("listing secret versions")

		it := c.secretManagerClient.ListSecretVersions(ctx, &secretspb.ListSecretVersionsRequest{
			Parent: fmt.Sprintf("projects/%s/secrets/%s", s.Parent, s.Name),
		})
		for {
			resp, err := it.Next()
			if err == iterator.Done {
				logger.Debug("out of versions")
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to list versions for %s: %w", s.Name, err)
			}

			allSecretVersions = append(allSecretVersions, &Secret{
				Parent:    project,
				Name:      s.Name,
				Version:   path.Base(resp.Name),
				UpdatedAt: timestampToTime(resp.CreateTime),
			})
		}
	}

	sort.Sort(secretList(allSecretVersions))

	return &ListResponse{
		Secrets: allSecretVersions,
	}, nil
}

func (c *Client) storageList(ctx context.Context, i *StorageListRequest) (*ListResponse, error) {
	bucket := i.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("missing bucket name")
	}

	prefix := i.Prefix
	generations := i.Generations

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket":      bucket,
		"prefix":      prefix,
		"generations": generations,
	})

	logger.Debug("list.start")
	defer logger.Debug("list.finish")

	allObjects := map[string][]*storage.ObjectAttrs{}

	query := &storage.Query{
		Prefix:   prefix,
		Versions: generations,
	}

	// List all objects
	it := c.storageClient.
		Bucket(bucket).
		Objects(ctx, query)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			logger.Debug("out of objects")
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		// Check that it has metadata
		if obj.Metadata == nil || obj.Metadata[MetadataIDKey] != "1" {
			logger.WithFields(logrus.Fields{
				"object":   obj.Name,
				"metadata": obj.Metadata,
			}).Debug("found object without metadata")
			continue
		}

		logger.WithField("object", obj.Name).Debug("adding object to list")
		allObjects[obj.Name] = append(allObjects[obj.Name], obj)
	}

	var result secretList

	// list objects returns all generations even if the live object is gone.
	// filter on names which have not been deleted
	logger.Debug("filtering objects with no live versions")

	for _, objects := range allObjects {
		foundLiveObject := false
		for _, obj := range objects {
			if obj.Deleted.IsZero() {
				foundLiveObject = true
				break
			}
		}

		if foundLiveObject {
			for _, obj := range objects {
				result = append(result, secretFromAttrs(bucket, obj, nil))
			}
		}
	}

	sort.Sort(result)

	return &ListResponse{
		Secrets: result,
	}, nil
}
