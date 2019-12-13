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
	"bytes"
	"testing"
)

func TestClient_Update_secretManager(t *testing.T) {
	testAcc(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)
		plaintext := []byte("my secret plaintext")

		if _, err := client.Update(ctx, &SecretManagerUpdateRequest{
			Project:   project,
			Name:      name,
			Plaintext: plaintext,
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
		defer testSecretManagerCleanup(t, project, name)
	})

	t.Run("missing_create_if_missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)
		plaintext := []byte("my secret plaintext")

		if _, err := client.Update(ctx, &SecretManagerUpdateRequest{
			Project:         project,
			Name:            name,
			Plaintext:       plaintext,
			CreateIfMissing: true,
		}); err != nil {
			t.Fatal(err)
		}
		defer testSecretManagerCleanup(t, project, name)
	})

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)
		plaintext := []byte("my secret plaintext")

		if _, err := client.Create(ctx, &SecretManagerCreateRequest{
			Project:   project,
			Name:      name,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testSecretManagerCleanup(t, project, name)

		plaintext2 := []byte("my new secret plaintext")
		updateResp, err := client.Update(ctx, &SecretManagerUpdateRequest{
			Project:   project,
			Name:      name,
			Plaintext: plaintext2,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := updateResp.Version, "2"; act != exp {
			t.Errorf("expected version %q to be %q", act, exp)
		}

		if act, exp := updateResp.Plaintext, plaintext2; !bytes.Equal(act, exp) {
			t.Errorf("expected plaintext %q to be %q", act, exp)
		}
	})
}

func TestClient_Update_storage(t *testing.T) {
	testAcc(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)
		plaintext := []byte("my secret plaintext")

		if _, err := client.Update(ctx, &StorageUpdateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
		defer testStorageCleanup(t, bucket, object)
	})

	t.Run("missing_create_if_missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)
		plaintext := []byte("my secret plaintext")

		if _, err := client.Update(ctx, &StorageUpdateRequest{
			Bucket:          bucket,
			Object:          object,
			Key:             key,
			Plaintext:       plaintext,
			CreateIfMissing: true,
		}); err != nil {
			t.Fatal(err)
		}
		defer testStorageCleanup(t, bucket, object)
	})

	t.Run("exists", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)
		plaintext := []byte("my secret plaintext")

		createResp, err := client.Create(ctx, &StorageCreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testStorageCleanup(t, bucket, object)

		plaintext2 := []byte("my new secret plaintext")
		updateResp, err := client.Update(ctx, &StorageUpdateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext2,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := updateResp.Generation, createResp.Generation; act == exp {
			t.Errorf("expected version %q to be changed", act)
		}

		if act, exp := updateResp.Plaintext, plaintext2; !bytes.Equal(act, exp) {
			t.Errorf("expected plaintext %q to be %q", act, exp)
		}
	})
}
