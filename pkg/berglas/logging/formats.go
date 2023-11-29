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
	"slices"
	"strings"
)

// Format represents the logging format.
type Format string

const (
	FormatJSON = Format("JSON")
	FormatText = Format("TEXT")
)

const (
	formatJSONName = string(FormatJSON)
	formatTextName = string(FormatText)
)

var formatNames = []string{
	formatJSONName,
	formatTextName,
}

// FormatNames returns the list of all log format names.
func FormatNames() []string {
	return slices.Clone(formatNames)
}

// LookupFormat attempts to get the formatter that corresponds to the given
// name. If no such formatter exists, it returns an error. If the empty string
// is given, it returns the JSON formatter.
func LookupFormat(name string) (Format, error) {
	switch v := strings.ToUpper(strings.TrimSpace(name)); v {
	case "":
		return FormatJSON, nil
	case formatJSONName:
		return FormatJSON, nil
	case formatTextName, "CONSOLE":
		return FormatText, nil
	default:
		return "", fmt.Errorf("no such format %q, valid formats are %q", name, formatNames)
	}
}
