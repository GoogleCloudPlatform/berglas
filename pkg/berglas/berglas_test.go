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
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	uuid "github.com/satori/go.uuid"
)

func TestBerglasIntegration(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration tests (short)")
	}

	t.Run("access", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		accessPlaintext, err := client.Access(ctx, &AccessRequest{
			Bucket: bucket,
			Object: object,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := accessPlaintext, plaintext; !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		createdSecret, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		readSecret, err := client.Read(ctx, &ReadRequest{
			Bucket:     bucket,
			Object:     object,
			Generation: createdSecret.Generation,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := readSecret.Plaintext, plaintext; !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		if err := client.Delete(ctx, &DeleteRequest{
			Bucket: bucket,
			Object: object,
		}); err != nil {
			t.Fatal(err)
		}

		if _, err := client.Access(ctx, &AccessRequest{
			Bucket: bucket,
			Object: object,
		}); err == nil {
			t.Errorf("expected secret to be deleted")
		}
	})

	t.Run("grant", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key, serviceAccount := testBucket(t), testObject(t), testKey(t), testServiceAccount(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		if err := client.Grant(ctx, &GrantRequest{
			Bucket:  bucket,
			Object:  object,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		handle, err := client.storageIAM(bucket, object)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := getIAMPolicyWithRetries(ctx, handle)
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
		if !found {
			t.Errorf("expected %q to contain %q", members, serviceAccount)
		}
	})

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, key := testBucket(t), testKey(t)
		plaintext := []byte("my secret value")

		object1 := "list-" + testObject(t)
		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object1,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object1)

		secret1, err := client.Read(ctx, &ReadRequest{
			Bucket: bucket,
			Object: object1,
		})
		if err != nil {
			t.Fatal(err)
		}

		object2 := "list-" + testObject(t)
		secret2, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object2,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object2)

		updatedSecret2, err := client.Update(ctx, &UpdateRequest{
			Bucket:    bucket,
			Object:    object2,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}

		list, err := client.List(ctx, &ListRequest{
			Bucket:      bucket,
			Prefix:      "list-",
			Generations: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		secrets := list.Secrets

		if !testSecretsInclude(secrets, secret1) {
			t.Errorf("expected %#v to include %#v", secrets, secret1)
		}

		if !testSecretsInclude(secrets, secret2) {
			t.Errorf("expected %#v to include %#v", secrets, secret2)
		}

		if !testSecretsInclude(secrets, updatedSecret2) {
			t.Errorf("expected %#v to include %#v", secrets, updatedSecret2)
		}
	})

	t.Run("replace", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		ref := fmt.Sprintf("berglas://%s/%s", bucket, object)
		os.Setenv("REPLACE_BAD", "not_a_ref")
		os.Setenv("REPLACE_GOOD", ref)

		if err := client.Replace(ctx, "REPLACE_BAD"); err == nil {
			t.Fatalf("expected error, got %s", os.Getenv("REPLACE_BAD"))
		}
		if act, exp := os.Getenv("REPLACE_BAD"), "not_a_ref"; act != exp {
			t.Errorf("expected %q to be %q", act, exp)
		}

		if err := client.Replace(ctx, "REPLACE_GOOD"); err != nil {
			t.Fatal(err)
		}
		if act, exp := os.Getenv("REPLACE_GOOD"), string(plaintext); act != exp {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("replace_value", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		ref := fmt.Sprintf("berglas://%s/%s", bucket, object)
		os.Setenv("REPLACE_VALUE_GOOD", "should_not_be_read")
		if err := client.ReplaceValue(ctx, "REPLACE_VALUE_GOOD", ref); err != nil {
			t.Fatal(err)
		}
		if act, exp := os.Getenv("REPLACE_VALUE_GOOD"), string(plaintext); act != exp {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)

		createdSecret, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: []byte("my secret value"),
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		updatedSecret, err := client.Update(ctx, &UpdateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: []byte("my new secret value"),
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		if act, exp := createdSecret.Generation, updatedSecret.Generation; act == exp {
			t.Errorf("expected %q to be different than %q", act, exp)
		}

		accessPlaintext, err := client.Access(ctx, &AccessRequest{
			Bucket: bucket,
			Object: object,
		})
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := accessPlaintext, updatedSecret.Plaintext; !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("revoke", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key, serviceAccount := testBucket(t), testObject(t), testKey(t), testServiceAccount(t)
		plaintext := []byte("my secret value")

		if _, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		}); err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		if err := client.Grant(ctx, &GrantRequest{
			Bucket:  bucket,
			Object:  object,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		if err := client.Revoke(ctx, &RevokeRequest{
			Bucket:  bucket,
			Object:  object,
			Members: []string{serviceAccount},
		}); err != nil {
			t.Fatal(err)
		}

		handle, err := client.storageIAM(bucket, object)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := getIAMPolicyWithRetries(ctx, handle)
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
		if found {
			t.Errorf("expected %q to not contain %q", members, serviceAccount)
		}
	})
}

func TestKMSKeyTrimVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		i    string
		o    string
	}{
		{
			"malformed",
			"foo",
			"foo",
		},
		{
			"no_version",
			"projects/p/locations/l/keyRings/kr/cryptoKeys/ck",
			"projects/p/locations/l/keyRings/kr/cryptoKeys/ck",
		},
		{
			"version",
			"projects/p/locations/l/keyRings/kr/cryptoKeys/ck/cryptoKeyVersions/1",
			"projects/p/locations/l/keyRings/kr/cryptoKeys/ck",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if act, exp := kmsKeyTrimVersion(tc.i), tc.o; act != exp {
				t.Errorf("expected %q to be %q", act, exp)
			}
		})
	}
}

func testClient(tb testing.TB) (*Client, context.Context) {
	tb.Helper()

	ctx := context.Background()
	client, err := New(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	return client, ctx
}

func testBucket(tb testing.TB) string {
	tb.Helper()

	bucket := os.Getenv("GOOGLE_CLOUD_BUCKET")
	if bucket == "" {
		tb.Fatal("missing GOOGLE_CLOUD_BUCKET")
	}
	return bucket
}

func testObject(tb testing.TB) string {
	tb.Helper()

	u, err := uuid.NewV4()
	if err != nil {
		tb.Fatal(err)
	}
	return u.String()
}

func testKey(tb testing.TB) string {
	tb.Helper()

	key := os.Getenv("GOOGLE_CLOUD_KMS_KEY")
	if key == "" {
		tb.Fatal("missing GOOGLE_CLOUD_KMS_KEY")
	}
	return key
}

func testServiceAccount(tb testing.TB) string {
	tb.Helper()

	sa := os.Getenv("GOOGLE_CLOUD_SERVICE_ACCOUNT")
	if sa == "" {
		tb.Fatal("missing GOOGLE_CLOUD_SERVICE_ACCOUNT")
	}
	if !strings.HasPrefix("serviceAccount:", sa) {
		sa = fmt.Sprintf("serviceAccount:%s", sa)
	}
	return sa
}

func testSecretsInclude(list []*Secret, s *Secret) bool {
	for _, v := range list {
		if v.Name == s.Name &&
			(v.Plaintext == nil || s.Plaintext == nil || bytes.Equal(v.Plaintext, s.Plaintext)) &&
			(v.Generation == 0 || s.Generation == 0 || v.Generation == s.Generation) {
			return true
		}
	}
	return false
}

func testCleanup(tb testing.TB, bucket, object string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := New(context.Background())
	if err != nil {
		tb.Fatal(err)
	}

	if err := client.Delete(ctx, &DeleteRequest{
		Object: object,
		Bucket: bucket,
	}); err != nil && !IsSecretDoesNotExistErr(err) {
		tb.Fatal(err)
	}
}
