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
	"sort"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type grantRequest interface {
	isGrantRequest()
}

// StorageGrantRequest is used as input to grant access to secrets backed Cloud
// Storage encrypted with Cloud KMS.
type StorageGrantRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the object in Cloud Storage.
	Object string

	// Members is the list of membership bindings. This should be in the format
	// described at https://godoc.org/google.golang.org/api/iam/v1#Binding.
	Members []string
}

func (r *StorageGrantRequest) isGrantRequest() {}

// GrantRequest is an alias for StorageGrantRequest for
// backwards-compatibility. New clients should use StorageGrantRequest.
type GrantRequest = StorageGrantRequest

// SecretManagerGrantRequest is used as input to grant access to a secret in
// Secret Manager.
type SecretManagerGrantRequest struct {
	// Project is the ID or number of the project where secrets live.
	Project string

	// Name is the name of the secret to access.
	Name string

	// Members is the list of membership bindings. This should be in the format
	// described at https://godoc.org/google.golang.org/api/iam/v1#Binding.
	Members []string
}

func (r *SecretManagerGrantRequest) isGrantRequest() {}

// Grant is a top-level package function for granting access to a secret. For
// large volumes of secrets, please create a client instead.
func Grant(ctx context.Context, i grantRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Grant(ctx, i)
}

// Grant adds IAM permission to the given entity to the storage object and the
// underlying KMS key.
func (c *Client) Grant(ctx context.Context, i grantRequest) error {
	if i == nil {
		return fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerGrantRequest:
		return c.secretManagerGrant(ctx, t)
	case *StorageGrantRequest:
		return c.storageGrant(ctx, t)
	default:
		return fmt.Errorf("unknown grant type %T", t)
	}
}

func (c *Client) secretManagerGrant(ctx context.Context, i *SecretManagerGrantRequest) error {
	project := i.Project
	if project == "" {
		return fmt.Errorf("missing project")
	}

	name := i.Name
	if name == "" {
		return fmt.Errorf("missing secret name")
	}

	members := i.Members
	if len(members) == 0 {
		return nil
	}
	sort.Strings(members)

	logger := c.Logger().WithFields(logrus.Fields{
		"project": project,
		"name":    name,
		"members": members,
	})

	logger.Debug("grant.start")
	defer logger.Debug("grant.finish")

	logger.Debug("granting access to secret")

	storageHandle := c.secretManagerIAM(project, name)
	if err := updateIAMPolicy(ctx, storageHandle, func(p *iam.Policy) *iam.Policy {
		for _, m := range members {
			p.Add(m, iamSecretManagerAccessor)
		}
		return p
	}); err != nil {
		terr, ok := grpcstatus.FromError(err)
		if ok && terr.Code() == grpccodes.NotFound {
			return errSecretDoesNotExist
		}

		return fmt.Errorf("failed to update Secret Manager IAM policy for %s: %w", name, err)
	}

	return nil
}

func (c *Client) storageGrant(ctx context.Context, i *StorageGrantRequest) error {
	bucket := i.Bucket
	if bucket == "" {
		return fmt.Errorf("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return fmt.Errorf("missing object name")
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
		return errSecretDoesNotExist
	}
	if err != nil {
		return fmt.Errorf("failed to read secret metadata: %w", err)
	}
	if attrs.Metadata == nil || attrs.Metadata[MetadataKMSKey] == "" {
		return fmt.Errorf("missing kms key in secret metadata")
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
		return fmt.Errorf("failed to update Storage IAM policy for %s: %w", object, err)
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
		return fmt.Errorf("failed to update KMS IAM policy for %s: %w", key, err)
	}

	return nil
}
