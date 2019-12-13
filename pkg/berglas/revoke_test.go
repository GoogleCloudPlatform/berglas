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
	"testing"

	"cloud.google.com/go/iam"
)

func TestClient_Revoke_secretManager(t *testing.T) {
	testAcc(t)

	policyIncludesServiceAccount := func(tb testing.TB, h *iam.Handle, serviceAccount string) bool {
		policy, err := getIAMPolicy(context.Background(), h)
		if err != nil {
			t.Fatal(err)
		}

		found := false
		members := policy.Members(iamSecretManagerAccessor)
		for _, member := range members {
			if member == serviceAccount {
				found = true
			}
		}
		return found
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name, serviceAccount := testProject(t), testName(t), testServiceAccount(t)

		if err := client.Revoke(ctx, &SecretManagerRevokeRequest{
			Project: project,
			Name:    name,
			Members: []string{serviceAccount},
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name, serviceAccount := testProject(t), testName(t), testServiceAccount(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &SecretManagerCreateRequest{
			Project:   project,
			Name:      name,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testSecretManagerCleanup(t, project, name)

		if err := client.Grant(ctx, &SecretManagerGrantRequest{
			Project: project,
			Name:    name,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		if !policyIncludesServiceAccount(t, client.secretManagerIAM(project, name), serviceAccount) {
			t.Errorf("expected policy to include %q", serviceAccount)
		}

		if err := client.Revoke(ctx, &SecretManagerRevokeRequest{
			Project: project,
			Name:    name,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		if policyIncludesServiceAccount(t, client.secretManagerIAM(project, name), serviceAccount) {
			t.Errorf("expected policy to not include %q", serviceAccount)
		}
	})
}

func TestClient_Revoke_storage(t *testing.T) {
	testAcc(t)

	policyIncludesServiceAccount := func(tb testing.TB, h *iam.Handle, serviceAccount string) bool {
		policy, err := getIAMPolicy(context.Background(), h)
		if err != nil {
			t.Fatal(err)
		}

		found := false
		members := policy.Members(iamObjectReader)
		for _, member := range members {
			if member == serviceAccount {
				found = true
			}
		}
		return found
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, serviceAccount := testBucket(t), testName(t), testServiceAccount(t)

		if err := client.Revoke(ctx, &StorageRevokeRequest{
			Bucket:  bucket,
			Object:  object,
			Members: []string{serviceAccount},
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key, serviceAccount := testBucket(t), testName(t), testKey(t), testServiceAccount(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &StorageCreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testStorageCleanup(t, bucket, object)

		if err := client.Grant(ctx, &StorageGrantRequest{
			Bucket:  bucket,
			Object:  object,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		if !policyIncludesServiceAccount(t, client.storageIAM(bucket, object), serviceAccount) {
			t.Errorf("expected policy to include %q", serviceAccount)
		}

		if err := client.Revoke(ctx, &StorageRevokeRequest{
			Bucket:  bucket,
			Object:  object,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		if policyIncludesServiceAccount(t, client.storageIAM(bucket, object), serviceAccount) {
			t.Errorf("expected policy to not include %q", serviceAccount)
		}
	})
}
