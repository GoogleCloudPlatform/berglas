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

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// APIExitCode is the exit code returned with an upstream API call fails.
	APIExitCode = 60

	// MisuseExitCode is the exit code returned when the user did something wrong
	// such as misused a flag.
	MisuseExitCode = 61
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
	stdin  = os.Stdin

	logFormat string
	logLevel  string

	accessGeneration int64

	listGenerations bool
	listPrefix      string

	key       string
	execLocal bool

	editor          string
	createIfMissing bool

	members []string

	projectID      string
	bucket         string
	bucketLocation string
	kmsLocation    string
	kmsKeyRing     string
	kmsCryptoKey   string
	smLocations    []string
)

var rootCmd = &cobra.Command{
	Use:   "berglas",
	Short: "Interact with encrypted secrets",
	Long: strings.Trim(`
berglas is a CLI tool to reading, writing, and deleting secrets from a Cloud
Storage bucket encrypted with a Google Cloud KMS key. Secrets are encrypted
locally using envelope encryption before being uploaded to Cloud Storage.

Secrets are specified in the format:

    <bucket>/<secret>

For example:

    my-gcs-bucket/my-secret
    my-gcs-bucket/foo/bar/baz

For more information and examples, see the help text for a specific command.
`, "\n"),
	SilenceErrors: true,
	SilenceUsage:  true,
	Version:       berglas.Version,
}

