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

package auto

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/GoogleCloudPlatform/berglas/pkg/retry"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
)

var (
	retryBase  = 500 * time.Millisecond
	retryTries = 5

	// continueOnError controls whether Berglas should continue on error or panic.
	// The default behavior is to panic.
	continueOnError, _ = strconv.ParseBool(os.Getenv("BERGLAS_CONTINUE_ON_ERROR"))

	// logLevel is the log level to use.
	logLevel, _ = logrus.ParseLevel(os.Getenv("BERGLAS_LOG_LEVEL"))
)

func init() {
	ctx := context.Background()

	client, err := berglas.New(ctx)
	if err != nil {
		handleError(errors.Wrap(err, "failed to initialize berglas client"))
		return
	}
	client.SetLogLevel(logLevel)

	runtimeEnv, err := client.DetectRuntimeEnvironment()
	if err != nil {
		handleError(errors.Wrap(err, "failed to detect environment"))
		return
	}

	envvarRefs, err := Resolve(ctx, runtimeEnv)
	if err != nil {
		handleError(errors.Wrap(err, "failed to resolve environment variables"))
		return
	}

	if len(envvarRefs) == 0 {
		log.Printf("[WARN] berglas auto was included, but no secrets were found in the environment")
		return
	}

	for k, v := range envvarRefs {
		if err := client.ReplaceValue(ctx, k, v); err != nil {
			handleError(errors.Wrapf(err, "failed to set %s", k))
		}
	}
}

// Resolve resolves the environment variables. Importing the package calls
// Resolve. It's a separate method primarily for testing. Implementers should
// not call this method.
func Resolve(ctx context.Context, runtimeEnv berglas.RuntimeEnvironment) (map[string]string, error) {
	var envvars map[string]string
	var err error
	if err := retry.RetryFib(ctx, retryBase, retryTries, func() error {
		envvars, err = runtimeEnv.EnvVars(ctx)
		if err != nil {
			if terr, ok := errors.Cause(err).(*googleapi.Error); ok {
				// Do not retry 400-level errors
				if terr.Code >= 400 && terr.Code <= 499 {
					return terr
				}
			}

			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to find environment variables")
	}

	envvarRefs := make(map[string]string)
	for k, v := range envvars {
		if berglas.IsReference(v) {
			envvarRefs[k] = v
		}
	}

	return envvarRefs, nil
}

func handleError(err error) {
	log.Printf("%s\n", err)
	if !continueOnError {
		panic(err)
	}
}
