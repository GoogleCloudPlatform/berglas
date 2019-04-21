package berglas_test

import (
	"context"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
)

var (
	ctx       = context.Background()
	client, _ = berglas.New(ctx)

	err     error
	secret  []byte
	secrets []string

	bucket = os.Getenv("GOOGLE_CLOUD_BUCKET")
	key    = os.Getenv("GOOGLE_CLOUD_KMS_KEY")
)

func ExampleNew() {
	client, err = berglas.New(ctx)
}

func ExampleClient_Create() {
	err = client.Create(ctx, &berglas.CreateRequest{
		Bucket:    bucket,
		Object:    "my-secret",
		Key:       key,
		Plaintext: []byte("my secret data"),
	})
}

func ExampleClient_List() {
	secrets, err = client.List(ctx, &berglas.ListRequest{
		Bucket: bucket,
	})

	log.Println(secrets) // ["my secret data"]
}

func ExampleClient_Access() {
	secret, err = client.Access(ctx, &berglas.AccessRequest{
		Bucket: bucket,
		Object: "my-secret",
	})

	log.Println(string(secret)) // "my secret data"
}

func ExampleClient_Grant() {
	err = client.Grant(ctx, &berglas.GrantRequest{
		Bucket: bucket,
		Object: "my-secret",
		Members: []string{
			"serviceAccount:builder@my-project.iam.gserviceaccount.com",
		},
	})
}

func ExampleClient_Revoke() {
	err = client.Revoke(ctx, &berglas.RevokeRequest{
		Bucket: bucket,
		Object: "my-secret",
		Members: []string{
			"serviceAccount:builder@my-project.iam.gserviceaccount.com",
		},
	})
}

func ExampleClient_Delete() {
	err = client.Delete(ctx, &berglas.DeleteRequest{
		Bucket: bucket,
		Object: "my-secret",
	})
}

func ExampleClient_Replace() {
	err = client.Replace(ctx, "MY_ENVVAR")
}

func ExampleClient_Resolve() {
	secret, err = client.Resolve(ctx, "berglas://my-bucket/my-secret")
}
