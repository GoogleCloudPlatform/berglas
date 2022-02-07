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

package berglas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// SetLogger is a lower-level library that allows injecting a custom logger.
func (c *Client) SetLogger(l *logrus.Logger) {
	c.loggerLock.Lock()
	c.logger = l
	c.loggerLock.Unlock()
}

// Logger returns the logger instance attached to this client.
func (c *Client) Logger() *logrus.Logger {
	c.loggerLock.RLock()
	l := c.logger
	c.loggerLock.RUnlock()
	return l
}

// SetLogLevel is a high-level function for setting the log level.
func (c *Client) SetLogLevel(level logrus.Level) {
	c.loggerLock.Lock()
	c.logger.SetLevel(level)
	c.loggerLock.Unlock()
}

// SetLogOutput is a high-level function for setting log output destination.
func (c *Client) SetLogOutput(out io.Writer) {
	c.loggerLock.Lock()
	c.logger.SetOutput(out)
	c.loggerLock.Unlock()
}

// SetLogFormatter sets the format of the logger. Use
func (c *Client) SetLogFormatter(formatter logrus.Formatter) {
	c.loggerLock.Lock()
	c.logger.SetFormatter(formatter)
	c.loggerLock.Unlock()
}

// LogFormatterStackdriver is a logrus-compatible formatter that formats entries
// in a Stackdriver-compatible way. It specifically produces JSON structured logs.
type LogFormatterStackdriver struct{}

// Format implements logrus formatter.
func (f *LogFormatterStackdriver) Format(entry *logrus.Entry) ([]byte, error) {
	// We'll usually add 3 and at most 4 more things to the map, so allocate that
	// now to avoid a re-allocation when we append later.
	data := make(logrus.Fields, len(entry.Data)+4)

	// Add all the entry fields
	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}

	// Set timestamp in preferred format
	data["timestamp"] = entry.Time.Format(time.RFC3339)

	// Set message
	data["message"] = entry.Message

	// Set severity based on level
	switch entry.Level {
	case logrus.PanicLevel:
		data["severity"] = "ALERT"
	case logrus.FatalLevel:
		data["severity"] = "CRITICAL"
	case logrus.ErrorLevel:
		data["severity"] = "ERROR"
	case logrus.WarnLevel:
		data["severity"] = "WARNING"
	case logrus.InfoLevel:
		data["severity"] = "INFO"
	case logrus.DebugLevel:
		data["severity"] = "DEBUG"
	case logrus.TraceLevel:
		data["severity"] = "DEBUG"
	}

	// Check if there's a caller
	if entry.HasCaller() {
		data["logging.googleapis.com/sourceLocation"] = map[string]string{
			"file":     entry.Caller.File,
			"line":     strconv.FormatInt(int64(entry.Caller.Line), 10),
			"function": entry.Caller.Function,
		}
	}

	// Allow an entry to override the buffer (useful for testing)
	b := entry.Buffer
	if b == nil {
		b = new(bytes.Buffer)
	}

	// JSON!
	if err := json.NewEncoder(b).Encode(data); err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return b.Bytes(), nil
}
