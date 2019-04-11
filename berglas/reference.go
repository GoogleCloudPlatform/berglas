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
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

const (
	// ReferencePrefix is the beginning identifier for a berglas secret reference.
	ReferencePrefix string = "berglas://"
)

// Reference is a parsed berglas reference.
type Reference struct {
	bucket   string
	object   string
	filepath string
}

// Bucket is the storage bucket where the secret lives.
func (r *Reference) Bucket() string {
	return r.bucket
}

// Object is the name of the secret in the storage bucket.
func (r *Reference) Object() string {
	return r.object
}

// Filepath is the disk to write the reference, if any.
func (r *Reference) Filepath() string {
	return r.filepath
}

// IsReference returns true if the given string looks like a berglas reference,
// false otherwise.
func IsReference(s string) bool {
	return strings.HasPrefix(s, ReferencePrefix)
}

// ParseReference parses a secret ref of the format `berglas://bucket/secret`
// and returns a structure representing that information.
func ParseReference(s string) (*Reference, error) {
	// Make sure it's a reference
	if !IsReference(s) {
		return nil, errors.New("not a berglas reference")
	}

	// Remove the berglas:// prefix
	s = strings.TrimPrefix(s, ReferencePrefix)

	// Remove any leading slashes (it messes up bucket names)
	s = strings.TrimPrefix(s, "/")

	// Parse the remainder as a URL to extract any query params
	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse secrets reference as url")
	}

	// Separate bucket from path
	ss := strings.SplitN(u.Path, "/", 2)
	if len(ss) < 2 {
		return nil, errors.Errorf("invalid secret format %q", s)
	}

	// Create the reference
	var r Reference
	r.bucket = ss[0]
	r.object = ss[1]

	// Parse out destination
	switch d := u.Query().Get("destination"); d {
	case "":
		// keep in environment variable
	case "tmpfile", "tempfile":
		// create a tempfile for the path
		f, err := ioutil.TempFile("", "berglas-")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create tempfile for secret")
		}
		if err := f.Close(); err != nil {
			return nil, errors.Wrap(err, "failed to close tempfile for secret")
		}
		r.filepath = f.Name()
	default:
		// assume file path
		r.filepath = d
	}

	return &r, nil
}
