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

import "testing"

func TestClient_Delete_secretManager(t *testing.T) {
	testAcc(t)

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &SecretManagerCreateRequest{
			Project:   project,
			Name:      name,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testSecretManagerCleanup(t, project, name)

		if err := client.Delete(ctx, &SecretManagerDeleteRequest{
			Project: project,
			Name:    name,
		}); err != nil {
			t.Fatal(err)
		}

		if _, err := client.Access(ctx, &SecretManagerAccessRequest{
			Project: project,
			Name:    name,
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)

		if err := client.Delete(ctx, &SecretManagerDeleteRequest{
			Project: project,
			Name:    name,
		}); err != nil {
			t.Fatal(err)
		}

		if _, err := client.Access(ctx, &SecretManagerAccessRequest{
			Project: project,
			Name:    name,
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})
}

func TestClient_Delete_storage(t *testing.T) {
	testAcc(t)

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)
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

		if err := client.Delete(ctx, &StorageDeleteRequest{
			Bucket: bucket,
			Object: object,
		}); err != nil {
			t.Fatal(err)
		}

		if _, err := client.Access(ctx, &StorageAccessRequest{
			Bucket: bucket,
			Object: object,
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object := testBucket(t), testName(t)

		if err := client.Delete(ctx, &StorageDeleteRequest{
			Bucket: bucket,
			Object: object,
		}); err != nil {
			t.Fatal(err)
		}

		if _, err := client.Access(ctx, &StorageAccessRequest{
			Bucket: bucket,
			Object: object,
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})
}
