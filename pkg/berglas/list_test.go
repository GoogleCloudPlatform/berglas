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

func TestClient_List_secretManager(t *testing.T) {
	testAcc(t)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, prefix := testProject(t), testName(t)

		list, err := client.List(ctx, &SecretManagerListRequest{
			Project: project,
			Prefix:  prefix,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(list.Secrets) > 0 {
			t.Errorf("expected no secrets, got %#v", list.Secrets)
		}
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, prefix := testProject(t), testName(t)

		for i := 0; i < 3; i++ {
			name := prefix + "-" + testName(t)
			if _, err := client.Create(ctx, &SecretManagerCreateRequest{
				Project:   project,
				Name:      name,
				Plaintext: []byte("test"),
			}); err != nil {
				t.Fatal(err)
			}
			defer testSecretManagerCleanup(t, project, name)
		}

		list, err := client.List(ctx, &SecretManagerListRequest{
			Project: project,
			Prefix:  prefix,
		})
		if err != nil {
			t.Fatal(err)
		}

		if d := len(list.Secrets); d != 3 {
			t.Errorf("expected 3 secrets, got %d: %#v", d, list.Secrets)
		}
	})

	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)

		if _, err := client.Create(ctx, &SecretManagerCreateRequest{
			Project:   project,
			Name:      name,
			Plaintext: []byte("test"),
		}); err != nil {
			t.Fatal(err)
		}
		defer testSecretManagerCleanup(t, project, name)

		for i := 0; i < 3; i++ {
			if _, err := client.Update(ctx, &SecretManagerUpdateRequest{
				Project:   project,
				Name:      name,
				Plaintext: []byte("test"),
			}); err != nil {
				t.Fatal(err)
			}
		}

		list, err := client.List(ctx, &SecretManagerListRequest{
			Project:  project,
			Prefix:   name,
			Versions: true,
		})
		if err != nil {
			t.Fatal(err)
		}

		if d := len(list.Secrets); d != 4 { // 4 because create creates the first version
			t.Errorf("expected 3 secrets, got %d: %#v", d, list.Secrets)
		}
	})
}

func TestClient_List_storage(t *testing.T) {
	testAcc(t)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, prefix := testBucket(t), testName(t)

		list, err := client.List(ctx, &StorageListRequest{
			Bucket: bucket,
			Prefix: prefix,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(list.Secrets) > 0 {
			t.Errorf("expected no secrets, got %#v", list.Secrets)
		}
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, prefix, key := testBucket(t), testName(t), testKey(t)

		for i := 0; i < 3; i++ {
			object := prefix + "-" + testName(t)
			if _, err := client.Create(ctx, &StorageCreateRequest{
				Bucket:    bucket,
				Object:    object,
				Key:       key,
				Plaintext: []byte("test"),
			}); err != nil {
				t.Fatal(err)
			}
			defer testStorageCleanup(t, bucket, object)
		}

		list, err := client.List(ctx, &StorageListRequest{
			Bucket: bucket,
			Prefix: prefix,
		})
		if err != nil {
			t.Fatal(err)
		}

		if d := len(list.Secrets); d != 3 {
			t.Errorf("expected 3 secrets, got %d: %#v", d, list.Secrets)
		}
	})

	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)

		if _, err := client.Create(ctx, &StorageCreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: []byte("test"),
		}); err != nil {
			t.Fatal(err)
		}
		defer testStorageCleanup(t, bucket, object)

		for i := 0; i < 3; i++ {
			if _, err := client.Update(ctx, &StorageUpdateRequest{
				Bucket:    bucket,
				Object:    object,
				Key:       key,
				Plaintext: []byte("test"),
			}); err != nil {
				t.Fatal(err)
			}
		}

		list, err := client.List(ctx, &StorageListRequest{
			Bucket:      bucket,
			Prefix:      object,
			Generations: true,
		})
		if err != nil {
			t.Fatal(err)
		}

		if d := len(list.Secrets); d != 4 { // 4 because create creates the first version
			t.Errorf("expected 3 secrets, got %d: %#v", d, list.Secrets)
		}
	})
}
