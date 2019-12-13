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

func TestClient_Read_secretManager(t *testing.T) {
	testAcc(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)

		_, err := client.Read(ctx, &SecretManagerReadRequest{
			Project: project,
			Name:    name,
		})
		if !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

	t.Run("latest", func(t *testing.T) {
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

		resp, err := client.Read(ctx, &SecretManagerReadRequest{
			Project: project,
			Name:    name,
		})
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := "1", resp.Version; exp != act {
			t.Errorf("expected version %q to be %q", act, exp)
		}

		if exp, act := plaintext, resp.Plaintext; !bytes.Equal(exp, act) {
			t.Errorf("expected plaintext %q to be %q", act, exp)
		}
	})

	t.Run("version", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		project, name := testProject(t), testName(t)
		plaintext := []byte("my secret plaintext")

		secret, err := client.Create(ctx, &SecretManagerCreateRequest{
			Project:   project,
			Name:      name,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testSecretManagerCleanup(t, project, name)

		resp, err := client.Read(ctx, &SecretManagerReadRequest{
			Project: project,
			Name:    name,
			Version: secret.Version,
		})
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := "1", resp.Version; exp != act {
			t.Errorf("expected version %q to be %q", act, exp)
		}

		if exp, act := plaintext, resp.Plaintext; !bytes.Equal(exp, act) {
			t.Errorf("expected plaintext %q to be %q", act, exp)
		}
	})
}

func TestClient_Read_storage(t *testing.T) {
	testAcc(t)

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object := testBucket(t), testName(t)

		_, err := client.Read(ctx, &StorageReadRequest{
			Bucket: bucket,
			Object: object,
		})
		if !IsSecretDoesNotExistErr(err) {
			t.Errorf("expected %q to be %q", err, errSecretDoesNotExist)
		}
	})

	t.Run("latest", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)
		plaintext := []byte("my secret plaintext")

		secret, err := client.Create(ctx, &StorageCreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testStorageCleanup(t, bucket, object)

		resp, err := client.Read(ctx, &StorageReadRequest{
			Bucket: bucket,
			Object: object,
		})
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := secret.Generation, resp.Generation; exp != act {
			t.Errorf("expected version %q to be %q", act, exp)
		}

		if exp, act := plaintext, resp.Plaintext; !bytes.Equal(exp, act) {
			t.Errorf("expected plaintext %q to be %q", act, exp)
		}
	})

	t.Run("version", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testName(t), testKey(t)
		plaintext := []byte("my secret plaintext")

		secret, err := client.Create(ctx, &StorageCreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testStorageCleanup(t, bucket, object)

		resp, err := client.Read(ctx, &StorageReadRequest{
			Bucket:     bucket,
			Object:     object,
			Generation: secret.Generation,
		})
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := secret.Generation, resp.Generation; exp != act {
			t.Errorf("expected generation %q to be %q", act, exp)
		}

		if exp, act := plaintext, resp.Plaintext; !bytes.Equal(exp, act) {
			t.Errorf("expected plaintext %q to be %q", act, exp)
		}
	})
}
