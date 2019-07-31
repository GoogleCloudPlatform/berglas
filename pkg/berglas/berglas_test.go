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
	"testing"

	uuid "github.com/satori/go.uuid"
)

func TestBerglasIntegration(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test (short)")
	}

	ctx := context.Background()

	bucket := os.Getenv("GOOGLE_CLOUD_BUCKET")
	if bucket == "" {
		t.Fatal("missing GOOGLE_CLOUD_BUCKET")
	}

	key := os.Getenv("GOOGLE_CLOUD_KMS_KEY")
	if key == "" {
		t.Fatal("missing GOOGLE_CLOUD_KMS_KEY")
	}

	sa := os.Getenv("GOOGLE_CLOUD_SERVICE_ACCOUNT")
	if sa == "" {
		t.Fatal("missing GOOGLE_CLOUD_SERVICE_ACCOUNT")
	}
	sa = fmt.Sprintf("serviceAccount:%s", sa)

	object, object2 := testUUID(t), testUUID(t)
	if len(object) < 3 || len(object2) < 3 {
		t.Fatal("bad uuid created")
	}
	// ensure non-matching prefix
	for i := 0; i < 10 && object[:3] == object2[:3]; i++ {
		object2 = testUUID(t)
	}
	if object[:3] == object2[:3] {
		t.Fatal("unable to generate non-prefix matching uuids")
	}

	c, err := New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	original := []byte("original text")
	updated := []byte("updated text")

	var secret *Secret
	var plaintext []byte

	if secret, err = c.Create(ctx, &CreateRequest{
		Bucket:    bucket,
		Object:    object,
		Key:       key,
		Plaintext: original,
	}); err != nil {
		t.Fatal(err)
	}

	if err := c.Grant(ctx, &GrantRequest{
		Bucket:  bucket,
		Object:  object,
		Members: []string{sa},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Access(ctx, &AccessRequest{
		Bucket: bucket,
		Object: object,
	}); err != nil {
		t.Fatal(err)
	}

	if secret, err = c.Read(ctx, &ReadRequest{
		Bucket: bucket,
		Object: object,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err = c.Create(ctx, &CreateRequest{
		Bucket:    bucket,
		Object:    object2,
		Key:       key,
		Plaintext: original,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err = c.Update(ctx, &UpdateRequest{
		Bucket:    bucket,
		Object:    object2,
		Plaintext: updated,
	}); err != nil {
		t.Fatal(err)
	}

	secrets, err := c.List(ctx, &ListRequest{
		Bucket: bucket,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !testStringInclude(secrets.Secrets, object, secret.Generation) {
		t.Errorf("expected %#v to include %q", secrets, object)
	}
	if !testStringInclude(secrets.Secrets, object2, 0) {
		t.Errorf("expected %#v to include %q", secrets, object2)
	}

	secrets, err = c.List(ctx, &ListRequest{
		Bucket: bucket,
		Prefix: object[:3],
	})
	if err != nil {
		t.Fatal(err)
	}
	if !testStringInclude(secrets.Secrets, object, secret.Generation) {
		t.Errorf("expected %#v to include %q", secrets, object)
	}
	if testStringInclude(secrets.Secrets, object2, secret.Generation) {
		t.Errorf("expected %#v to not include %q", secrets, object)
	}

	var updatedSecret *Secret
	if updatedSecret, err = c.Update(ctx, &UpdateRequest{
		Bucket:         bucket,
		Object:         object,
		Generation:     secret.Generation,
		Key:            secret.KMSKey,
		Metageneration: secret.Metageneration,
		Plaintext:      updated,
	}); err != nil {
		t.Fatal(err)
	}

	secrets, err = c.List(ctx, &ListRequest{
		Bucket:      bucket,
		Prefix:      object,
		Generations: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !testStringInclude(secrets.Secrets, object, updatedSecret.Generation) {
		t.Errorf("expected %#v to include %q", secrets, object)
	}
	if !testStringInclude(secrets.Secrets, object, secret.Generation) {
		t.Errorf("expected %#v to include %q", secrets, object)
	}

	plaintext, err = c.Access(ctx, &AccessRequest{
		Bucket: bucket,
		Object: object,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plaintext, updated) {
		t.Errorf("expected %q to be %q", plaintext, updated)
	}

	plaintext, err = c.Access(ctx, &AccessRequest{
		Bucket:     bucket,
		Object:     object,
		Generation: secret.Generation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plaintext, original) {
		t.Errorf("expected %q to be %q", plaintext, original)
	}

	if err := c.Revoke(ctx, &RevokeRequest{
		Bucket:  bucket,
		Object:  object,
		Members: []string{sa},
	}); err != nil {
		t.Fatal(err)
	}

	if err := c.Delete(ctx, &DeleteRequest{
		Bucket: bucket,
		Object: object,
	}); err != nil {
		t.Fatal(err)
	}

	if err := c.Delete(ctx, &DeleteRequest{
		Bucket: bucket,
		Object: object2,
	}); err != nil {
		t.Fatal(err)
	}
}

func testStringInclude(l []*Secret, n string, g int64) bool {
	for _, v := range l {
		if n == v.Name && (g == 0 || g == v.Generation) {
			return true
		}
	}
	return false
}

func testUUID(tb testing.TB) string {
	tb.Helper()

	u, err := uuid.NewV4()
	if err != nil {
		tb.Fatal(err)
	}
	return u.String()
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
