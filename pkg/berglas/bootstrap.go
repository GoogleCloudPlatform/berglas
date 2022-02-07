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
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type bootstrapRequest interface {
	isBootstrapRequest()
}

// StorageBootstrapRequest is used as input to bootstrap Cloud Storage and Cloud
// KMS.
type StorageBootstrapRequest struct {
	// ProjectID is the ID of the project where the bucket should be created.
	ProjectID string

	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// BucketLocation is the location where the bucket should live.
	BucketLocation string

	// KMSLocation is the location where the KMS key ring should live.
	KMSLocation string

	// KMSKeyRing is the name of the KMS key ring.
	KMSKeyRing string

	// KMSCryptoKey is the name of the KMS crypto key.
	KMSCryptoKey string
}

func (r *StorageBootstrapRequest) isBootstrapRequest() {}

// BootstrapRequest is an alias for StorageBootstrapRequest for
// backwards-compatibility. New clients should use StorageBootstrapRequest.
type BootstrapRequest = StorageBootstrapRequest

// SecretManagerBootstrapRequest is used as input to bootstrap Secret Manager.
// This is a noop.
type SecretManagerBootstrapRequest struct{}

func (r *SecretManagerBootstrapRequest) isBootstrapRequest() {}

// Bootstrap is a top-level package that creates a Cloud Storage bucket and
// Cloud KMS key with the proper IAM permissions.
func Bootstrap(ctx context.Context, i bootstrapRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Bootstrap(ctx, i)
}

// Bootstrap adds IAM permission to the given entity to the storage object and the
// underlying KMS key.
func (c *Client) Bootstrap(ctx context.Context, i bootstrapRequest) error {
	if i == nil {
		return fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerBootstrapRequest:
		return c.secretManagerBootstrap(ctx, t)
	case *StorageBootstrapRequest:
		return c.storageBootstrap(ctx, t)
	default:
		return fmt.Errorf("unknown bootstrap type %T", t)
	}
}

func (c *Client) secretManagerBootstrap(ctx context.Context, i *SecretManagerBootstrapRequest) error {
	return nil // noop
}

func (c *Client) storageBootstrap(ctx context.Context, i *StorageBootstrapRequest) error {
	projectID := i.ProjectID
	if projectID == "" {
		return fmt.Errorf("missing project ID")
	}

	bucket := i.Bucket
	if bucket == "" {
		return fmt.Errorf("missing bucket name")
	}

	bucketLocation := strings.ToUpper(i.BucketLocation)
	if bucketLocation == "" {
		bucketLocation = "US"
	}

	kmsLocation := i.KMSLocation
	if kmsLocation == "" {
		kmsLocation = "global"
	}

	kmsKeyRing := i.KMSKeyRing
	if kmsKeyRing == "" {
		kmsKeyRing = "berglas"
	}

	kmsCryptoKey := i.KMSCryptoKey
	if kmsCryptoKey == "" {
		kmsCryptoKey = "berglas-key"
	}

	logger := c.Logger().WithFields(logrus.Fields{
		"project_id":      projectID,
		"bucket":          bucket,
		"bucket_location": bucketLocation,
		"kms_location":    kmsLocation,
		"kms_key_ring":    kmsKeyRing,
		"kms_crypto_key":  kmsCryptoKey,
	})

	logger.Debug("bootstrap.start")
	defer logger.Debug("bootstrap.finish")

	// Create the KMS key ring
	logger.Debug("creating KMS key ring")

	if _, err := c.kmsClient.CreateKeyRing(ctx, &kmspb.CreateKeyRingRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s",
			projectID, kmsLocation),
		KeyRingId: kmsKeyRing,
	}); err != nil {
		logger.WithError(err).Error("failed to create KMS key ring")

		terr, ok := grpcstatus.FromError(err)
		if !ok || terr.Code() != grpccodes.AlreadyExists {
			return fmt.Errorf("failed to create KMS key ring %s: %w", kmsKeyRing, err)
		}
	}

	// Create the KMS crypto key
	logger.Debug("creating KMS crypto key")

	rotationPeriod := 30 * 24 * time.Hour
	if _, err := c.kmsClient.CreateCryptoKey(ctx, &kmspb.CreateCryptoKeyRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s/keyRings/%s",
			projectID, kmsLocation, kmsKeyRing),
		CryptoKeyId: kmsCryptoKey,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ENCRYPT_DECRYPT,
			RotationSchedule: &kmspb.CryptoKey_RotationPeriod{
				RotationPeriod: &durationpb.Duration{
					Seconds: int64(rotationPeriod.Seconds()),
				},
			},
			NextRotationTime: &timestamppb.Timestamp{
				Seconds: time.Now().Add(time.Duration(rotationPeriod)).Unix(),
			},
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				Algorithm:       kmspb.CryptoKeyVersion_GOOGLE_SYMMETRIC_ENCRYPTION,
				ProtectionLevel: kmspb.ProtectionLevel_SOFTWARE,
			},
		},
	}); err != nil {
		logger.WithError(err).Error("failed to create KMS crypto key")

		terr, ok := grpcstatus.FromError(err)
		if !ok || terr.Code() != grpccodes.AlreadyExists {
			return fmt.Errorf("failed to create KMS crypto key %s: %w", kmsCryptoKey, err)
		}
	}

	// Create the storage bucket
	logger.Debug("creating bucket")

	if err := c.storageClient.Bucket(bucket).Create(ctx, projectID, &storage.BucketAttrs{
		PredefinedACL:              "private",
		PredefinedDefaultObjectACL: "private",
		Location:                   bucketLocation,
		VersioningEnabled:          true,
		Lifecycle: storage.Lifecycle{
			Rules: []storage.LifecycleRule{
				{
					Action: storage.LifecycleAction{
						Type: "Delete",
					},
					Condition: storage.LifecycleCondition{
						NumNewerVersions: 10,
					},
				},
			},
		},
		Labels: map[string]string{
			"purpose": "berglas",
		},
	}); err != nil {
		logger.WithError(err).Error("failed to create bucket")

		if !isBucketAlreadyExistsError(err) {
			return fmt.Errorf("failed to create storage bucket %s: %w", bucket, err)
		}
	}

	return nil
}

// isBucketAlreadyExistsError returns true if the given error corresponds to the
// error that occurs when a bucket already exists.
func isBucketAlreadyExistsError(err error) bool {
	terr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	return terr.Code == 409 && strings.Contains(terr.Message, "You already own this bucket")
}
