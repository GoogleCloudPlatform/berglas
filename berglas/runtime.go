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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	cloudfunctions "google.golang.org/api/cloudfunctions/v1"
	iam "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
)

type RuntimeEnvironment interface {
	EnvVars(context.Context) (map[string]string, error)
}

func DetectRuntimeEnvironment() (RuntimeEnvironment, error) {
	if os.Getenv("X_GOOGLE_FUNCTION_NAME") != "" {
		return new(CloudFunctionEnv), nil
	}

	if os.Getenv("K_REVISION") != "" {
		return new(CloudRunEnv), nil
	}

	return nil, errors.New("unknown runtime")
}

// GCF
type CloudFunctionEnv struct{}

// EnvVars returns the list of envvars set on the function.
func (e *CloudFunctionEnv) EnvVars(ctx context.Context) (map[string]string, error) {
	// Compute the name of the function
	name := fmt.Sprintf("projects/%s/locations/%s/functions/%s",
		os.Getenv("X_GOOGLE_GCP_PROJECT"),
		os.Getenv("X_GOOGLE_FUNCTION_REGION"),
		os.Getenv("X_GOOGLE_FUNCTION_NAME"))

	client, err := cloudfunctions.NewService(ctx,
		option.WithUserAgent(UserAgent))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cloud functions client")
	}

	f, err := client.
		Projects.
		Locations.
		Functions.
		Get(name).
		Do()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cloud function environment variables")
	}

	return f.EnvironmentVariables, nil
}

type CloudRunEnv struct{}

// EnvVars returns the list of envvars set on the virtual machine.
func (e *CloudRunEnv) EnvVars(ctx context.Context) (map[string]string, error) {
	// TODO: replace with cloud run client library when it launches
	// TODO: stop hard-coding region when an envvar exists
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	region := "us-central1"
	revision := os.Getenv("K_REVISION")

	endpoint := fmt.Sprintf("https://run.googleapis.com/v1alpha1/projects/%s/locations/%s/revisions/%s",
		project, region, revision)

	client, err := google.DefaultClient(ctx, iam.CloudPlatformScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cloud run client")
	}
	client.Timeout = 15 * time.Second

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cloud run request")
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute cloud run request")
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cloud run response body")
	}

	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "failed to communicate with cloud run: %s", d)
	}

	var s cloudRunService
	if err := json.Unmarshal(d, &s); err != nil {
		return nil, err
	}

	envvars := make(map[string]string)
	for _, env := range s.Spec.Container.Env {
		envvars[env.Name] = env.Value
	}

	return envvars, nil
}

type cloudRunService struct {
	Spec struct {
		Container struct {
			Env []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"env"`
		} `json:"container"`
	} `json:"spec"`
}
