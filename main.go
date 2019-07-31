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
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
const (
	// APIExitCode is the exit code returned with an upstream API call fails.
	APIExitCode = 60

	// MisuseExitCode is the exit code returned when the user or system has
	// generated an error.
	MisuseExitCode = 61
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
	stdin  = os.Stdin

	accessGeneration int64

	listGenerations bool
	listPrefix      string

	overwrite bool
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
	Use:   "access [secret]",
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
  berglas access my-secrets/api-key --generation 1563925940580201
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

var createCmd = &cobra.Command{
	Use:   "create [secret] [data]",
	Short: "Create or overwrite a secret",
	Long: strings.Trim(`
Creates a new secret with the given name and contents, encrypted with the
provided Cloud KMS key. If the secret already exists, its contents are
overwritten.

If an object already exists at that location, the operation fails unless
--overwrite is specified.
Use the "edit" command to update an existing secret.
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
	Use:   "delete [secret]",
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
	Use:   "edit [secret]",
	Short: "Edit an existing secret",
	Long: strings.Trim(`
Updates the contents of an existing secret by reading the encrypted data from Google
Cloud Storage, decrypting it with Google Cloud KMS, editing it in-place using an editor,
encrypting the updated content using Google Cloud KMS, writing it back into Google
Cloud Storage. The file must be saved and editor must exit with exit code 0 for the
secret to update.
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
	Use:   "exec -- [subcommand]",
	Short: "Spawn an environment with secrets",
	Long: strings.Trim(`
Parse berglas references and spawn the given command with the secrets in the
childprocess environment similar to exec(1). This is very useful in Docker
containers or languages that do not support auto-import.

By default, this command attempts to communicate with the Cloud APIs to find the
list of environment variables set on a resource. If you are not running inside a
supported runtime, you can specify "-local" to parse the local environment
variables instead.

Berglas will remain the parent process, but stdin, stdout, stderr, and any
signals are proxied to the child process.
`, "\n"),
	Example: strings.Trim(`
  # Spawn a subshell with secrets populated
  berglas exec -- ${SHELL}

  # Run "myapp" after parsing local references
  berglas exec --local -- myapp --with-args
`, "\n"),
	Args: cobra.MinimumNArgs(1),
	RunE: execRun,
}

var grantCmd = &cobra.Command{
	Use:   "grant [secret]",
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
	Use:   "list [bucket]",
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

var revokeCmd = &cobra.Command{
	Use:   "revoke [secret]",
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

	Long: strings.Trim(`
`, "\n"),

}

func main() {
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().Int64Var(&accessGeneration, "generation", 0,
		"Get a specific generation")

	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.Flags().StringVarP(&projectID, "project", "p", "",
		"Google Cloud Project ID")
	bootstrapCmd.MarkFlagRequired("project")
	bootstrapCmd.Flags().StringVarP(&bucket, "bucket", "b", "",
		"Name of the Cloud Storage bucket to create")
	bootstrapCmd.MarkFlagRequired("bucket")
	bootstrapCmd.Flags().StringVarP(&bucketLocation, "bucket-location", "l", "US",
		"Location in which to create Cloud Storage bucket")
	bootstrapCmd.Flags().StringVarP(&kmsLocation, "kms-location", "m", "global",
		"Location in which to create the Cloud KMS key ring")
	bootstrapCmd.Flags().StringVarP(&kmsKeyRing, "kms-keyring", "r", "berglas",
		"Name of the KMS key ring to create")
	bootstrapCmd.Flags().StringVarP(&kmsCryptoKey, "kms-key", "k", "berglas-key",
		"Name of the KMS key to create")

	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&key, "key", "k", "",
		"KMS key to use for encryption")
	createCmd.MarkFlagRequired("key")
	createCmd.Flags().BoolVar(&overwrite, "overwrite", false,
		"Overwrite the secret")

	rootCmd.AddCommand(deleteCmd)

	rootCmd.AddCommand(editCmd)
	editCmd.Flags().StringVar(&editor, "editor", "",
		strings.Trim(`
The editor program to use. If this flag is not specified, it defaults to
reading env vars "VISUAL", or "EDITOR" (in that order). If neither environment variable
is found, these commands are attempted: "vi", "emacs", "nano", "pico". The command is
invoked with just one argument: a temporary filename with contents of the secret. The
command MUST exit with code 0.
`, "\n"))
	editCmd.Flags().BoolVar(&createIfMissing, "create-if-missing", false,
		"Create the secret if it doesn't exist")
	editCmd.Flags().StringVarP(&key, "key", "k", "",
		"KMS key to use for encryption (only used when secret doesn't exist)")

	rootCmd.AddCommand(execCmd)
	execCmd.Flags().BoolVarP(&execLocal, "local", "l", false,
		"Parse local environment variables for secrets instead of querying the Cloud APIs")

	rootCmd.AddCommand(grantCmd)
	grantCmd.Flags().StringSliceVarP(&members, "member", "m", nil,
		"Member to add")

	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listGenerations, "all-generations", false,
		"List all versions of secrets")
	listCmd.Flags().StringVar(&listPrefix, "prefix", "",
		"List secrets that match prefix")

	rootCmd.AddCommand(revokeCmd)
	revokeCmd.Flags().StringSliceVarP(&members, "member", "m", nil,
		"Member to remove")


	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		if exitError, ok := err.(ExitError); ok && exitError.ExitCode != 0 {
			os.Exit(exitError.ExitCode)
		}
		os.Exit(1)
	}
}

