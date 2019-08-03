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

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
)

// Revoke is a top-level package function for revokeing access to a secret. For
// large volumes of secrets, please create a client instead.
func Revoke(ctx context.Context, i *RevokeRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Revoke(ctx, i)
}

// RevokeRequest is used as input to a revoke secret request.
type RevokeRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Members is the list of membership bindings. This should be in the format
	// described at https://godoc.org/google.golang.org/api/iam/v1#Binding.
	Members []string
}

// Revoke removes IAM permission to the given entity on the storage object and
// the underlying KMS key.
func (c *Client) Revoke(ctx context.Context, i *RevokeRequest) error {
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

	members := i.Members
	if len(members) == 0 {
		return nil
	}

	// Get attributes to find the KMS key
	objHandle := c.storageClient.Bucket(bucket).Object(object)
	attrs, err := objHandle.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return errors.New("secret object not found")
	}
	if err != nil {
		return errors.Wrap(err, "failed to read secret metadata")
	}
	if attrs.Metadata == nil || attrs.Metadata[MetadataKMSKey] == "" {
		return errors.New("missing kms key in secret metadata")
	}
	key := attrs.Metadata[MetadataKMSKey]

	// Remove access to storage
	storageHandle := c.storageIAM(bucket, object)
	if err := updateIAMPolicy(ctx, storageHandle, func(p *iam.Policy) *iam.Policy {
		for _, m := range members {
			p.Remove(m, iamObjectReader)
		}
		return p
	}); err != nil {
		return errors.Wrapf(err, "failed to update Storage IAM policy for %s", object)
	}

	// Remove access to KMS
	kmsHandle := c.kmsClient.ResourceIAM(key)
	if err := updateIAMPolicy(ctx, kmsHandle, func(p *iam.Policy) *iam.Policy {
		for _, m := range members {
			p.Remove(m, iamKMSDecrypt)
		}
		return p
	}); err != nil {
		return errors.Wrapf(err, "failed to update KMS IAM policy for %s", key)
	}

	return nil
}
