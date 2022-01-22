# Berglas

[![Build Status](https://img.shields.io/github/workflow/status/GoogleCloudPlatform/berglas/Test.svg?style=flat-square)](https://github.com/GoogleCloudPlatform/berglas/actions)
[![GoDoc](https://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][berglas-godoc]

![Berglas Logo](/logos/berglas.svg)

Berglas is a command line tool and library for storing and retrieving
secrets on Google Cloud. Secrets are encrypted with [Cloud KMS][cloud-kms] and
stored in [Cloud Storage][cloud-storage]. An interoperable layer also exists with [Secret Manager][secret-manager].

- As a **CLI**, `berglas` automates the process of encrypting, decrypting, and
  storing data on Google Cloud.

- As a **library**, `berglas` automates the inclusion of secrets into various
  Google Cloud runtimes.

**Berglas is not an officially supported Google product.**


## Setup

### Prerequisites

1. Install the [Cloud SDK][cloud-sdk] for your operating system. Alternatively,
   you can run these commands from [Cloud Shell][cloud-shell], which has the SDK
   and other popular tools pre-installed.

    If you are running from your local machine, you also need Default
    Application Credentials:

    ```text
    gcloud auth application-default login
    ```

    This will open a web browser and prompt for a login to your Google account.
    On headless devices, you will need to create a service account. For more
    information, please see the [authentication](#authentication) section.

1. Install the `berglas` CLI using **one** of the following methods:

    - Install a pre-compiled binary for your operating system:

        - [darwin/amd64](https://storage.googleapis.com/berglas/main/darwin_amd64/berglas)
        - [linux/amd64](https://storage.googleapis.com/berglas/main/linux_amd64/berglas)
        - [windows/amd64](https://storage.googleapis.com/berglas/main/windows_amd64/berglas)

      This will download the latest version built against the main branch. To
      download a specific version, specify a git tag in place of "main" in the
      URL.

      Depending on your operating system, you may need to mark the downloaded binary as executable:

        ```text
        chmod +x /path/to/berglas
        ```

    - If you use [Homebrew](https://brew.sh/) on macOS, you can install like this:

      ```text
      brew install berglas
      ```

    - Use an official Docker container:

      ```text
      asia-docker.pkg.dev/berglas/berglas/berglas
      europe-docker.pkg.dev/berglas/berglas/berglas
      us-docker.pkg.dev/berglas/berglas/berglas
      ```

      This will pull the latest version built against the main branch. To use
      a specific version, specify a git tag in place of "latest" in the URL.

      Note: the `gcr.io/berglas/berglas:latest` image remains for backwards
      compatibility, but new versions are **not** published there.

    - Install from source (requires a working Go installation):

      ```text
      go install github.com/GoogleCloudPlatform/berglas@latest
      ```

1. Export your project ID as an environment variable. The rest of this setup
   guide assumes this environment variable is set:

    ```text
    export PROJECT_ID=my-gcp-project-id
    ```

    Please note, this is the project _ID_, not the project _name_ or project
    _number_. You can find the project ID by running `gcloud projects list` or
    in the web UI.

### Secret Manager Storage

1. Enable required services on the project:

    ```text
    gcloud services enable --project ${PROJECT_ID} \
      secretmanager.googleapis.com
    ```

### Cloud Storage Storage

1. Export your desired Cloud Storage bucket name. The rest of this setup guide
   assumes this environment variable is set:

    ```text
    export BUCKET_ID=my-secrets
    ```

    Replace `my-secrets` with the name of your bucket. Set only the name,
    without the `gs://` prefix. **This bucket should not exist yet!**

1. Enable required services on the project:

    ```text
    gcloud services enable --project ${PROJECT_ID} \
      cloudkms.googleapis.com \
      storage-api.googleapis.com \
      storage-component.googleapis.com
    ```

1. Bootstrap a Berglas environment. This will create a new Cloud Storage bucket
   for storing secrets and a Cloud KMS key for encrypting data.

    ```text
    berglas bootstrap --project $PROJECT_ID --bucket $BUCKET_ID
    ```

    This command uses the default values. You can customize the storage bucket
    and KMS key configuration using the optional flags. Run `berglas bootstrap
    -h` for more details.

    If you want full control over the creation of the Cloud Storage and Cloud
    KMS keys, please see the [custom setup documentation][custom-setup].

1. _(Optional)_ Bootstrap a Berglas environment specifying a bucket location. By
   default the berglas bucket is created in the multi-regional location `US`.
   You can specify your location by using the following command. Please see the
   list of supported locations in the [GCP bucket location documentation
   page](https://cloud.google.com/storage/docs/locations)

    ```text
    export BUCKET_LOCATION=europe-west1
    berglas bootstrap \
      --project $PROJECT_ID \
      --bucket $BUCKET_ID \
      --bucket-location $BUCKET_LOCATION
    ```

    This command uses the default values. You can customize the storage bucket
    and KMS key configuration using the optional flags. Run `berglas bootstrap
    -h` for more details.

    If you want full control over the creation of the Cloud Storage and Cloud
    KMS keys, please see the [custom setup documentation][custom-setup].

1. _(Optional)_ Enable [Cloud Audit logging][cloud-audit] on the bucket:

    Please note this will enable audit logging on all Cloud KMS keys and all
    Cloud Storage buckets in the project, which may incur additional costs.

    1. Download the exiting project IAM policy:

        ```text
        gcloud projects get-iam-policy ${PROJECT_ID} > policy.yaml
        ```

    1. Add Cloud Audit logging for Cloud KMS and Cloud Storage:

        ```text
        cat <<EOF >> policy.yaml
        auditConfigs:
        - auditLogConfigs:
          - logType: DATA_READ
          - logType: ADMIN_READ
          - logType: DATA_WRITE
          service: cloudkms.googleapis.com
        - auditLogConfigs:
          - logType: ADMIN_READ
          - logType: DATA_READ
          - logType: DATA_WRITE
          service: storage.googleapis.com
        EOF
        ```

    1. Submit the new policy:

        ```text
        gcloud projects set-iam-policy ${PROJECT_ID} policy.yaml
        ```

    1. Remove the updated policy from local disk:

        ```text
        rm policy.yaml
        ```


## CLI Usage

1. Create a secret:

    Using Secret Manager storage:

    ```text
    berglas create sm://${PROJECT_ID}/foo my-secret-data
    ```

    Using Cloud Storage storage:

    ```text
    berglas create ${BUCKET_ID}/foo my-secret-data \
      --key projects/${PROJECT_ID}/locations/global/keyRings/berglas/cryptoKeys/berglas-key
    ```

1. Grant access to a secret:

    Using Secret Manager storage:

    ```text
    berglas grant sm://${PROJECT_ID}/foo --member user:user@mydomain.com
    ```

    Using Cloud Storage storage:

    ```text
    berglas grant ${BUCKET_ID}/foo --member user:user@mydomain.com
    ```

1. Access a secret's data:

    Using Secret Manager storage:

    ```text
    berglas access sm://${PROJECT_ID}/foo
    my-secret-data
    ```

    Using Cloud Storage storage:

    ```text
    berglas access ${BUCKET_ID}/foo
    my-secret-data
    ```

1. Spawn a child process with secrets populated in the child's environment:

    ```text
    berglas exec -- myapp --flag-a --flag-b
    ```

    This will spawn `myapp` with an environment parsed by berglas.

1. Access data from a specific version/generation of a secret:

    Using Secret Manager storage:

    ```text
    berglas access sm://${PROJECT_ID}/foo#1
    my-previous-secret-data
    ```

    Using Cloud Storage storage:

    ```text
    berglas access ${BUCKET_ID}/foo#1563925940580201
    my-previous-secret-data
    ```

1. Revoke access to a secret:

    Using Secret Manager storage:

    ```text
    berglas revoke sm://${PROJECT_ID}/foo --member user:user@mydomain.com
    my-previous-secret-data
    ```

    Using Cloud Storage storage:

    ```text
    berglas revoke ${BUCKET_ID}/foo --member user:user@mydomain.com
    ```

1. Delete a secret:

    Using Secret Manager storage:

    ```text
    berglas delete sm://${PROJECT_ID}/foo
    ```

    Using Cloud Storage storage:

    ```text
    berglas delete ${BUCKET_ID}/foo
    ```

In addition to standard Unix exit codes, if the CLI exits with a known error,
Berglas will exit with one of the following:

- `60` - API error. Berglas got a bad response when communicating with an
  upstream API.

- `61` - Misuse error. You gave unexpected input or behavior. Please read the
  error message. Open an issue if you think this is a mistake.

The only exception is `berglas exec`, which will exit with the exit status of
its child command, if one was provided.


## Integrations

- **App Engine (Flex)** - When invoked via [App Engine Flex][app-engine-flex],
  Berglas resolves environment variables to their plaintext values using the
  [`berglas://reference syntax][reference-syntax]. This integration works with
  any language runtime because berglas serves as the entrypoint to the Docker
  container. See [examples/appengineflex](examples/appengineflex) for examples
  and invocations.

- **App Engine (Standard)** - When invoked via [App Engine][app-engine],
  Berglas resolves environment variables to their plaintext values using the
  [`berglas://`reference syntax][reference-syntax]. This integration only works
  with the Go language runtime because it requires importing the `auto/`
  package. See [examples/appengine](examples/appengine) for examples
  and invocations.

- **Cloud Run** - When invoked via [Cloud Run][cloud-run], Berglas resolves
  environment variables to their plaintext values using the [`berglas://`
  reference syntax][reference-syntax]. This integration works with any language
  runtime because berglas serves as the entrypoint to the Docker container. See
  [examples/cloudrun](examples/cloudrun) for examples and invocations.

- **Cloud Functions** - When invoked via [Cloud Functions][cloud-functions],
  Berglas resolves environment variables to their plaintext values using the
  [`berglas://` reference syntax][reference-syntax]. This integration only works
  with the Go language runtime because it requires importing the `auto/`
  package. See [examples/cloudfunctions](examples/cloudfunctions) for examples
  and invocations.

- **Cloud Build** - When invoked via [Cloud Build][cloud-build], Berglas
  resolves environment variables to plaintext values using the [`berglas://`
  reference syntax][reference-syntax]. This integration only works with volume
  mounts, so all Berglas secrets need to specify the `?destination` parameter.
  See [examples/cloudbuild](examples/cloudbuild) for examples and invocations.

- **Kubernetes** - Kubernetes pods can consume Berglas secrets by installing a
  [MutatingWebhook][k8s-mutating]. This webhook mutates incoming pods with the
  [`berglas://` reference syntax][reference-syntax] in environment references to
  resolve at runtime. This integration works with any container, but all pods
  requesting berglas secrets must set an command in their Kubernetes manifests.
  See [examples/kubernetes](examples/kubernetes) for samples and installation
  instructions.

- **Anything** - Wrap any process with `berglas exec --` and Berglas will
  parse any local environment variables with the [`berglas://` reference
  syntax][reference-syntax] and spawn your app as a subprocess with the
  plaintext environment replaced.

## Logging

Both the berglas CLI and berglas library support debug-style logging. This logging is off by default because it adds additional overhead and logs information that may be security-sensitive.

The default logging behavior for the berglas CLI is "text" (it can be changed
with the `--log-format` flag). The default logging behavior for the berglas
library is structured JSON which integrates well with Cloud Logging (it can be
changed to any valid formatter and you can even inject your own logger).


## Examples

Examples are available in the [`examples/` folder](examples).


## Library Usage

Berglas is also a Go library that can be imported in Go projects:

```go
import (
	_ "github.com/GoogleCloudPlatform/berglas/pkg/auto"
)
```

When imported, the `berglas` package will:

1. Download and decrypt any secrets that match the [Berglas environment
variable reference syntax][reference-syntax] in the environment.

1. Replace the value for the environment variable with the decrypted secret.

You can also opt out of auto-parsing and call the library yourself instead:

```go
import (
	"context"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
)

func main() {
	ctx := context.Background()

	// This higher-level API parses the secret reference at the specified
	// environment variable, downloads and decrypts the secret, and replaces the
	// contents of the given environment variable with the secret result.
	if err := berglas.Replace(ctx, "MY_SECRET"); err != nil {
		log.Fatal(err)
	}

	// This lower-level API parses the secret reference, downloads and decrypts
	// the secret, and returns the result. This is useful if you need to mutate
	// the result.
	if v := os.Getenv("MY_SECRET"); v != "" {
		plaintext, err := berglas.Resolve(ctx, v)
		if err != nil {
			log.Fatal(err)
		}
		os.Unsetenv("MY_SECRET")
		os.Setenv("MY_OTHER_SECRET", string(plaintext))
	}
}
```

For more examples and documentation, please see the [godoc][berglas-godoc].


## Authentication

By default, Berglas uses Google Cloud Default Application Credentials. If you
have [gcloud][cloud-sdk] installed locally, ensure you have application default
credentials:

```text
gcloud auth application-default login
```

On GCP services (like Cloud Build, Compute, etc), it will use the service
account attached to the resource.

To use a specific service account, set the `GOOGLE_APPLICATION_CREDENTIALS`
environment variable to the _filepath_ to the JSON file where your credentials
reside on disk:

```text
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/my/credentials.json
```

To learn more, please see the [Google Cloud Service Account
documentation][iam-service-accounts].


## Authorization

To control who or what has access to a secret, use `berglas grant` and `berglas
revoke` commands. These methods use [Cloud IAM][cloud-iam] internally. Any
service account or entity using Berglas will need to authorize using the
`cloud-platform` scope.

### Secret Manager Storage

Creating a secret requires `roles/secretmanager.admin` on Secret Manager in the
project.

Accessing a secret requires `roles/secretmanager.secretAccessor` on the secret
in Secret Manager.

Deleting a secret requires `roles/secretmanager.admin` on Secret Manager in the
project.

### Cloud Storage Storage

Creating a secret requires `roles/storage.objectCreator` on the Cloud Storage
bucket and `roles/cloudkms.cryptoKeyEncrypter` on the Cloud KMS key.

Accessing a secret requires `roles/storage.objectViewer` on the Cloud Storage
bucket and `roles/cloudkms.cryptoKeyDecrypter` on the Cloud KMS key.

Deleting a secret requires `roles/storage.objectAdmin` on the Cloud Storage bucket.


## Implementation

### Secret Manager Storage

This section describes the Secret Manager implementation. This knowledge is not
required to use Berglas, but it is included for security-conscious/curious users
who want to learn about how Berglas works internally to build a threat model.

1. Berglas calls the [Secret Manager][secret-manager] API directly for all
   operations.

### Cloud Storage Storage

This section describes the Cloud Storage implementation. This knowledge is not
required to use Berglas, but it is included for security-conscious/curious users
who want to learn about how Berglas works internally to build a threat model.

When encrypting a secret:

1. Berglas generates an AES-256-GCM data encryption key (DEK) using [Go's crypto
package][go-crypto] for each secret. (N.B. each secret has its own, unique DEK).

1. Berglas encrypts the plaintext data using the locally-generated DEK,
producing encrypted ciphertext, prepended with the AES-GCM nonce.

1. Berglas encrypts the DEK using the specified Cloud KMS key, also known as a
key encryption key (KEK). This process is called [envelope
encryption][envelope-encryption].

1. Berglas stores the Cloud KMS key name, encrypted DEK, and encrypted ciphertext
as a single blob in Cloud Storage.

When decrypting a secret:

1. Berglas downloads the blob from Cloud Storage and separates the Cloud KMS key name,
encrypted DEK, and ciphertext out of the blob.

1. Berglas decrypts the DEK using Cloud KMS. This is part of [envelope encryption][envelope-encryption].

1. Berglas decrypts the ciphertext data locally using the decrypted DEK.


## Security &amp; Threat Model

See the [security and threat model][threat-model].


## FAQ

**Q: Should I use Berglas or [Secret Manager][secret-manager]?**
<br>
Berglas is compatible with [Secret Manager][secret-manager] and offers
convenience wrappers around managing secrets regardless of whether they reside
in Cloud Storage or Secret Manager. New projects should investigate using Secret
Manager directly as it has less operational overhead and complexity, but Berglas
will continue to support Cloud Storage + Cloud KMS secrets.

**Q: Is there a size limit on the data I can encrypt?**
<br>
Berglas is targeted at application secrets like certificates, passwords, and
API keys. While its possible to encrypt larger binary files like PDFs or images,
Berglas uses a a GCM cipher mode to encrypt data, meaning the data must fit in
memory and is [limited to 64GiB][gcm-limits].

**Q: Why do you use [envelope encryption][envelope-encryption] instead of
encrypting the data directly with [Cloud KMS][cloud-kms]?**
<br>
Envelope encryption allows for encrypting the data at the _application layer_,
and it enables encryption of larger payloads, since Cloud KMS has a limit on the
size of the payload it can encrypt. By using envelope encryption, Cloud KMS
always encrypts a fixed size data (the AES-256-GCM key). This saves bandwidth
(since large payloads are encrypted locally) and increases the size of the data
which can be encrypted.

**Q: Why does Berglas need permission to view my GCP resource?**
<br>
Berglas communicates with the API to read the environment variables that were
set on the resource at deploy time. Otherwise, a package could inject arbitrary
environment variables in the Berglas format during application boot.

**Q: I renamed a secret in Cloud Storage and now it fails to decrypt - why?**
<br>
Berglas encrypts secrets with additional authenticated data including the name
of the secret. This reduces the chance an attacker can escalate privilege by
convincing someone to rename a secret so they can gain access.

**Q: Why is it named Berglas?**
<br>
Berglas is a famous magician who is best known for his secrets.


## Contributing

Please see the [contributing
guidelines](https://github.com/GoogleCloudPlatform/berglas/tree/main/CONTRIBUTING.md).


## License

This library is licensed under Apache 2.0. Full license text is available in
[LICENSE](https://github.com/GoogleCloudPlatform/berglas/tree/main/LICENSE).



[app-engine]: https://cloud.google.com/appengine/
[app-engine-flex]: https://cloud.google.com/appengine/docs/flexible/
[cloud-audit]: https://cloud.google.com/logging/docs/audit/configure-data-access#config-api
[cloud-build]: https://cloud.google.com/cloud-build
[cloud-kms]: https://cloud.google.com/kms
[cloud-kms-iam]: https://cloud.google.com/kms/docs/iam
[cloud-functions]: https://cloud.google.com/functions
[cloud-iam]: https://cloud.google.com/iam
[cloud-run]: https://cloud.google.com/run
[cloud-storage]: https://cloud.google.com/storage
[cloud-storage-iam]: https://cloud.google.com/storage/docs/access-control/iam
[cloud-shell]: https://cloud.google.com/shell
[cloud-sdk]: https://cloud.google.com/sdk
[k8s-mutating]: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/
[secret-manager]: https://cloud.google.com/secret-manager
[go-crypto]: https://golang.org/pkg/crypto/
[envelope-encryption]: https://cloud.google.com/kms/docs/envelope-encryption
[custom-setup]: https://github.com/GoogleCloudPlatform/berglas/blob/main/doc/custom-setup.md
[reference-syntax]: https://github.com/GoogleCloudPlatform/berglas/blob/main/doc/reference-syntax.md
[threat-model]: https://github.com/GoogleCloudPlatform/berglas/blob/main/doc/threat-model.md
[releases]: https://github.com/GoogleCloudPlatform/berglas/releases
[berglas-godoc]: https://godoc.org/github.com/GoogleCloudPlatform/berglas
[gcm-limits]: https://crypto.stackexchange.com/questions/31793/plain-text-size-limits-for-aes-gcm-mode-just-64gb
[iam-service-accounts]: https://cloud.google.com/iam/docs/service-accounts
