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

package auto

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

type testRuntime struct{}

func (r *testRuntime) EnvVars(ctx context.Context) (map[string]string, error) {
	return map[string]string{
		"FOO":     "bar",
		"API_KEY": "berglas://foo/bar",
	}, nil
}

type errorRuntime struct{}

func (r *errorRuntime) EnvVars(ctx context.Context) (map[string]string, error) {
	return nil, errors.New("an error occurred")
}

type retryableErrorRuntime struct {
	sync.Mutex
	tries int
}

func (r *retryableErrorRuntime) EnvVars(ctx context.Context) (map[string]string, error) {
	r.Lock()
	defer r.Unlock()

	if r.tries >= 3 {
		return map[string]string{
			"FOO":     "bar",
			"API_KEY": "berglas://foo/bar",
		}, nil
	}
	r.tries++
	return nil, &googleapi.Error{Code: 500, Message: "retry later"}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	retryBase = 5 * time.Millisecond
	retryTries = 5

	t.Run("normal", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		refs, err := Resolve(ctx, new(testRuntime))
		if err != nil {
			t.Fatal(err)
		}
		exp := map[string]string{"API_KEY": "berglas://foo/bar"}
		if !reflect.DeepEqual(refs, exp) {
			t.Errorf("expected %q to be %q", refs, exp)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		refs, err := Resolve(ctx, new(errorRuntime))
		if err == nil {
			t.Fatal("expected error")
		}
		if refs != nil {
			t.Errorf("expected err to be nil")
		}
	})

	t.Run("retryable", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		refs, err := Resolve(ctx, new(retryableErrorRuntime))
		if err != nil {
			t.Fatal(err)
		}
		exp := map[string]string{"API_KEY": "berglas://foo/bar"}
		if !reflect.DeepEqual(refs, exp) {
			t.Errorf("expected %q to be %q", refs, exp)
		}
	})
}
