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

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// Grant is a top-level package function for granting access to a secret. For
// large volumes of secrets, please create a client instead.
func Grant(ctx context.Context, i *GrantRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Grant(ctx, i)
}

// GrantRequest is used as input to a grant secret request.
type GrantRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Members is the list of membership bindings. This should be in the format
	// described at https://godoc.org/google.golang.org/api/iam/v1#Binding.
	Members []string
}

// Grant adds IAM permission to the given entity to the storage object and the
// underlying KMS key.
func (c *Client) Grant(ctx context.Context, i *GrantRequest) error {
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

	// Grant access to storage
	storageHandle, err := iamHandle(bucket, object)
	if err != nil {
		return errors.Wrap(err, "failed to create Storage IAM client")
	}

	storageP, err := storageHandle.Policy(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get Storage IAM policy")
	}

	for _, m := range members {
		storageP.Add(m, iamObjectReader)
	}

	if err := storageHandle.SetPolicy(ctx, storageP); err != nil {
		return errors.Wrapf(err, "failed to update Storage IAM policy for %s", object)
	}

	// Grant access to KMS
	kmsHandle := c.kmsClient.CryptoKeyIAM(&kmspb.CryptoKey{Name: key})
	kmsP, err := kmsHandle.Policy(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get KMS IAM policy")
	}

	// Add members to the policy
	for _, m := range members {
		kmsP.Add(m, iamKMSDecrypt)
	}

	// Save the policy
	if err := kmsHandle.SetPolicy(ctx, kmsP); err != nil {
		return errors.Wrapf(err, "failed to update KMS IAM policy for %s", key)
	}

	return nil
}
