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
	"reflect"
	"testing"
)

func TestParseReference(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		s    string
		exp  *Reference
		err  bool
	}{
		{
			"empty",
			"",
			nil,
			true,
		},
		{
			"space",
			"    ",
			nil,
			true,
		},
		{
			"no-slash",
			"foo",
			nil,
			true,
		},
		{
			"no-prefix",
			"foo/bar",
			nil,
			true,
		},

		// Secret Manager
		{
			"sm-no-secret",
			"sm://foo",
			nil,
			true,
		},
		{
			"sm-prefix",
			"sm://foo/bar",
			&Reference{
				project: "foo",
				name:    "bar",
				typ:     ReferenceTypeSecretManager,
			},
			false,
		},
		{
			"folder",
			"sm://foo/bar/baz/bacon", // secret names cannot be nested
			nil,
			true,
		},
		{
			"destination_path",
			"sm://foo/bar?destination=/var/foo",
			&Reference{
				project:  "foo",
				name:     "bar",
				filepath: "/var/foo",
				typ:      ReferenceTypeSecretManager,
			},
			false,
		},
		{
			"destination_path",
			"sm://foo/bar?destination=/var/foo#12",
			&Reference{
				project:  "foo",
				name:     "bar",
				version:  "12",
				filepath: "/var/foo",
				typ:      ReferenceTypeSecretManager,
			},
			false,
		},

		// Storage
		{
			"berglas-no-secret",
			"berglas://foo",
			nil,
			true,
		},
		{
			"berglas-prefix",
			"berglas://foo/bar",
			&Reference{
				bucket: "foo",
				object: "bar",
				typ:    ReferenceTypeStorage,
			},
			false,
		},
		{
			"folder",
			"berglas://foo/bar/baz/bacon",
			&Reference{
				bucket: "foo",
				object: "bar/baz/bacon",
				typ:    ReferenceTypeStorage,
			},
			false,
		},
		{
			"destination_path",
			"berglas://foo/bar?destination=/var/foo",
			&Reference{
				bucket:   "foo",
				object:   "bar",
				filepath: "/var/foo",
				typ:      ReferenceTypeStorage,
			},
			false,
		},
		{
			"destination_path",
			"berglas://foo/bar?destination=/var/foo#1563925173373377",
			&Reference{
				bucket:     "foo",
				object:     "bar",
				generation: 1563925173373377,
				filepath:   "/var/foo",
				typ:        ReferenceTypeStorage,
			},
			false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			act, err := ParseReference(tc.s)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(act, tc.exp) {
				t.Errorf("expected %#v to be %#v", act, tc.exp)
			}
		})
	}
}

func TestReference_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ref  *Reference
		exp  string
	}{
		{
			"sm_plain",
			&Reference{project: "project", name: "secret", typ: ReferenceTypeSecretManager},
			"sm://project/secret",
		},
		{
			"sm_version",
			&Reference{project: "project", name: "secret", version: "123", typ: ReferenceTypeSecretManager},
			"sm://project/secret#123",
		},
		{
			"berglas_plain",
			&Reference{bucket: "bucket", object: "secret", typ: ReferenceTypeStorage},
			"berglas://bucket/secret",
		},
		{
			"berglas_generation",
			&Reference{bucket: "bucket", object: "secret", generation: 1234567890, typ: ReferenceTypeStorage},
			"berglas://bucket/secret#1234567890",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			act, exp := tc.ref.String(), tc.exp
			if act != tc.exp {
				t.Errorf("expected %#v to be %#v", act, exp)
			}
		})
	}
}