var accessCmd = &cobra.Command{
	Use:   "access SECRET",
	Short: "Access a secret's contents",
	Long: strings.Trim(`
Accesses the contents of a secret by reading the encrypted data from Google
Cloud Storage and decrypting it with Google Cloud KMS.

The result will be the raw value without any additional formatting or newline
characters.
`, "\n"),
	Example: strings.Trim(`
  # Read a secret named "api-key" from the bucket "my-secrets"
  berglas access my-secrets/api-key

  # Read generation 1563925940580201 of a secret named "api-key" from the bucket "my-secrets"
  berglas access my-secrets/api-key#1563925940580201
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: accessRun,
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap a berglas environment",
	Long: strings.Trim(`
Bootstrap a Berglas environment by creating a Cloud Storage bucket and a Cloud
KMS key with properly scoped permissions to the caller.

This command will create a new Cloud Storage bucket with "private" ACLs and
grant permission only to the caller in the specified project. It will enable
versioning on the bucket, configured to retain the last 10 verions. If the
bucket already exists, an error is returned.

This command will also create a Cloud KMS key ring and crypto key in the
specified project. If the key ring or crypto key already exist, no errors are
returned.
`, "\n"),
	Example: strings.Trim(`
  # Bootstrap a berglas environment
  berglas bootstrap --project my-project --bucket my-bucket
`, "\n"),
	Args: cobra.ExactArgs(0),
	RunE: bootstrapRun,
}

var completionCmd = &cobra.Command{
	Use:   "completion SHELL",
	Args:  cobra.ExactArgs(1),
	Short: "Outputs shell completion for the given shell (bash or zsh)",
	Long: strings.Trim(
		`Outputs shell completion for the given shell (bash or zsh)

This depends on the bash-completion package. To install it:

  # Mac OS X
  brew install bash-completion

  # Debian
  apt-get install bash-completion

Zsh users may also put the file somewhere on their $fpath, like
/usr/local/share/zsh/site-functions
`, "\n"),
	Example: strings.Trim(`
  # Enable completion for bash users
  source <(berglas completion bash)

  # Enable completion for zsh users
  source <(berglas completion zsh)
`, "\n"),
	RunE: completionRun,
}

var createCmd = &cobra.Command{
	Use:   "create SECRET DATA",
	Short: "Create a secret",
	Long: strings.Trim(`
Creates a new secret with the given name and contents, encrypted with the
provided Cloud KMS key. If the secret already exists, an error is returned.

Use the "edit" or "update" commands to update an existing secret.
`, "\n"),
	Example: strings.Trim(`
  # Create a secret named "api-key" with the contents "abcd1234"
  berglas create my-secrets/api-key abcd1234 \
    --key projects/my-p/locations/global/keyRings/my-kr/cryptoKeys/my-k

  # Read a secret from stdin
  echo ${SECRET} | berglas create my-secrets/api-key - --key...

  # Read a secret from a local file
  berglas create my-secrets/api-key @/path/to/file --key...
`, "\n"),
	Args: cobra.ExactArgs(2),
	RunE: createRun,
}

var deleteCmd = &cobra.Command{
	Use:   "delete SECRET",
	Short: "Remove a secret",
	Long: strings.Trim(`
Deletes a secret from a Google Cloud Storage bucket by deleting the underlying
GCS object. If the secret does not exist, this operation is a no-op.

This command will exit successfully even if the secret does not exist.
`, "\n"),
	Example: strings.Trim(`
  # Delete a secret named "api-key"
  berglas delete my-secrets/api-key
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: deleteRun,
}

var editCmd = &cobra.Command{
	Use:   "edit SECRET",
	Short: "Edit an existing secret",
	Long: strings.Trim(`
Updates the contents of an existing secret by reading the encrypted data from
Google Cloud Storage, decrypting it with Google Cloud KMS, editing it in-place
using an editor, encrypting the updated content using Google Cloud KMS, writing
it back into Google Cloud Storage.

The file must be saved with changes and editor must exit with exit code 0 for
the secret to be updated.
`, "\n"),
	Example: strings.Trim(`
  # Edit a secret named "api-key" from the bucket "my-secrets"
  berglas edit my-secrets/api-key

  # Edit a secret named "api-key" from the bucket "my-secrets" using emacs
  berglas edit my-secrets/api-key --editor emacs
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: editRun,
}

var execCmd = &cobra.Command{
	Use:   "exec -- SUBCOMMAND",
	Short: "Spawn an environment with secrets",
	Long: strings.Trim(`
Parse berglas references and spawn the given command with the secrets in the
childprocess environment similar to exec(1). This is very useful in Docker
containers or languages that do not support auto-import.

Berglas will remain the parent process, but stdin, stdout, stderr, and any
signals are proxied to the child process.

WARNING: Using berglas exec exposes secrets in plaintext in environment
variables. You should have a strong understanding of your software supply
chain security before blindly running a process with berglas exec. The
resolved secrets will be in plaintext and available to the entire process.
`, "\n"),
	Example: strings.Trim(`
  # Spawn a subshell with secrets populated
  berglas exec -- ${SHELL}
`, "\n"),
	Args: cobra.MinimumNArgs(1),
	RunE: execRun,
}

var grantCmd = &cobra.Command{
	Use:   "grant SECRET",
	Short: "Grant access to a secret",
	Long: strings.Trim(`
Grant IAM access to an existing secret for a given list of members. The secret
must exist before access can be granted.

When executed, this command grants each specified member two IAM permissions:

  - roles/storage.legacyObjectReader on the Cloud Storage object
  - roles/cloudkms.cryptoKeyDecrypter on the Cloud KMS crypto key

Members must be specified with their type, for example:

  - domain:mydomain.com
  - group:group@mydomain.com
  - serviceAccount:xyz@gserviceaccount.com
  - user:user@mydomain.com
`, "\n"),
	Example: strings.Trim(`
  # Grant access to a user
  berglas grant my-secrets/api-key --member user:user@mydomain.com

  # Grant access to service account
  berglas grant my-secrets/api-key \
    --member serviceAccount:sa@project.iam.gserviceaccount.com

  # Add multiple members
  berglas grant my-secrets/api-key \
    --member user:user@mydomain.com \
    --member serviceAccount:sa@project.iam.gserviceaccount.com
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: grantRun,
}

var listCmd = &cobra.Command{
	Use:   "list BUCKET",
	Short: "List secrets in a bucket",
	Long: strings.Trim(`
Lists secrets by name in the given Google Cloud Storage bucket. It does not
read their values, only their key names. To retrieve the value of a secret, use
the "access" command instead.
`, "\n"),
	Example: strings.Trim(`
  # List all secrets in the bucket "my-secrets"
  berglas list my-secrets

  # List all secrets with names starting with "secret" in the bucket "my-secrets"
  berglas list my-secrets --prefix secret

  # List all generations of all secrets in the bucket "my-secrets"
  berglas list my-secrets --all-generations
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: listRun,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate BUCKET ",
	Short: "Migrate Berglas secrets to Secret Manager",
	Long: strings.Trim(`
Migrate secrets in the given Google Cloud Storage bucket to Secret Manager. This
is designed to be a single-use command and should not be used as part of a
regular workflow. Secrets will be migrated as-is with the following caveats:

- Deeply-nested secrets in folders will be underscored. Since Secret Manager
  does not support nested structures, any secrets in the bucket inside folders
  will be renamed with the slash ("/") as an underscore ("_").

- Generation versions are not preserved. Generations (versions) in Cloud Storage
  are random integers. Versions in Secret Manager are auto-incrementing. While
  relative ordering will be preserved, the versions will differ.

This command is intentionally a slow and non-parallelized operation to both
avoid quota limits and to discourage recurrent use.
`, "\n"),
	Example: strings.Trim(`
  # Migrate all secrets in the "my-secrets" bucket to Secret Manager in the
  # project "my-project"
  berglas migrate my-secrets --project my-project
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: migrateRun,
}

var revokeCmd = &cobra.Command{
	Use:   "revoke SECRET",
	Short: "Revoke access to a secret",
	Long: strings.Trim(`
Revoke IAM access to an existing secret for a given list of members. The secret
must exist for access to be revoked.

When executed, this command revokes the following IAM permissions for each
member:

  - roles/storage.legacyObjectReader on the Cloud Storage object
  - roles/cloudkms.cryptoKeyDecrypter on the Cloud KMS crypto key

If the member is not granted the IAM permissions, no action is taken.
Specifically, this does not return an error if the member did not originally
have permission to access the secret.

Members must be specified with their type, for example:

  - domain:mydomain.com
  - group:group@mydomain.com
  - serviceAccount:xyz@gserviceaccount.com
  - user:user@mydomain.com
`, "\n"),
	Example: strings.Trim(`
  # Revoke access from a user
  berglas revoke my-secrets/api-key --member user:user@mydomain.com

  # Revoke revoke from a service account
  berglas grant my-secrets/api-key \
    --member serviceAccount:sa@project.iam.gserviceaccount.com

  # Remove multiple members
  berglas revoke my-secrets/api-key \
    --member user:user@mydomain.com \
    --member serviceAccount:sa@project.iam.gserviceaccount.com
`, "\n"),
	Args: cobra.ExactArgs(1),
	RunE: revokeRun,
}

var updateCmd = &cobra.Command{
	Use:   "update SECRET [DATA]",
	Short: "Update an existing secret",
	Long: strings.Trim(`
Update an existing secret. If the secret does not exist, an error is returned.

Run with --create-if-missing to force creation of the secret if it does not
already exist.
`, "\n"),
	Example: strings.Trim(`
  # Update the secret named "api-key" with the contents "new-contents"
  berglas update my-secrets/api-key new-contents

  # Update the secret named "api-key" with a new KMS encryption key, keeping
  # the original secret value
  berglas update my-secrets/api-key --key=...

  # Update the secret named "api-key", creating it if it does not already exist
  berglas update my-secrets/api-key abcd1234 --create-if-missing --key...
`, "\n"),
	Args: cobra.RangeArgs(1, 2),
	RunE: updateRun,
}

func main() {
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	rootCmd.PersistentFlags().StringVarP(&logFormat, "log-format", "f", "console",
		"Format in which to log")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "fatal",
		"Level at which to log")

	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().Int64Var(&accessGeneration, "generation", 0,
		"Get a specific generation")
	if err := accessCmd.Flags().MarkDeprecated("generation",
		"please use hash notation instead (e.g. my-secrets/api-key#12345)"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().StringVar(&projectID, "project", "",
		"Google Cloud Project ID")
	if err := bootstrapCmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}
	bootstrapCmd.Flags().StringVar(&bucket, "bucket", "",
		"Name of the Cloud Storage bucket to create")
	if err := bootstrapCmd.MarkFlagRequired("bucket"); err != nil {
		panic(err)
	}
	bootstrapCmd.Flags().StringVar(&bucketLocation, "bucket-location", "US",
		"Location in which to create Cloud Storage bucket")
	bootstrapCmd.Flags().StringVar(&kmsLocation, "kms-location", "global",
		"Location in which to create the Cloud KMS key ring")
	bootstrapCmd.Flags().StringVar(&kmsKeyRing, "kms-keyring", "berglas",
		"Name of the KMS key ring to create")
	bootstrapCmd.Flags().StringVar(&kmsCryptoKey, "kms-key", "berglas-key",
		"Name of the KMS key to create")

	rootCmd.AddCommand(completionCmd)

	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&key, "key", "",
		"KMS key to use for encryption")
	createCmd.Flags().StringSliceVar(&smLocations, "locations", nil,
		"Comma-separated canonical IDs in which to replicate secrets (e.g. 'us-east1,us-west-1')")

	rootCmd.AddCommand(deleteCmd)

	rootCmd.AddCommand(editCmd)
	editCmd.Flags().StringVar(&editor, "editor", "",
		"Editor program to use. If unspecified, this defaults to $VISUAL or "+
			"$EDITOR in that order.")
	editCmd.Flags().BoolVar(&createIfMissing, "create-if-missing", false,
		"Create the secret if it doesn't exist")
	editCmd.Flags().StringVar(&key, "key", "",
		"KMS key to use for encryption (only used when secret doesn't exist)")

	rootCmd.AddCommand(execCmd)
	execCmd.Flags().BoolVar(&execLocal, "local", false, "")
	if err := execCmd.Flags().MarkDeprecated("local", "there is no replacement"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(grantCmd)
	grantCmd.Flags().StringSliceVar(&members, "member", nil,
		"Member to add")

	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listGenerations, "all-generations", false,
		"List all versions of secrets")
	listCmd.Flags().StringVar(&listPrefix, "prefix", "",
		"List secrets that match prefix")

	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVar(&projectID, "project", "",
		"Google Cloud Project ID")
	if err := migrateCmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(revokeCmd)
	revokeCmd.Flags().StringSliceVar(&members, "member", nil,
		"Member to remove")

	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&createIfMissing, "create-if-missing", false,
		"Create the secret if it does not already exist")
	updateCmd.Flags().StringVar(&key, "key", "",
		"KMS key to use for re-encryption")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		if terr, ok := err.(*exitError); ok {
			os.Exit(terr.code)
		}
		os.Exit(1)
	}
}

func accessRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	// Deprecated - update to new syntax
	if accessGeneration != 0 {
		args[0] = fmt.Sprintf("%s#%d", args[0], accessGeneration)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		plaintext, err := client.Access(ctx, &berglas.SecretManagerAccessRequest{
			Project: ref.Project(),
			Name:    ref.Name(),
			Version: ref.Version(),
		})
		if err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "%s", plaintext)
	case berglas.ReferenceTypeStorage:
		plaintext, err := client.Access(ctx, &berglas.StorageAccessRequest{
			Bucket:     ref.Bucket(),
			Object:     ref.Object(),
			Generation: ref.Generation(),
		})
		if err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "%s", plaintext)
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

func bootstrapRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	if err := client.Bootstrap(ctx, &berglas.BootstrapRequest{
		ProjectID:      projectID,
		Bucket:         bucket,
		BucketLocation: bucketLocation,
		KMSLocation:    kmsLocation,
		KMSKeyRing:     kmsKeyRing,
		KMSCryptoKey:   kmsCryptoKey,
	}); err != nil {
		return apiError(err)
	}

	kmsKeyID := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, kmsLocation, kmsKeyRing, kmsCryptoKey)

	fmt.Fprintf(stdout, "Successfully created berglas environment:\n")
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "  Bucket: %s\n", bucket)
	fmt.Fprintf(stdout, "  KMS key: %s\n", kmsKeyID)
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "To create a secret:\n")
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "  berglas create %s/my-secret abcd1234 \\\n", bucket)
	fmt.Fprintf(stdout, "    --key %s\n", kmsKeyID)
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "To grant access to that secret:\n")
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "  berglas grant %s/my-secret \\\n", bucket)
	fmt.Fprintf(stdout, "    --member user:jane.doe@mycompany.com\n")
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "For more help and examples, please run \"berglas -h\".\n")
	return nil
}

