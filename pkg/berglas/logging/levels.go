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
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

const (
	LevelDebug     = slog.Level(-4)
	LevelInfo      = slog.Level(0)
	LevelNotice    = slog.Level(2)
	LevelWarning   = slog.Level(4)
	LevelError     = slog.Level(8)
	LevelEmergency = slog.Level(12)
)

const (
	levelUnknownName   = "UNKNOWN"
	levelDebugName     = "DEBUG"
	levelInfoName      = "INFO"
	levelNoticeName    = "NOTICE"
	levelWarningName   = "WARNING"
	levelErrorName     = "ERROR"
	levelEmergencyName = "EMERGENCY"
)

var (
	levelUnknownSlogValue   = slog.StringValue(levelUnknownName)
	levelDebugSlogValue     = slog.StringValue(levelDebugName)
	levelInfoSlogValue      = slog.StringValue(levelInfoName)
	levelNoticeSlogValue    = slog.StringValue(levelNoticeName)
	levelWarningSlogValue   = slog.StringValue(levelWarningName)
	levelErrorSlogValue     = slog.StringValue(levelErrorName)
	levelEmergencySlogValue = slog.StringValue(levelEmergencyName)
)

var levelNames = []string{
	levelDebugName,
	levelInfoName,
	levelNoticeName,
	levelWarningName,
	levelErrorName,
	levelEmergencyName,
}

// LevelNames returns the list of all log level names.
func LevelNames() []string {
	return slices.Clone(levelNames)
}

// LookupLevel attempts to get the level that corresponds to the given name. If
// no such level exists, it returns an error. If the empty string is given, it
// returns Info level.
func LookupLevel(name string) (slog.Level, error) {
	switch v := strings.ToUpper(strings.TrimSpace(name)); v {
	case "":
		return LevelInfo, nil
	case levelDebugName:
		return LevelDebug, nil
	case levelInfoName:
		return LevelInfo, nil
	case levelNoticeName:
		return LevelNotice, nil
	case levelWarningName, "WARN": // "WARN" maintains compat with old logger
		return LevelWarning, nil
	case levelErrorName, "ERR", "FATAL": // "ERR" and "FATAL" maintains compat with old logger
		return LevelError, nil
	case levelEmergencyName:
		return LevelEmergency, nil
	default:
		return 0, fmt.Errorf("no such level %q, valid levels are %q", name, levelNames)
	}
}

// LevelSlogValue returns the [slog.Value] representation of the level.
func LevelSlogValue(l slog.Level) slog.Value {
	switch l {
	case LevelDebug:
		return levelDebugSlogValue
	case LevelInfo:
		return levelInfoSlogValue
	case LevelNotice:
		return levelNoticeSlogValue
	case LevelWarning:
		return levelWarningSlogValue
	case LevelError:
		return levelErrorSlogValue
	case LevelEmergency:
		return levelEmergencySlogValue
	default:
		return levelUnknownSlogValue
	}
}

// LevelString returns the string representation of the given level. Note this
// is different from calling String() on the Level, which uses the slog
// implementation.
func LevelString(l slog.Level) string {
	switch l {
	case LevelDebug:
		return levelDebugName
	case LevelInfo:
		return levelInfoName
	case LevelNotice:
		return levelNoticeName
	case LevelWarning:
		return levelWarningName
	case LevelError:
		return levelErrorName
	case LevelEmergency:
		return levelEmergencyName
	default:
		return levelUnknownName
	}
}