func accessRun(_ *cobra.Command, args []string) error {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		return ExitError{err, 2}
	}

	ctx := cliCtx()
	secret, err := berglas.Access(ctx, &berglas.AccessRequest{
		Bucket:     bucket,
		Object:     object,
		Generation: accessGeneration,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s", secret.Plaintext)
	return nil
}

func bootstrapRun(_ *cobra.Command, args []string) error {
	ctx := cliCtx()
	if err := berglas.Bootstrap(ctx, &berglas.BootstrapRequest{
		ProjectID:      projectID,
		Bucket:         bucket,
		BucketLocation: bucketLocation,
		KMSLocation:    kmsLocation,
		KMSKeyRing:     kmsKeyRing,
		KMSCryptoKey:   kmsCryptoKey,
	}); err != nil {
		return err
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

func createRun(_ *cobra.Command, args []string) error {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		return ExitError{err, 2}
	}

	data := strings.TrimSpace(args[1])
	plaintext, err := readData(data)
	if err != nil {
		return ExitError{err, 2}
	}

	ctx := cliCtx()
	var secret *berglas.Secret
	if secret, err = berglas.Create(ctx, &berglas.CreateRequest{
		Bucket:    bucket,
		Object:    object,
		Key:       key,
		Plaintext: plaintext,
		Overwrite: overwrite,
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Successfully created secret: %s with generation: %d\n", object, secret.Generation)
	return nil
}

func deleteRun(_ *cobra.Command, args []string) error {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		return ExitError{err, 2}
	}

	ctx := cliCtx()
	if err := berglas.Delete(ctx, &berglas.DeleteRequest{
		Bucket: bucket,
		Object: object,
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Successfully deleted secret if it existed: %s\n", object)
	return nil
}

func findEditor() error {
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		return errors.New("Unable to determine editor. Please set EDITOR or manually specify using the " +
			"--editor flag and try again")
	}
	return nil
}

func editRun(_ *cobra.Command, args []string) (err error) {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		return ExitError{err, 2}
	}

	ctx := cliCtx()
	client, err := berglas.New(ctx)
	if err != nil {
		return errors.Wrap(err, "the secret was not updated")
	}
	var secret *berglas.Secret
	if secret, err = client.Access(ctx, &berglas.AccessRequest{
		Bucket: bucket,
		Object: object,
	}); err != nil && !(createIfMissing && berglas.IsDoesNotExist(err)) {
		return err
	}

	if err = findEditor(); err != nil {
		return err
	}

	f, err := ioutil.TempFile("", "berglas-")
	if err != nil {
		return errors.Wrap(err, "failed to create tempfile for secret")
	}

	defer func() {
		if rmErr := os.Remove(f.Name()); rmErr != nil && err == nil {
			err = rmErr
		}
	}()

	if secret != nil {
		if _, err := f.Write(secret.Plaintext); err != nil {
			return errors.Wrap(err, "failed to write tempfile for secret")
		}
	}
	if err := f.Sync(); err != nil {
		return errors.Wrap(err, "failed to sync tempfile for secret")
	}
	if err := f.Close(); err != nil {
		return errors.Wrap(err, "failed to close tempfile for secret")
	}

	editorSplit := strings.Split(editor, " ")
	editorCmd, editorArgs := editorSplit[0], editorSplit[1:]
	editorArgs = append(editorArgs, f.Name())
	cmd := exec.Command(editorCmd, editorArgs...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return ExitError{errors.Wrap(err, "failed to start editor"), 2}
	}
	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return ExitError{errors.Wrap(err, "editor failed to exit cleanly"), 2}
		}
		return ExitError{errors.Wrap(err, "unknown failure in running editor"), 2}
	}

	newPlaintext, err := ioutil.ReadFile(f.Name())
	if err != nil {
		return ExitError{errors.Wrapf(err, "failed to read secret tempfile"), 2}
	}

	if secret == nil {
		if secret, err = berglas.Create(ctx, &berglas.CreateRequest{
			Bucket:    bucket,
			Object:    object,
			Key:       key,
			Plaintext: newPlaintext,
		}); err != nil {
			return err
		}

		fmt.Fprintf(stdout, "Successfully created secret: %s with generation: %d\n", object, secret.Generation)
		return
	}

	if bytes.Equal(secret.Plaintext, newPlaintext) {
		fmt.Fprintf(stdout, "[WARN] Secret not updated")
		return
	}

	if secret, err = client.Update(ctx, &berglas.UpdateRequest{
		Bucket:         bucket,
		Object:         object,
		Generation:     secret.Generation,
		Key:            secret.KMSKey,
		Metageneration: secret.Metageneration,
		Plaintext:      newPlaintext,
	}); err != nil {
		return ExitError{errors.Wrapf(err, "failed to update secret"), 2}
	}

	fmt.Fprintf(stdout, "Successfully updated secret: %s with generation: %d\n", object, secret.Generation)
	return nil
}

func execRun(_ *cobra.Command, args []string) (err error) {
	execCmd := args[0]
	execArgs := args[1:]

	ctx := cliCtx()
	c, err := berglas.New(ctx)
	if err != nil {
		return err
	}

	env := os.Environ()

	if execLocal {
		// Parse local env
		for i, e := range env {
			p := strings.SplitN(e, "=", 2)
			if len(p) < 2 {
				continue
			}

			k, v := p[0], p[1]
			if !berglas.IsReference(v) {
				continue
			}

			s, err := c.Resolve(ctx, v)
			if err != nil {
				return err
			}
			env[i] = fmt.Sprintf("%s=%s", k, s)
		}
	} else {
		// Parse remote env
		runtimeEnv, err := berglas.DetectRuntimeEnvironment()
		if err != nil {
			err = errors.Wrap(err, "failed to detect runtime environment")
			return err
		}

		envvars, err := runtimeEnv.EnvVars(ctx)
		if err != nil {
			err = errors.Wrap(err, "failed to find environment variables")
			return ExitError{err, 2}
		}

		for k, v := range envvars {
			if !berglas.IsReference(v) {
				continue
			}

			s, err := c.Resolve(ctx, v)
			if err != nil {
				return err
			}
			env = append(env, fmt.Sprintf("%s=%s", k, s))
		}
	}

	// Spawn the command
	cmd := exec.Command(execCmd, execArgs...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return ExitError{err, 2}
	}

	// Listen for signals and send them to the underlying command
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh)
	go func() {
		s := <-signalCh
		if cmd.Process != nil {
			if signalErr := cmd.Process.Signal(s); signalErr != nil && err == nil {
				err = ExitError{errors.Wrap(signalErr, "failed to signal command"), 2}
			}
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return ExitError{exitErr, status.ExitStatus()}
			}
		}
		return ExitError{err, 2}
	}
	return nil
}

