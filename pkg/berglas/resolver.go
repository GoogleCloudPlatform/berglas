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
	"runtime"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// chmodSupported indicates whether the OS supports chmod
const chmodSupported = runtime.GOOS != "windows" && runtime.GOOS != "plan9"

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
	logger := c.Logger().WithFields(logrus.Fields{
		"reference": s,
	})

	logger.Debug("resolve.start")
	defer logger.Debug("resolve.finish")

	ref, err := ParseReference(s)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse reference %s", s)
	}

	var req accessRequest
	switch ref.Type() {
	case ReferenceTypeSecretManager:
		req = &SecretManagerAccessRequest{
			Project: ref.Project(),
			Name:    ref.Name(),
			Version: ref.Version(),
		}
	case ReferenceTypeStorage:
		req = &StorageAccessRequest{
			Bucket:     ref.Bucket(),
			Object:     ref.Object(),
			Generation: ref.Generation(),
		}
	}

	plaintext, err := c.Access(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access secret %s/%s", ref.Bucket(), ref.Object())
	}

	if pth := ref.Filepath(); pth != "" {
		logger.WithField("filepath", pth).Debug("writing to filepath")

		f, err := os.OpenFile(ref.Filepath(), os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open filepath %s", pth)
		}

		if chmodSupported {
			if err := f.Chmod(0600); err != nil {
				return nil, errors.Wrapf(err, "failed to chmod filepath %s", pth)
			}
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
