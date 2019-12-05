package berglas

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestClient_Resolve(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration tests (short)")
	}

	t.Run("tempfile", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		secret, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		genericRef := fmt.Sprintf("berglas://%s/%s?destination=tempfile#%d",
			bucket, object, secret.Generation)

		b, err := client.Resolve(ctx, genericRef)
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := "berglas-", string(b); !strings.Contains(act, exp) {
			t.Errorf("expected %q to contain %q", act, exp)
		}
	})

	t.Run("userfile", func(t *testing.T) {
		t.Parallel()

		client, ctx := testClient(t)
		bucket, object, key := testBucket(t), testObject(t), testKey(t)
		plaintext := []byte("my secret value")

		secret, err := client.Create(ctx, &CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer testCleanup(t, bucket, object)

		// Create a tempfile to resolve to
		tmpFile, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		specificRef := fmt.Sprintf("berglas://%s/%s?destination=%s#%d",
			bucket, object, tmpFile.Name(), secret.Generation)

		b, err := client.Resolve(ctx, specificRef)
		if err != nil {
			t.Fatal(err)
		}

		if exp, act := tmpFile.Name(), string(b); act != exp {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})
}
