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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

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

func testProject(tb testing.TB) string {
	tb.Helper()

	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		tb.Fatal("missing GOOGLE_CLOUD_PROJECT")
	}
	return project
}

func testBucket(tb testing.TB) string {
	tb.Helper()

	bucket := os.Getenv("GOOGLE_CLOUD_BUCKET")
	if bucket == "" {
		tb.Fatal("missing GOOGLE_CLOUD_BUCKET")
	}
	return bucket
}

func testName(tb testing.TB) string {
	tb.Helper()

	// 32 bytes is a 64 character hex value
	b := make([]byte, 32)
	n, err := rand.Read(b)
	if err != nil {
		tb.Fatal(err)
	}
	if got, want := n, len(b); got != want {
		tb.Fatalf("invalid length, got: %v, want: %v", got, want)
	}
	return hex.EncodeToString(b)
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

func testSecretManagerCleanup(tb testing.TB, project, name string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := New(context.Background())
	if err != nil {
		tb.Fatal(err)
	}

	if err := client.Delete(ctx, &SecretManagerDeleteRequest{
		Project: project,
		Name:    name,
	}); err != nil {
		tb.Fatal(err)
	}
}

func testStorageCleanup(tb testing.TB, bucket, object string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := New(context.Background())
	if err != nil {
		tb.Fatal(err)
	}

	if err := client.Delete(ctx, &StorageDeleteRequest{
		Object: object,
		Bucket: bucket,
	}); err != nil {
		tb.Fatal(err)
	}
}

func testAcc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping acceptance tests (-short)")
	}

	t.Parallel()
}
