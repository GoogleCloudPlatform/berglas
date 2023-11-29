// Copyright 2023 The Berglas Authors
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

// Package logging is an opinionated structured logging library based on
// [log/slog].
//
// This package also aliases most top-level functions in [log/slog] to reduce
// the need to manage the additional import.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

// contextKey is a private string type to prevent collisions in the context map.
type contextKey string

// loggerKey points to the value in the context where the logger is stored.
const loggerKey = contextKey("logger")

var defaultLoggerOnce = sync.OnceValue(func() *slog.Logger {
	l, err := New(os.Stderr, "warning", "json", false)
	if err != nil {
		panic(fmt.Errorf("failed to create logger: %w", err))
	}
	return l
})

// New creates a new logger in the specified format and writes to the provided
// writer at the provided level. Use the returned leveler to dynamically change
// the level to a different value after creation.
//
// If debug is true, the logging level is set to the lowest possible value
// (meaning all messages will be printed), and the output will include source
// information. This is very expensive, and you should not enable it unless
// actively debugging.
//
// It returns the configured logger and a leveler which can be used to change
// the logger's level dynamically. The leveler does not require locking to
// change the level.
func New(w io.Writer, logLevel, logFormat string, debug bool) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{
		ReplaceAttr: cloudLoggingAttrsEncoder(),
	}

	level, err := LookupLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid value %q for log level: %w", logLevel, err)
	}

	format, err := LookupFormat(logFormat)
	if err != nil {
		return nil, fmt.Errorf("invalid value %q for log format: %w", logFormat, err)
	}

	// Enable the most detailed log level and add source information in debug
	// mode.
	if debug {
		opts.AddSource = true
		level = math.MinInt
	}

	switch format {
	case FormatJSON:
		return slog.New(NewLevelHandler(level, slog.NewJSONHandler(w, opts))), nil
	case FormatText:
		return slog.New(NewLevelHandler(level, slog.NewTextHandler(w, opts))), nil
	default:
		return nil, fmt.Errorf("unknown log format %q", format)
	}
}

// SetLevel adjusts the level on the provided logger. The handler on the given
// logger must be a [LevelableHandler] or else this function panics. If you
// created a logger through this package, it will automatically satisfy that
// interface.
//
// This function is safe for concurrent use.
//
// It returns the provided logger for convenience and easier chaining.
func SetLevel(logger *slog.Logger, level slog.Level) *slog.Logger {
	if typ, ok := logger.Handler().(LevelableHandler); ok {
		typ.SetLevel(level)
		return logger
	}

	panic("handler is not capable of setting levels")
}

// WithLogger creates a new context with the provided logger attached.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext returns the logger stored in the context. If no such logger
// exists, a default logger is returned.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}

	return defaultLoggerOnce()
}

// cloudLoggingAttrsEncoder updates the [slog.Record] attributes to match the
// key names and [format for Google Cloud Logging].
//
// [format for Google Cloud Logging]: https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
func cloudLoggingAttrsEncoder() func([]string, slog.Attr) slog.Attr {
	const (
		keySeverity = "severity"
		keyError    = "error"
		keyMessage  = "message"
		keySource   = "logging.googleapis.com/sourceLocation"
	)

	return func(groups []string, a slog.Attr) slog.Attr {
		// Google Cloud Logging uses "severity" instead of "level":
		// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#logseverity
		if a.Key == slog.LevelKey {
			a.Key = keySeverity

			// Use the custom level names to match Google Cloud logging.
			val := a.Value.Any()
			typ, ok := val.(slog.Level)
			if !ok {
				panic(fmt.Sprintf("level is not slog.Level (got %T)", val))
			}
			a.Value = LevelSlogValue(typ)
		}

		// Google Cloud Logging uses "message" instead of "msg":
		// https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		if a.Key == slog.MessageKey {
			a.Key = keyMessage
		}

		// Google Cloud Logging uses "logging.google..." instead of "source":
		// https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		if a.Key == slog.SourceKey {
			a.Key = keySource
		}

		// Re-format durations to be their string format.
		if a.Value.Kind() == slog.KindDuration {
			val := a.Value.Duration()
			a.Value = slog.StringValue(humanDuration(val))
		}

		return a
	}
}

// humanDuration prints the time duration without zero elements, rounding to
// seconds. It trims any trailing zero parts, so "1h0m0s" becomes "1h".
func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)

	if d == 0 {
		return "0s"
	}

	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if idx := strings.Index(s, "h0m"); idx > 0 {
		s = s[:idx+1] + s[idx+3:]
	}
	return s
}
