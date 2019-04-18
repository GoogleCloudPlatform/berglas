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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/GoogleCloudPlatform/berglas/berglas"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
	stdin  = os.Stdin

	key       string
	execLocal bool
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
}

var accessCmd = &cobra.Command{
	Use:   "access [secret]",
	Short: "Access a secret's contents",
	Long: strings.Trim(`
Accesses the contents of a secret by reading the encrypted data from Google
Cloud Storage and decyrpting it with Google Cloud KMS.

The result will be the raw value without any additional formatting or newline
characters.
`, "\n"),
	Example: strings.Trim(`
  # Read a secret named "api-key" from the bucket "my-secrets"
  berglas access my-secrets/api-key
`, "\n"),
	Args: cobra.ExactArgs(1),
	Run:  accessRun,
}

var createCmd = &cobra.Command{
	Use:   "create [secret] [data]",
	Short: "Create or overwrite a secret",
	Long: strings.Trim(`
Creates a new secret with the given name and contents, encrypted with the
provided Cloud KMS key If the secret already exists, its contents are
overwritten.

If a secret already exists at that location, its contents are overwritten.
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
	Run:  createRun,
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
	Run:  deleteRun,
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
	Run:  execRun,
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
  # List all secrets in the bucket "foo"
  berglas list my-secrets
`, "\n"),
	Args: cobra.ExactArgs(1),
	Run:  listRun,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show berlgas version",
	Long: strings.Trim(`
Show berglas version.
`, "\n"),
	Args: cobra.ExactArgs(0),
	Run:  versionRun,
}

func main() {
	rootCmd.AddCommand(accessCmd)

	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&key, "key", "k", "", "KMS key to use for encryption")
	createCmd.MarkFlagRequired("key")

	rootCmd.AddCommand(deleteCmd)

	rootCmd.AddCommand(execCmd)
	execCmd.Flags().BoolVarP(&execLocal, "local", "l", false, "Parse local environment variables for secrets instead of querying the Cloud APIs")

	rootCmd.AddCommand(listCmd)

	rootCmd.AddCommand(versionCmd)

	rootCmd.Execute()
}

func accessRun(_ *cobra.Command, args []string) {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		handleError(err, 2)
	}

	ctx := cliCtx()
	plaintext, err := berglas.Access(ctx, &berglas.AccessRequest{
		Bucket: bucket,
		Object: object,
	})
	if err != nil {
		handleError(err, 1)
	}

	fmt.Fprintf(stdout, "%s", plaintext)
}

func createRun(_ *cobra.Command, args []string) {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		handleError(err, 2)
	}

	data := strings.TrimSpace(args[1])
	plaintext, err := readData(data)
	if err != nil {
		handleError(err, 2)
	}

	ctx := cliCtx()
	if err := berglas.Create(ctx, &berglas.CreateRequest{
		Bucket:    bucket,
		Object:    object,
		Key:       key,
		Plaintext: plaintext,
	}); err != nil {
		handleError(err, 1)
	}

	fmt.Fprintf(stdout, "Successfully created secret: %s\n", object)
}

func deleteRun(_ *cobra.Command, args []string) {
	bucket, object, err := parseRef(args[0])
	if err != nil {
		handleError(err, 2)
	}

	ctx := cliCtx()
	if err := berglas.Delete(ctx, &berglas.DeleteRequest{
		Bucket: bucket,
		Object: object,
	}); err != nil {
		handleError(err, 1)
	}

	fmt.Fprintf(stdout, "Successfully deleted secret if it existed: %s\n", object)
}

func execRun(_ *cobra.Command, args []string) {
	execCmd := args[0]
	execArgs := args[1:]

	ctx := cliCtx()
	c, err := berglas.New(ctx)
	if err != nil {
		handleError(err, 1)
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
				handleError(err, 1)
			}
			env[i] = fmt.Sprintf("%s=%s", k, s)
		}
	} else {
		// Parse remote env
		runtimeEnv, err := berglas.DetectRuntimeEnvironment()
		if err != nil {
			err = errors.Wrap(err, "failed to detect runtime environment")
			handleError(err, 1)
		}

		envvars, err := runtimeEnv.EnvVars(ctx)
		if err != nil {
			err = errors.Wrap(err, "failed to find environment variables")
			handleError(err, 2)
		}

		for k, v := range envvars {
			if !berglas.IsReference(v) {
				continue
			}

			s, err := c.Resolve(ctx, v)
			if err != nil {
				handleError(err, 1)
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
		handleError(err, 2)
	}

	// Listen for signals and send them to the underlying command
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh)
	go func() {
		select {
		case s := <-signalCh:
			if cmd.Process != nil {
				cmd.Process.Signal(s)
			}
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				handleError(exitErr, status.ExitStatus())
			}
		}
		handleError(err, 2)
	}
}

func listRun(_ *cobra.Command, args []string) {
	bucket := strings.TrimPrefix(args[0], "gs://")

	ctx := cliCtx()
	secrets, err := berglas.List(ctx, &berglas.ListRequest{
		Bucket: bucket,
	})
	if err != nil {
		handleError(err, 1)
	}

	for _, s := range secrets {
		fmt.Fprintf(stdout, "%s\n", s)
	}
}

func versionRun(_ *cobra.Command, _ []string) {
	fmt.Fprintf(stdout, "%s\n", berglas.Version)
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

	ss := strings.SplitN(s, "/", 2)
	if len(ss) < 2 {
		return "", "", errors.Errorf("secret does not match format gs://<bucket>/<secret>: %s", s)
	}

	return ss[0], ss[1], nil
}

// handleError prints the error to stderr and exits with the given status.
func handleError(err error, status int) {
	fmt.Fprintf(stderr, "%s\n", err)
	os.Exit(status)
}
