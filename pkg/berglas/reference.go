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
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// ReferencePrefixStorage is the prefix for berglas references
	ReferencePrefixStorage = "berglas://"

	// ReferencePrefixSecretManager is the prefix for secret manager references
	ReferencePrefixSecretManager = "sm://"
)

// ReferenceType is the type of Berglas reference. It is used to distinguish
// between different source types.
type ReferenceType int8

const (
	_ ReferenceType = iota
	ReferenceTypeSecretManager
	ReferenceTypeStorage
)

// Reference is a parsed berglas reference.
type Reference struct {
	// Storage properties
	bucket     string
	object     string
	generation int64

	// Secret Manager properties
	project string
	name    string
	version string

	// Common properties
	typ      ReferenceType
	filepath string
}

// Bucket is the storage bucket where the secret lives. This is only set on
// Cloud Storage secrets.
func (r *Reference) Bucket() string {
	return r.bucket
}

// Object is the name of the secret in the storage bucket. This is only set on
// Cloud Storage secrets.
func (r *Reference) Object() string {
	return r.object
}

// Generation is the secret generation, if any. This is only set on Cloud
// Storage secrets.
func (r *Reference) Generation() int64 {
	return r.generation
}

// Project is the GCP project where the secret lives. This is only set on Secret
// Manager secrets.
func (r *Reference) Project() string {
	return r.project
}

// Name is the name. This is only set on Secret Manager secrets.
func (r *Reference) Name() string {
	return r.name
}

// Version is the version. This is only set on Secret Manager secrets.
func (r *Reference) Version() string {
	return r.version
}

// Filepath is the disk to write the reference, if any.
func (r *Reference) Filepath() string {
	return r.filepath
}

// Type is the type of reference, used for switching.
func (r *Reference) Type() ReferenceType {
	return r.typ
}

// String prints the best representation for the secret.
func (r *Reference) String() string {
	switch r.typ {
	case ReferenceTypeSecretManager:
		if r.version == "" {
			return fmt.Sprintf("sm://%s/%s", r.project, r.name)
		} else {
			return fmt.Sprintf("sm://%s/%s#%s", r.project, r.name, r.version)
		}
	case ReferenceTypeStorage:
		if r.generation == 0 {
			return fmt.Sprintf("berglas://%s/%s", r.bucket, r.object)
		} else {
			return fmt.Sprintf("berglas://%s/%s#%d", r.bucket, r.object, r.generation)
		}
	default:
		return fmt.Sprintf("unknown type %T", r.typ)
	}
}

// IsReference returns true if the given string looks like a berglas or secret
// manager reference.
func IsReference(s string) bool {
	return IsStorageReference(s) || IsSecretManagerReference(s)
}

// IsStorageReference returns true if the given string looks like a
// Cloud Storage reference.
func IsStorageReference(s string) bool {
	return strings.HasPrefix(s, ReferencePrefixStorage)
}

// IsSecretManagerReference returns true if the given string looks like a secret
// manager reference.
func IsSecretManagerReference(s string) bool {
	return strings.HasPrefix(s, ReferencePrefixSecretManager)
}

// ParseReference parses a secret ref of the format `berglas://bucket/secret` or
// `sm://project/secret` and returns a structure representing that information.
func ParseReference(s string) (*Reference, error) {
	// Make sure it's a reference and strip out the prefix
	switch {
	case IsSecretManagerReference(s):
		s = strings.TrimPrefix(s, ReferencePrefixSecretManager)
		return secretManagerParseReference(s)
	case IsStorageReference(s):
		s = strings.TrimPrefix(s, ReferencePrefixStorage)
		return storageParseReference(s)
	default:
		return nil, fmt.Errorf("not a storage or secret manager reference")
	}
}

func secretManagerParseReference(s string) (*Reference, error) {
	// Parse the remainder as a URL to extract any query params
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secrets reference as url: %w", err)
	}

	// Separate project from secret
	ss := strings.SplitN(u.Path, "/", 2)
	if len(ss) < 2 {
		return nil, fmt.Errorf("invalid secret format %q", s)
	}

	// Create the reference
	var r Reference
	r.typ = ReferenceTypeSecretManager
	r.project = ss[0]
	r.name = ss[1]

	if u.Fragment != "" {
		r.version = u.Fragment
	}

	// Secrets cannot be nested
	if strings.Contains(r.name, "/") {
		return nil, fmt.Errorf("invalid secret name %q", r.name)
	}

	// Parse destination
	path, err := refExtractFilepath(r.name, u.Query().Get("destination"))
	if err != nil {
		return nil, err
	}
	r.filepath = path

	return &r, nil
}

func storageParseReference(s string) (*Reference, error) {
	// Remove any leading slashes (it messes up bucket names)
	s = strings.TrimPrefix(s, "/")

	// Parse the remainder as a URL to extract any query params
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secrets reference as url: %w", err)
	}

	// Separate bucket from path
	ss := strings.SplitN(u.Path, "/", 2)
	if len(ss) < 2 {
		return nil, fmt.Errorf("invalid secret format %q", s)
	}

	// Create the reference
	var r Reference
	r.typ = ReferenceTypeStorage
	r.bucket = ss[0]
	r.object = ss[1]

	if u.Fragment != "" {
		if generation, err := strconv.ParseInt(u.Fragment, 0, 64); err == nil {
			r.generation = generation
		}
	}

	// Parse destination
	path, err := refExtractFilepath(r.object, u.Query().Get("destination"))
	if err != nil {
		return nil, err
	}
	r.filepath = path

	return &r, nil
}

func refExtractFilepath(object, s string) (string, error) {
	switch s {
	case "tmpfile", "tempfile":
		suffix := filepath.Ext(object)
		pattern := fmt.Sprintf("berglas-*%s", suffix)
		// create a tempfile for the path
		f, err := ioutil.TempFile("", pattern)
		if err != nil {
			return "", fmt.Errorf("failed to create tempfile for secret: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("failed to close tempfile for secret: %w", err)
		}
		return f.Name(), nil
	default:
		// assume file path - this works if s is "" too
		return s, nil
	}
}
