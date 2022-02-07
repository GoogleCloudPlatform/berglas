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

import "errors"

const (
	// errSecretAlreadyExists is the error returned if a secret already exists.
	errSecretAlreadyExists = Error("secret already exists")

	// errSecretDoesNotExist is the error returned if a secret does not exist.
	errSecretDoesNotExist = Error("secret does not exist")

	// errSecretModified is the error returned when preconditions fail.
	errSecretModified = Error("secret modified between read and write")
)

// Error is an error from Berglas.
type Error string

// Error implements the error interface.
func (e Error) Error() string {
	return string(e)
}

// IsSecretAlreadyExistsErr returns true if the given error means that the
// secret already exists.
func IsSecretAlreadyExistsErr(err error) bool {
	return errors.Is(err, errSecretAlreadyExists)
}

// IsSecretDoesNotExistErr returns true if the given error means that the secret
// does not exist.
func IsSecretDoesNotExistErr(err error) bool {
	return errors.Is(err, errSecretDoesNotExist)
}

// IsSecretModifiedErr returns true if the given error means that the secret
// was modified (CAS failure).
func IsSecretModifiedErr(err error) bool {
	return errors.Is(err, errSecretModified)
}
