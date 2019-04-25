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
	"time"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/GoogleCloudPlatform/berglas/pkg/retry"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
)

var (
	retryBase  = 500 * time.Millisecond
	retryTries = 5
)

func init() {
	ctx := context.Background()

	runtimeEnv, err := berglas.DetectRuntimeEnvironment()
	if err != nil {
		log.Printf("[ERR] failed to detect environment: %s", err)
		return
	}

	envvarRefs, err := Resolve(ctx, runtimeEnv)
	if err != nil {
		log.Printf("[ERR] %s", err)
		return
	}

	if len(envvarRefs) == 0 {
		log.Printf("[WARN] berglas auto was included, but no secrets were found in the environment")
		return
	}

	client, err := berglas.New(ctx)
	if err != nil {
		log.Printf("[ERR] failed to initialize berglas client: %s", err)
		return
	}

	for k := range envvarRefs {
		if err := client.Replace(ctx, k); err != nil {
			log.Printf("[ERR] failed to set %s: %s", k, err)
		}
	}
}

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
