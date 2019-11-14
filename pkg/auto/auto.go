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

// Package auto automatically parses berglas references when imported.
//
//     import (
//       _ "github.com/GoogleCloudPlatform/berglas/pkg/auto"
//     )
//
// Set environment variables on your deployment using the berglas:// prefix in
// the format:
//
//     berglas://<bucket>/<secret>?<params>
//
// - "bucket" is the name of the Google Cloud Storage bucket where secrets
// are stored
// - "secret" is the path to the full path to a secret inside the bucket
// - "params" are URL query parameters that configure behavior
//
// Examples:
//
//     berglas://my-bucket/my-secret
//     berglas://my-bucket/path/to/secret?destination=tempfile
//     berglas://my-bucket/path/to/secret?destination=/var/foo/bar
//
// On init, the package queries the list of configured environment variables
// against the metadata service. If environment variables match, their values
// are automatically replaced with the secret value.
//
//
// By default, any errors result in a panic. If you want the function to
// continue executing even if resolution or communication fails, set the
// environment variable `BERGLAS_CONTINUE_ON_ERROR` to `true` or do not use the
// auto package.
//
// To see log output, set `BERGLAS_LOG_LEVEL` to "trace" or "debug".
package auto
