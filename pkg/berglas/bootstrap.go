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
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// Bootstrap is a top-level package that creates a Cloud Storage bucket and
// Cloud KMS key with the proper IAM permissions.
func Bootstrap(ctx context.Context, i *BootstrapRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Bootstrap(ctx, i)
}

// BootstrapRequest is used as input to a bootstrap a berglas setup.
type BootstrapRequest struct {
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

// Bootstrap adds IAM permission to the given entity to the storage object and the
// underlying KMS key.
func (c *Client) Bootstrap(ctx context.Context, i *BootstrapRequest) error {
	if i == nil {
		return errors.New("missing request")
	}

	projectID := i.ProjectID
	if projectID == "" {
		return errors.New("missing project ID")
	}

	bucket := i.Bucket
	if bucket == "" {
		return errors.New("missing bucket name")
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

	// Attempt to create the KMS key ring
	if _, err := c.kmsClient.CreateKeyRing(ctx, &kmspb.CreateKeyRingRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s",
			projectID, kmsLocation),
		KeyRingId: kmsKeyRing,
	}); err != nil {
		terr, ok := grpcstatus.FromError(err)
		if !ok || terr.Code() != grpccodes.AlreadyExists {
			return errors.Wrapf(err, "failed to create KMS key ring %s", kmsKeyRing)
		}
	}

	// Attempt to create the KMS crypto key
	rotationPeriod := 30 * 24 * time.Hour
	if _, err := c.kmsClient.CreateCryptoKey(ctx, &kmspb.CreateCryptoKeyRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s/keyRings/%s",
			projectID, kmsLocation, kmsKeyRing),
		CryptoKeyId: kmsCryptoKey,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ENCRYPT_DECRYPT,
			RotationSchedule: &kmspb.CryptoKey_RotationPeriod{
				RotationPeriod: &duration.Duration{
					Seconds: int64(rotationPeriod.Seconds()),
				},
			},
			NextRotationTime: &timestamp.Timestamp{
				Seconds: time.Now().Add(time.Duration(rotationPeriod)).Unix(),
			},
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				Algorithm:       kmspb.CryptoKeyVersion_GOOGLE_SYMMETRIC_ENCRYPTION,
				ProtectionLevel: kmspb.ProtectionLevel_SOFTWARE,
			},
		},
	}); err != nil {
		terr, ok := grpcstatus.FromError(err)
		if !ok || terr.Code() != grpccodes.AlreadyExists {
			return errors.Wrapf(err, "failed to create KMS crypto key %s", kmsCryptoKey)
		}
	}

	// Create the storage bucket
	if err := c.storageClient.Bucket(bucket).Create(ctx, projectID, &storage.BucketAttrs{
		PredefinedACL:              "private",
		PredefinedDefaultObjectACL: "private",
		Location:                   bucketLocation,
		VersioningEnabled:          true,
		Lifecycle: storage.Lifecycle{
			Rules: []storage.LifecycleRule{
				storage.LifecycleRule{
					Action: storage.LifecycleAction{
						Type: "Delete",
					},
					Condition: storage.LifecycleCondition{
						NumNewerVersions: 10,
					},
				},
				storage.LifecycleRule{
					Action: storage.LifecycleAction{
						Type: "Delete",
					},
					Condition: storage.LifecycleCondition{
						AgeInDays: 30,
						Liveness:  storage.Archived,
					},
				},
			},
		},
		Labels: map[string]string{
			"purpose": "berglas",
		},
	}); err != nil {
		if isBucketAlreadyExistsError(err) {
			err = errors.New("bucket already exists")
		}
		return errors.Wrapf(err, "failed to create storage bucket %s", bucket)
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