func grantRun(_ *cobra.Command, args []string) error {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		return ExitError{err, 2}
	}

	sort.Strings(members)

	ctx := cliCtx()
	if err := berglas.Grant(ctx, &berglas.GrantRequest{
		Bucket:  bucket,
		Object:  object,
		Members: members,
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Successfully granted permission on %s to: \n- %s\n",
		object, strings.Join(members, "\n- "))
	return nil
}

func listRun(_ *cobra.Command, args []string) error {
	bucket := strings.TrimPrefix(args[0], "gs://")

	ctx := cliCtx()
	list, err := berglas.List(ctx, &berglas.ListRequest{
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

	return nil
}

func revokeRun(_ *cobra.Command, args []string) error {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		return ExitError{err, 2}
	}

	sort.Strings(members)

	ctx := cliCtx()
	if err := berglas.Revoke(ctx, &berglas.RevokeRequest{
		Bucket:  bucket,
		Object:  object,
		Members: members,
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Successfully revoked permission on %s to: \n- %s\n",
		object, strings.Join(members, "\n- "))
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

// cliCtx is a context that is canceled on os.Interrupt.
func cliCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx
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
		return r.ReadBytes('\n')
	case strings.HasPrefix(s, "\\"):
		return []byte(s[1:]), nil
	default:
		return []byte(s), nil
	}
}

// parseRef parses a secret ref into a bucket, secret path, and any errors.
func parseRef(s string) (string, string, error) {
	s = strings.TrimPrefix(s, "gs://")
	s = strings.TrimPrefix(s, "berglas://")

	ss := strings.SplitN(s, "/", 2)
	if len(ss) < 2 {
		return "", "", errors.Errorf("secret does not match format gs://<bucket>/<secret> or the format berglas://<bucket>/<secret>: %s", s)
	}

	return ss[0], ss[1], nil
}
