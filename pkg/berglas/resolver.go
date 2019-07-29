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
	"context"
	"os"

	"github.com/pkg/errors"
)

// Resolve parses and extracts a berglas reference. See Client.Resolve for more
// details and examples.
func Resolve(ctx context.Context, s string) ([]byte, error) {
	client, err := New(ctx)
	if err != nil {
		return nil, err
	}
	return client.Resolve(ctx, s)
}

// Resolve parses and extracts a berglas reference. The result is the plaintext
// secrets contents, or a path to the decrypted contents on disk.
func (c *Client) Resolve(ctx context.Context, s string) ([]byte, error) {
	ref, err := ParseReference(s)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse reference %s", s)
	}

	secret, err := c.Access(ctx, &AccessRequest{
		Bucket: ref.Bucket(),
		Object: ref.Object(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access secret %s/%s", ref.Bucket(), ref.Object())
	}
	plaintext := secret.Plaintext

	if pth := ref.Filepath(); pth != "" {
		f, err := os.OpenFile(ref.Filepath(), os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open filepath %s", pth)
		}

		if err := f.Chmod(0600); err != nil {
			return nil, errors.Wrapf(err, "failed to chmod filepath %s", pth)
		}

		if _, err := f.Write(plaintext); err != nil {
			return nil, errors.Wrapf(err, "failed to write secret to filepath %s", pth)
		}

		if err := f.Sync(); err != nil {
			return nil, errors.Wrapf(err, "failed to sync filepath %s", pth)
		}

		if err := f.Close(); err != nil {
			return nil, errors.Wrapf(err, "failed to close filepath %s", pth)
		}

		// Set the plaintext to the resulting file path
		plaintext = []byte(f.Name())
	}

	return plaintext, nil
}

// Replace parses a berglas reference and replaces it. See Client.Replace for
// more details and examples.
func Replace(ctx context.Context, key string) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Replace(ctx, key)
}

// ReplaceValue parses a berglas reference from value. If parsing and extraction
// is successful, this function sets the value of the environment variable to the
// resolved secret reference.
func (c *Client) ReplaceValue(ctx context.Context, key string, value string) error {
	plaintext, err := c.Resolve(ctx, os.Getenv(key))
	if err != nil {
		return err
	}

	if err := os.Setenv(key, string(plaintext)); err != nil {
		return errors.Wrapf(err, "failed to set %s", key)
	}
	return nil
}

// Replace parses a berglas reference from the environment variable at the
// given environment variable name. If parsing and extraction is successful,
// this function replaces the value of the environment variable to the resolved
// secret reference.
func (c *Client) Replace(ctx context.Context, key string) error {
	return c.ReplaceValue(ctx, key, os.Getenv(key))
}
