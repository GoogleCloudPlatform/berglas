# Berglas Threat Model

Because Berglas is storing secrets and other sensitive information, it is
important that you have a full understanding of the threat model.


## Rest

### Secret Manager Storage

Berglas uses the Secret Manager APIs directly. Secret Manager encrypts data automatically.

### Cloud Storage Storage

At rest, data is encrypted twice. First, the data is encrypted using a
single-use local 256-bit AES-GCM key. Then the local key is encrypted using a
remote Google Cloud KMS key. Only the encrypted bytes are stored in Cloud
Storage.

Additionally, Cloud Storage adds an [additional layer of disk-level
encryption](https://cloud.google.com/security/encryption-at-rest/).

**Mitigations**

- N/A


## Transit

Data is protected in transit between Berglas and the Google Cloud APIs via TLS.
You can verify the TLS connection against Google's public CA. There is
intentionally no way to disable TLS verification.

**Mitigations**

- N/A


## Entropy

### Secret Manager Storage

Entropy is not required.

### Cloud Storage Storage

Berglas relies on local system entropy to generate AES-GCM keys for envelope
encryption using Go's standard library. If there is not enough entropy, Berglas
will block waiting for entropy. A skilled attacker may be able to decrease the
availability of a system creating Berglas secrets by starving the system of
entropy.

**Mitigations**

- Do not run Berglas as a shared service
- Ensure `/dev/urandom` (or virtio-rng, etc) are available


## IAM

Access to secrets is controlled via Cloud IAM. You must maintain tight control
over IAM policies to ensure a more secure setup. Granting overly broad
permissions (such as "Owner") greatly decreases your security posturing.

**Mitigations**

- Grant specific IAM permissions
- Separate IAM permissions for "read secret" from "write secret"
- Use dedicated GCP service accounts where possible
- Enable and monitor Cloud Audit logging


## Crypto Algorithm

### Secret Manager Storage

N/A

### Cloud Storage Storage

Berglas generates and consumes AES-256 GCM keys for envelope encryption. At the
time of this writing, there are no known exploits or compromises for limited-use
GCM keys. Should research find that single-use AES-256-GCM keys are less secure,
Berglas will update to new algorithms and provide a migration.

**Mitigations**

- None at this time


## In-band Key Negotiation

### Secret Manager Storage

N/A

### Cloud Storage Storage

Berglas persists the Cloud KMS key ID on the secret object's metadata so that it
need-not be specified during decryption operations. Berglas relies on IAM to
ensure the caller has access to read the secret object, read the secret object's
metadata, and decrypt access using the Cloud KMS key.

An attacker with permission to write data to the Cloud Storage bucket could
overwrite existing secret objects using their own secret data and Cloud KMS key.
While this attacker would not have access to the plaintext material of the
original secret, they could _replace_ the contents of the secret. Depending on
secret consumer's implementation, this could lead to privilege escalation or
even arbitrary code execution.

To illustrate this scenario, consider a process that searches a database for a
secret:

```go
apiKey := os.Getenv("API_KEY") // resolved via Berglas

statement := fmt.Sprintf("SELECT * FROM users WHERE api_key = '%s'", apiKey)
sql.Exec(statement)
```

Now suppose an attacker is able to devise that an application consumes this
secret using Berglas. Also suppose that attacker is able to gain arbitrary
"write" access to the Cloud Storage bucket, perhaps through social engineering
or a leaked credential. The attacker could overwrite the `API_KEY` secret with
their own value, encrypted with their own Cloud KMS key. For example, the
attacker could create a secret with the contents:

```text
foo'; SELECT * FROM users; --
```

In this example, the attacker would be able to execute arbitrary SQL commands
due to the lack of sanitization of the secret input. The attack vectors vary
depending on how the secrets are consumed.

**Mitigations**

- Treat secrets from Berglas as external user input
- Ensure the most minimal set of IAM permissions
- Separate "read secret" from "write secret" permission
- Watch Cloud Audit logs for new write operations
- Routinely audit Cloud IAM permissions


## In-memory Access

When using Berglas auto or `berglas exec`, secrets ultimately end up in the
process environment in plaintext. Any code running in that process (or any root
user on the same OS with privilege to trigger a process dump) could retrieve
the plaintext values.

Similarly, some languages and frameworks automatically dump their environment as
part of debugging in the event of a crash or panic. Since that framework is
running inside the context of berglas, it will dump the plaintext values.

**Mitigations**

- Only run trusted code and dependencies
- Routinely audit your dependencies
- Leverage automated vulnerability scanning
- Disable core dumps in your frameworks
- Disallow outbound network access (sans where explicitly required)


## Filesystem Access

When using `berglas edit` command, secrets are briefly written to a temporary
file on the local filesystem to be opened using an external editor. The file is
owned by current user and guarded by filesystem permissions. Any malicious
executable running as that user may be able to access the plaintext values of
the secret. Furthermore, some editors may cache contents of the file in a
temporary file (such as a swapfile for vim).

**Mitigations**

- Only install trusted executables on the computer used to edit secrets
- Leverage automated vulnerability scanning
- Disable file caching in the editor (e.g. `:setlocal swapfile` for vim)
- Close the file before closing the editor
- Exit the editor promptly even if no edits were made
