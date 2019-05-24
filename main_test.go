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

package main

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func Test_readData(t *testing.T) {
	t.Parallel()

	t.Run("text", func(t *testing.T) {
		t.Parallel()

		r, err := readData("blob")
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := r, []byte("blob"); !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("file", func(t *testing.T) {
		t.Parallel()

		f, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		if err := ioutil.WriteFile(f.Name(), []byte("fileblob"), 0600); err != nil {
			t.Fatal(err)
		}

		r, err := readData("@" + f.Name())
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := r, []byte("fileblob"); !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})

	t.Run("escape", func(t *testing.T) {
		t.Parallel()

		r, err := readData("\\@file.txt")
		if err != nil {
			t.Fatal(err)
		}

		if act, exp := r, []byte("@file.txt"); !bytes.Equal(act, exp) {
			t.Errorf("expected %q to be %q", act, exp)
		}
	})
}

func Test_parseRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		s              string
		bucket, secret string
		err            bool
	}{
		{
			"empty",
			"",
			"", "",
			true,
		},
		{
			"space",
			"    ",
			"", "",
			true,
		},
		{
			"no-slash",
			"foo",
			"", "",
			true,
		},
		{
			"slash",
			"foo/bar",
			"foo", "bar",
			false,
		},
		{
			"gs-prefix",
			"gs://foo/bar",
			"foo", "bar",
			false,
		},
		{
			"folder",
			"gs://foo/bar/baz/bacon",
			"foo", "bar/baz/bacon",
			false,
		},
		{
			"berglas-prefix",
			"berglas://foo/bar",
			"foo", "bar",
			false,
		},
		{
			"berglas + folder",
			"berglas://foo/bar/baz/bacon",
			"foo", "bar/baz/bacon",
			false,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bucket, secret, err := parseRef(tc.s)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if act, exp := bucket, tc.bucket; act != exp {
				t.Errorf("expected %q to be %q", act, exp)
			}

			if act, exp := secret, tc.secret; act != exp {
				t.Errorf("expected %q to be %q", act, exp)
			}
		})
	}
}
