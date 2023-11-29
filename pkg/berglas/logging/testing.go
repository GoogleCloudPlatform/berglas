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

package logging

import (
	"io"
	"log/slog"
	"math"
	"testing"
)

// TestLogger creates a new logger for use in tests. It will only log messages
// when tests fail and the tests were run with verbose (-v).
func TestLogger(tb testing.TB) *slog.Logger {
	tb.Helper()

	w := &testingWriter{tb}

	// Use the lowest possible level (aka log everything). Slog levels are
	// arbitrary integers, so by choosing the lowest possible integer, we will
	// catch every possible log.
	level := slog.Level(math.MinInt)

	return slog.New(NewLevelHandler(level, slog.NewTextHandler(w, &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Drop time key since the test failures will include timestamps
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}

			return cloudLoggingAttrsEncoder()(groups, a)
		},
	})))
}

var _ io.Writer = (*testingWriter)(nil)

type testingWriter struct {
	tb testing.TB
}

func (t *testingWriter) Write(b []byte) (int, error) {
	if !testing.Verbose() {
		return 0, nil
	}

	t.tb.Log(string(b))
	return len(b), nil
}