func completionRun(cmd *cobra.Command, args []string) error {
	switch shell := args[0]; shell {
	case "bash":
		if err := rootCmd.GenBashCompletion(stdout); err != nil {
			err = fmt.Errorf("failed to generate bash completion: %w", err)
			return apiError(err)
		}
	case "zsh":
		if err := rootCmd.GenZshCompletion(stdout); err != nil {
			err = fmt.Errorf("failed to generate zsh completion: %w", err)
			return apiError(err)
		}

		// enable the `source <(berglas completion SHELL)` pattern for zsh
		if _, err := io.WriteString(stdout, "compdef _berglas berglas\n"); err != nil {
			err = fmt.Errorf("failed to run compdef: %w", err)
			return apiError(err)
		}
	default:
		err := fmt.Errorf("unknown completion %q", shell)
		return misuseError(err)
	}

	return nil
}

func createRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	data := strings.TrimSpace(args[1])
	plaintext, err := readData(data)
	if err != nil {
		return misuseError(err)
	}

	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		secret, err := client.Create(ctx, &berglas.SecretManagerCreateRequest{
			Project:   ref.Project(),
			Name:      ref.Name(),
			Locations: smLocations,
			Plaintext: plaintext,
		})
		if err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully created secret [%s] with version [%s]\n",
			secret.Name, secret.Version)
	case berglas.ReferenceTypeStorage:
		// Check if no unsupported options have been given
		if len(smLocations) > 0 {
			return misuseError(fmt.Errorf("locations on a per-secret basis unsupported for Storage keys"))
		}

		// Create the requested secret
		secret, err := client.Create(ctx, &berglas.StorageCreateRequest{
			Bucket:    ref.Bucket(),
			Object:    ref.Object(),
			Key:       key,
			Plaintext: plaintext,
		})
		if err != nil {
			return apiError(err)
		}

		fmt.Fprintf(stdout, "Successfully created secret [%s] with generation [%d]\n",
			secret.Name, secret.Generation)
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

func deleteRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		if err := client.Delete(ctx, &berglas.SecretManagerDeleteRequest{
			Project: ref.Project(),
			Name:    ref.Name(),
		}); err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully deleted secret [%s] if it existed\n",
			ref.Name())
	case berglas.ReferenceTypeStorage:
		if err := client.Delete(ctx, &berglas.StorageDeleteRequest{
			Bucket: ref.Bucket(),
			Object: ref.Object(),
		}); err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully deleted secret [%s] if it existed\n",
			ref.Object())
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

func editRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	// Find the editor
	var editor string
	for _, e := range []string{"VISUAL", "EDITOR"} {
		if v := os.Getenv(e); v != "" {
			editor = v
			break
		}
	}
	if editor == "" {
		err := fmt.Errorf("no editor is set - set VISUAL or EDITOR")
		return apiError(err)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	var originalSecret *berglas.Secret

	// Get the existing secret
	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		originalSecret, err = client.Read(ctx, &berglas.SecretManagerReadRequest{
			Project: ref.Project(),
			Name:    ref.Name(),
			Version: ref.Version(),
		})
	case berglas.ReferenceTypeStorage:
		originalSecret, err = client.Read(ctx, &berglas.StorageReadRequest{
			Bucket:     ref.Bucket(),
			Object:     ref.Object(),
			Generation: ref.Generation(),
		})
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	if err != nil {
		return apiError(err)
	}

	// Create the tempfile
	f, err := ioutil.TempFile("", "berglas-")
	if err != nil {
		err = fmt.Errorf("failed to create tempfile for secret: %w", err)
		return apiError(err)
	}

	defer func() {
		if err := os.Remove(f.Name()); err != nil {
			fmt.Fprintf(stderr, "failed to cleanup tempfile %s: %s\n", f.Name(), err)
		}
	}()

	// Write contents to the original file
	if _, err := f.Write(originalSecret.Plaintext); err != nil {
		err = fmt.Errorf("failed to write tempfile for secret: %w", err)
		return apiError(err)
	}

	if err := f.Sync(); err != nil {
		err = fmt.Errorf("failed to sync tempfile for secret: %w", err)
		return apiError(err)
	}

	if err := f.Close(); err != nil {
		err = fmt.Errorf("failed to close tempfile for secret: %w", err)
		return apiError(err)
	}

	// Spawn editor
	editorSplit := strings.Split(editor, " ")
	editorCmd, editorArgs := editorSplit[0], editorSplit[1:]
	editorArgs = append(editorArgs, f.Name())
	externalCmd := exec.CommandContext(ctx, editorCmd, editorArgs...)
	externalCmd.Stdin = stdin
	externalCmd.Stdout = stdout
	externalCmd.Stderr = stderr
	if err := externalCmd.Start(); err != nil {
		err = fmt.Errorf("failed to start editor: %w", err)
		return misuseError(err)
	}
	if err := externalCmd.Wait(); err != nil {
		if terr, ok := err.(*exec.ExitError); ok && terr.ProcessState != nil {
			code := terr.ProcessState.ExitCode()
			return exitWithCode(code, fmt.Errorf("editor did not exit 0: %w", err))
		}
		err = fmt.Errorf("unknown failure in running editor: %w", err)
		return misuseError(err)
	}

	// Read the new secret value
	newPlaintext, err := ioutil.ReadFile(f.Name())
	if err != nil {
		err = fmt.Errorf("failed to read secret tempfile: %w", err)
		return misuseError(err)
	}

	// Error if the secret is empty
	if len(newPlaintext) == 0 {
		err := fmt.Errorf("secret is empty")
		return misuseError(err)
	}

	if bytes.Equal(newPlaintext, originalSecret.Plaintext) {
		err := fmt.Errorf("secret unchanged - not going to update")
		return misuseError(err)
	}

	// Update the secret
	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		updatedSecret, err := client.Update(ctx, &berglas.SecretManagerUpdateRequest{
			Project:   ref.Project(),
			Name:      ref.Name(),
			Plaintext: newPlaintext,
		})
		if err != nil {
			err = fmt.Errorf("failed to update secret: %w", err)
			return misuseError(err)
		}

		fmt.Fprintf(stdout, "Successfully updated secret [%s] to version [%s]\n",
			updatedSecret.Name, updatedSecret.Version)
	case berglas.ReferenceTypeStorage:
		updatedSecret, err := client.Update(ctx, &berglas.StorageUpdateRequest{
			Bucket:         ref.Bucket(),
			Object:         ref.Object(),
			Generation:     originalSecret.Generation,
			Key:            originalSecret.KMSKey,
			Metageneration: originalSecret.Metageneration,
			Plaintext:      newPlaintext,
		})
		if err != nil {
			err = fmt.Errorf("failed to update secret: %w", err)
			return misuseError(err)
		}

		fmt.Fprintf(stdout, "Successfully updated secret [%s] with generation [%d]\n",
			updatedSecret.Name, updatedSecret.Generation)
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

func execRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	execCmd := args[0]
	execArgs := args[1:]

	// Parse local env
	env := os.Environ()

	for i, e := range env {
		p := strings.SplitN(e, "=", 2)
		if len(p) < 2 {
			continue
		}

		k, v := p[0], p[1]
		if !berglas.IsReference(v) {
			continue
		}

		s, err := client.Resolve(ctx, v)
		if err != nil {
			return apiError(err)
		}
		env[i] = fmt.Sprintf("%s=%s", k, s)
	}

	execCmdFull, err := exec.LookPath(execCmd)
	if err != nil {
		return fmt.Errorf("failed to lookup path for %q: %w", execCmd, err)
	}

	// Unlike os/exec, execv(3) expects the arguments to include the command.
	execArgs = append([]string{execCmdFull}, execArgs...)

	if err := syscall.Exec(execCmdFull, execArgs, env); err != nil {
		return fmt.Errorf("failed to execute %q: %w", execCmd, err)
	}
	return nil
}

func grantRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	sort.Strings(members)

	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		if err := client.Grant(ctx, &berglas.SecretManagerGrantRequest{
			Project: ref.Project(),
			Name:    ref.Name(),
			Members: members,
		}); err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully granted permission on [%s] to: \n- %s\n",
			ref.Name(), strings.Join(members, "\n- "))
	case berglas.ReferenceTypeStorage:
		if err := client.Grant(ctx, &berglas.StorageGrantRequest{
			Bucket:  ref.Bucket(),
			Object:  ref.Object(),
			Members: members,
		}); err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully granted permission on [%s] to: \n- %s\n",
			ref.Object(), strings.Join(members, "\n- "))
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

func listRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	var list *berglas.ListResponse

	switch {
	case strings.HasPrefix(args[0], "sm://"):
		project := strings.Trim(strings.TrimPrefix(args[0], "sm://"), "/")
		list, err = client.List(ctx, &berglas.SecretManagerListRequest{
			Project:  project,
			Prefix:   listPrefix,
			Versions: listGenerations,
		})
		if err != nil {
			return apiError(err)
		}

		if len(list.Secrets) == 0 {
			return nil
		}

		tw := new(tabwriter.Writer)
		tw.Init(stdout, 0, 4, 4, ' ', 0)
		fmt.Fprintf(tw, "NAME\tVERSION\tUPDATED\n")
		for _, s := range list.Secrets {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, s.Version, s.UpdatedAt.Local())
		}
		tw.Flush()
	default:
		bucket := strings.Trim(strings.TrimPrefix(args[0], "gs://"), "/")
		list, err = client.List(ctx, &berglas.ListRequest{
			Bucket:      bucket,
			Prefix:      listPrefix,
			Generations: listGenerations,
		})
		if err != nil {
			return apiError(err)
		}

		if len(list.Secrets) == 0 {
			return nil
		}

		tw := new(tabwriter.Writer)
		tw.Init(stdout, 0, 4, 4, ' ', 0)
		fmt.Fprintf(tw, "NAME\tGENERATION\tUPDATED\n")
		for _, s := range list.Secrets {
			fmt.Fprintf(tw, "%s\t%d\t%s\n", s.Name, s.Generation, s.UpdatedAt.Local())
		}
		tw.Flush()
	}

	return nil
}

func migrateRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	bucket := strings.Trim(strings.TrimPrefix(args[0], "gs://"), "/")

	storageList, err := client.List(ctx, &berglas.StorageListRequest{
		Bucket:      bucket,
		Generations: true,
	})
	if err != nil {
		return apiError(err)
	}

	for _, s := range storageList.Secrets {
		name := strings.Replace(s.Name, "/", "_", -1)
		fmt.Fprintf(stdout, "Migrating %s to projects/%s/secrets/%s... ",
			s.Name, projectID, name)

		secret, err := client.Read(ctx, &berglas.StorageReadRequest{
			Bucket: s.Parent,
			Object: s.Name,
		})
		if err != nil {
			return apiError(err)
		}

		if len(secret.Plaintext) == 0 {
			fmt.Fprintf(stdout, "skip (empty plaintext)\n")
			continue
		}

		if _, err := client.Update(ctx, &berglas.SecretManagerUpdateRequest{
			Project:         projectID,
			Name:            name,
			Plaintext:       secret.Plaintext,
			CreateIfMissing: true,
		}); err != nil {
			return apiError(err)
		}

		fmt.Fprintf(stdout, "done!\n")
	}

	return nil
}

func revokeRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	sort.Strings(members)

	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		if err := client.Revoke(ctx, &berglas.SecretManagerRevokeRequest{
			Project: ref.Project(),
			Name:    ref.Name(),
			Members: members,
		}); err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully revoked permission on [%s] from: \n- %s\n",
			ref.Name(), strings.Join(members, "\n- "))
	case berglas.ReferenceTypeStorage:
		if err := client.Revoke(ctx, &berglas.StorageRevokeRequest{
			Bucket:  ref.Bucket(),
			Object:  ref.Object(),
			Members: members,
		}); err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully revoked permission on [%s] from: \n- %s\n",
			ref.Object(), strings.Join(members, "\n- "))
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

func updateRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	client, err := clientWithContext(ctx)
	if err != nil {
		return misuseError(err)
	}

	ref, err := parseRef(args[0])
	if err != nil {
		return misuseError(err)
	}

	var plaintext []byte
	if len(args) > 1 {
		plaintext, err = readData(strings.TrimSpace(args[1]))
		if err != nil {
			return misuseError(err)
		}
	}

	switch t := ref.Type(); t {
	case berglas.ReferenceTypeSecretManager:
		secret, err := client.Update(ctx, &berglas.SecretManagerUpdateRequest{
			Project:         ref.Project(),
			Name:            ref.Name(),
			Plaintext:       plaintext,
			CreateIfMissing: createIfMissing,
		})
		if err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully updated secret [%s] to version [%s]\n",
			secret.Name, secret.Version)
	case berglas.ReferenceTypeStorage:
		secret, err := client.Update(ctx, &berglas.StorageUpdateRequest{
			Bucket:          ref.Bucket(),
			Object:          ref.Object(),
			Key:             key,
			Plaintext:       plaintext,
			CreateIfMissing: createIfMissing,
		})
		if err != nil {
			return apiError(err)
		}
		fmt.Fprintf(stdout, "Successfully updated secret [%s] to generation [%d]\n",
			secret.Name, secret.Generation)
	default:
		return misuseError(fmt.Errorf("unknown type %T", t))
	}

	return nil
}

// exitError is a typed error to return.
type exitError struct {
	err  error
	code int
}

// Error implements error.
func (e *exitError) Error() string {
	if e.err == nil {
		return "<missing error>"
	}
	return e.err.Error()
}

// exitWithCode prints exits with the specified error and exit code.
func exitWithCode(code int, err error) *exitError {
	return &exitError{
		err:  err,
		code: code,
	}
}

// apiError returns the given error with an API error exit code.
func apiError(err error) *exitError {
	return exitWithCode(APIExitCode, err)
}

// misuseError returns the given error with a userland exit code.
func misuseError(err error) *exitError {
	return exitWithCode(MisuseExitCode, err)
}

// logger returns the logger for this cli.
func logger() (*logrus.Logger, error) {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	var formatter logrus.Formatter
	switch logFormat {
	case "console", "text":
		formatter = new(logrus.TextFormatter)
	case "json":
		formatter = new(berglas.LogFormatterStackdriver)
	default:
		return nil, fmt.Errorf("unknown log format %q", logFormat)
	}

	return &logrus.Logger{
		Out:       stderr,
		Formatter: formatter,
		Hooks:     make(logrus.LevelHooks),
		Level:     level,
	}, nil
}

// clientWithContext returns an instantiated berglas client and context with a
// closer.
func clientWithContext(ctx context.Context) (*berglas.Client, error) {
	logger, err := logger()
	if err != nil {
		return nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	client, err := berglas.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create berglas client: %w", err)
	}
	client.SetLogger(logger)

	return client, nil
}

// readData reads the given string. If the string starts with an "@", it is
// assumed to be a filepath. If the string starts with a "-", data is read from
// stdin. If the data starts with a "\", it is assumed to be an escape character
// only when specified as the first character.
func readData(s string) ([]byte, error) {
	switch {
	case strings.HasPrefix(s, "@"):
		return ioutil.ReadFile(s[1:])
	case strings.HasPrefix(s, "-"):
		r := bufio.NewReader(stdin)
		b, err := r.ReadBytes('\n')
		if err == io.EOF {
			return b, nil
		}
		if err != nil {
			return nil, err
		}
		return b, nil
	case strings.HasPrefix(s, "\\"):
		return []byte(s[1:]), nil
	default:
		return []byte(s), nil
	}
}

// parseRef parses a secret ref and returns any errors.
func parseRef(r string) (*berglas.Reference, error) {
	s := r

	// Replace gs:// with berglas://
	if strings.HasPrefix(s, "gs://") {
		s = "berglas://" + s[5:]
	}

	// If there's no protocol, assume berglas:// (backwards compat)
	if !strings.Contains(s, "://") {
		s = "berglas://" + s
	}

	ref, err := berglas.ParseReference(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference %q: %w", s, err)
	}
	return ref, nil
}
