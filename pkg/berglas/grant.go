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

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	sort.Strings(members)

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket":  bucket,
		"object":  object,
		"members": members,
	})

	logger.Debug("grant.start")
	defer logger.Debug("grant.finish")

	// Get attributes to find the KMS key
	logger.Debug("finding storage object")

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

	logger = logger.WithField("key", key)
	logger.Debug("found kms key")

	// Grant access to storage
	logger.Debug("granting access to storage")

	storageHandle := c.storageIAM(bucket, object)
	if err := updateIAMPolicy(ctx, storageHandle, func(p *iam.Policy) *iam.Policy {
		for _, m := range members {
			p.Add(m, iamObjectReader)
		}
		return p
	}); err != nil {
		return errors.Wrapf(err, "failed to update Storage IAM policy for %s", object)
	}

	// Grant access to KMS
	logger.Debug("granting access to kms")

	kmsHandle := c.kmsClient.ResourceIAM(key)
	if err := updateIAMPolicy(ctx, kmsHandle, func(p *iam.Policy) *iam.Policy {
		for _, m := range members {
			p.Add(m, iamKMSDecrypt)
		}
		return p
	}); err != nil {
		return errors.Wrapf(err, "failed to update KMS IAM policy for %s", key)
	}

	return nil
}
