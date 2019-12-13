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

	err          error
	secret       *berglas.Secret
	plaintext    []byte
	listResponse *berglas.ListResponse

	project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	bucket  = os.Getenv("GOOGLE_CLOUD_BUCKET")
	key     = os.Getenv("GOOGLE_CLOUD_KMS_KEY")
)

func ExampleNew() {
	client, err = berglas.New(ctx)
}

func ExampleClient_Access_secretManager() {
	plaintext, err = client.Access(ctx, &berglas.SecretManagerAccessRequest{
		Project: project,
		Name:    "my-secret",
	})

	log.Println(plaintext) // "abcd1234"
}

func ExampleClient_Access_storage() {
	plaintext, err = client.Access(ctx, &berglas.StorageAccessRequest{
		Bucket: bucket,
		Object: "my-secret",
	})

	log.Println(plaintext) // "abcd1234"
}

func ExampleClient_Bootstrap_secretManager() {
	// This is a noop - there's nothing to bootstrap with Secret Manager
	err = client.Bootstrap(ctx, &berglas.SecretManagerBootstrapRequest{})
}

func ExampleClient_Bootstrap_storage() {
	err = client.Bootstrap(ctx, &berglas.StorageBootstrapRequest{
		ProjectID:      "my-project",
		Bucket:         bucket,
		BucketLocation: "US",
		KMSLocation:    "global",
		KMSKeyRing:     "berglas",
		KMSCryptoKey:   "berglas-key",
	})
}

func ExampleClient_Create_secretManager() {
	secret, err = client.Create(ctx, &berglas.SecretManagerCreateRequest{
		Project:   project,
		Name:      "my-secret",
		Plaintext: []byte("my secret data"),
	})

	log.Printf("%v\n", secret)
}

func ExampleClient_Create_storage() {
	secret, err = client.Create(ctx, &berglas.StorageCreateRequest{
		Bucket:    bucket,
		Object:    "my-secret",
		Key:       key,
		Plaintext: []byte("my secret data"),
	})

	log.Printf("%v\n", secret)
}

func ExampleClient_Delete_secretManager() {
	err = client.Delete(ctx, &berglas.SecretManagerDeleteRequest{
		Project: project,
		Name:    "my-secret",
	})
}

func ExampleClient_Delete_storage() {
	err = client.Delete(ctx, &berglas.StorageDeleteRequest{
		Bucket: bucket,
		Object: "my-secret",
	})
}

func ExampleClient_Grant_secretManager() {
	err = client.Grant(ctx, &berglas.SecretManagerGrantRequest{
		Project: project,
		Name:    "my-secret",
		Members: []string{
			"serviceAccount:builder@my-project.iam.gserviceaccount.com",
		},
	})
}

func ExampleClient_Grant_storage() {
	err = client.Grant(ctx, &berglas.StorageGrantRequest{
		Bucket: bucket,
		Object: "my-secret",
		Members: []string{
			"serviceAccount:builder@my-project.iam.gserviceaccount.com",
		},
	})
}

func ExampleClient_List_secretManager() {
	listResponse, err = client.List(ctx, &berglas.SecretManagerListRequest{
		Project: project,
	})

	log.Println(listResponse) // [&Secret{...}]
}

func ExampleClient_List_storage() {
	listResponse, err = client.List(ctx, &berglas.StorageListRequest{
		Bucket: bucket,
	})

	log.Println(listResponse) // [&Secret{...}]
}

func ExampleClient_Read_secretManager() {
	secret, err = client.Read(ctx, &berglas.SecretManagerReadRequest{
		Project: project,
		Name:    "my-secret",
		Version: "12",
	})

	log.Println(secret) // &Secret{...}
}

func ExampleClient_Read_storage() {
	secret, err = client.Read(ctx, &berglas.StorageReadRequest{
		Bucket:     bucket,
		Object:     "my-secret",
		Generation: secret.Generation,
	})

	log.Println(secret) // &Secret{...}
}

func ExampleClient_Revoke_secretManager() {
	err = client.Revoke(ctx, &berglas.SecretManagerRevokeRequest{
		Project: project,
		Name:    "my-secret",
		Members: []string{
			"serviceAccount:builder@my-project.iam.gserviceaccount.com",
		},
	})
}

func ExampleClient_Revoke_storage() {
	err = client.Revoke(ctx, &berglas.StorageRevokeRequest{
		Bucket: bucket,
		Object: "my-secret",
		Members: []string{
			"serviceAccount:builder@my-project.iam.gserviceaccount.com",
		},
	})
}

func ExampleClient_Replace_secretManager() {
	// MY_ENVVAR = "sm://my-project/my-secret#12"
	err = client.Replace(ctx, "MY_ENVVAR")
}

func ExampleClient_Replace_storage() {
	// MY_ENVVAR = "berglas://my-bucket/my-object#12248904892"
	err = client.Replace(ctx, "MY_ENVVAR")
}

func ExampleClient_Resolve_secretManager() {
	plaintext, err = client.Resolve(ctx, "sm://my-project/my-secret")
	log.Println(plaintext) // "my secret data"
}

func ExampleClient_Resolve_storage() {
	plaintext, err = client.Resolve(ctx, "berglas://my-bucket/my-object")
	log.Println(plaintext) // "my secret data"
}

func ExampleClient_Update_secretManager() {
	secret, err = client.Update(ctx, &berglas.SecretManagerUpdateRequest{
		Project:   project,
		Name:      "my-secret",
		Plaintext: []byte("my updated secret data"),
	})

	log.Println(secret) // [&Secret{"my updated secret data"...}]
}

func ExampleClient_Update_storage() {
	secret, err = client.Update(ctx, &berglas.StorageUpdateRequest{
		Bucket:         bucket,
		Object:         "my-secret",
		Generation:     secret.Generation,
		Key:            secret.KMSKey,
		Metageneration: secret.Metageneration,
		Plaintext:      []byte("my updated secret data"),
	})

	log.Println(secret) // [&Secret{"my updated secret data"...}]
}
