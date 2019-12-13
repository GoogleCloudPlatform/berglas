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

func TestClient_Access_secretManager(t *testing.T) {
	testAcc(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)

		_, err := client.Access(ctx, &SecretManagerAccessRequest{
			Project: project,
			Name:    name,
		})
		if !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
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

		resp, err := client.Access(ctx, &SecretManagerAccessRequest{
			Project: project,
			Name:    name,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := resp, plaintext; !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})
}

func TestClient_Access_storage(t *testing.T) {
	testAcc(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object := testBucket(t), testName(t)

		_, err := client.Access(ctx, &StorageAccessRequest{
			Bucket: bucket,
			Object: object,
		})
		if !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

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

		resp, err := client.Access(ctx, &StorageAccessRequest{
			Bucket: bucket,
			Object: object,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := resp, plaintext; !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})
}
