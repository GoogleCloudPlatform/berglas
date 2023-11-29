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
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/GoogleCloudPlatform/berglas/pkg/berglas/logging"
)

var (
	// continueOnError controls whether Berglas should continue on error or panic.
	// The default behavior is to panic.
	continueOnError, _ = strconv.ParseBool(os.Getenv("BERGLAS_CONTINUE_ON_ERROR"))

	// logLevel is the log level to use.
	logLevel = os.Getenv("BERGLAS_LOG_LEVEL")

	// logFormat is the log level to use.
	logFormat = os.Getenv("BERGLAS_LOG_FORMAT")
)

func init() {
	ctx, done := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer done()

	logger, err := logging.New(os.Stderr, logLevel, logFormat, false)
	if err != nil {
		handleError(fmt.Errorf("failed to initialize logger: %w", err))
		return
	}
	ctx = logging.WithLogger(ctx, logger)

	client, err := berglas.New(ctx)
	if err != nil {
		handleError(fmt.Errorf("failed to initialize berglas client: %w", err))
		return
	}

	for _, e := range os.Environ() {
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
			handleError(fmt.Errorf("failed to parse %q: %w", k, err))
			continue
		}

		if err := os.Setenv(k, string(s)); err != nil {
			handleError(fmt.Errorf("failed to set %q: %w", k, err))
			continue
		}
	}
}

func handleError(err error) {
	log.Printf("%s\n", err)
	if !continueOnError {
		panic(err)
	}
}
