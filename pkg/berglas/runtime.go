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
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	cloudfunctions "google.golang.org/api/cloudfunctions/v1"
	iam "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	"google.golang.org/appengine"
)

// RuntimeEnvironment is an interface for getting the envvars of a runtime.
type RuntimeEnvironment interface {
	EnvVars(context.Context) (map[string]string, error)
}

// DetectRuntimeEnvironment returns the most like runtime environment.
func DetectRuntimeEnvironment() (RuntimeEnvironment, error) {
	if os.Getenv("X_GOOGLE_FUNCTION_NAME") != "" {
		return new(cloudFunctionEnv), nil
	}

	if os.Getenv("K_REVISION") != "" {
		return new(cloudRunEnv), nil
	}

	if appengine.IsAppEngine() {
		return new(gaeEnv), nil
	}

	return nil, errors.New("unknown runtime")
}

// cloudFunctionEnv is a Google Cloud Functions environment.
type cloudFunctionEnv struct{}

// EnvVars returns the list of envvars set on the function.
func (e *cloudFunctionEnv) EnvVars(ctx context.Context) (map[string]string, error) {
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

// cloudRunEnv is a Google Cloud Run environment.
type cloudRunEnv struct{}

// EnvVars returns the list of envvars set on the virtual machine.
func (e *cloudRunEnv) EnvVars(ctx context.Context) (map[string]string, error) {
	revision := os.Getenv("K_REVISION")

	project, err := valueFromMetadata(ctx, "project/project-id")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get project ID")
	}

	zone, err := valueFromMetadata(ctx, "instance/zone")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get zone")
	}
	zone = path.Base(zone)

	region := ""
	if i := strings.LastIndex(zone, "-"); i > -1 {
		region = zone[0:i]
	}
	if region == "" {
		return nil, errors.Errorf("failed to extract region from zone: %s", zone)
	}

	endpoint := fmt.Sprintf("https://%s-run.googleapis.com/apis/serving.knative.dev/v1alpha1/namespaces/%s/revisions/%s",
		region, project, revision)

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
		return nil, errors.Errorf("failed to communicate with cloud run: %s", d)
	}

	var s cloudRunService
	if err := json.Unmarshal(d, &s); err != nil {
		return nil, err
	}

	// v1alpha1 API can return a list or single container. When we migrate to the
	// beta API, it will always return a list.
	container := s.Spec.Container
	if len(s.Spec.Containers) > 0 {
		container = s.Spec.Containers[0]
	}

	envvars := make(map[string]string)
	for _, env := range container.Env {
		envvars[env.Name] = env.Value
	}

	return envvars, nil
}

// valueFromMetadata queries the GCP metadata service to get information at the
// specified path.
func valueFromMetadata(ctx context.Context, path string) (string, error) {
	path = fmt.Sprintf("http://metadata.google.internal/computeMetadata/v1/%s", path)

	client, err := google.DefaultClient(ctx, iam.CloudPlatformScope)
	if err != nil {
		return "", errors.Wrap(err, "failed to create cloud run client")
	}
	client.Timeout = 15 * time.Second

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create metadata request for %s", path)
	}
	req.Header.Set("Metadata-Flavor", "Google")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get metadata for %s", path)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read body for metadata for %s", path)
	}

	if resp.StatusCode != 200 {
		return "", errors.Errorf("failed to get metadata for %s: %s", path, d)
	}

	return string(d), nil
}

type cloudRunService struct {
	Spec struct {
		Containers []cloudRunContainer `json:"containers"`
		Container  cloudRunContainer   `json:"container"`
	} `json:"spec"`
}

type cloudRunContainer struct {
	Env []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
}

// gaeEnv is a Google App Engine environment.
type gaeEnv struct{}

type appengineVersion struct {
	EnvVariables map[string]string `json:"envVariables"`
}

// EnvVars returns the list of envvars set on this app engine version
func (e *gaeEnv) EnvVars(ctx context.Context) (map[string]string, error) {
	version := os.Getenv("GAE_VERSION")
	service := os.Getenv("GAE_SERVICE")

	project, err := valueFromMetadata(ctx, "project/project-id")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get project ID")
	}

	endpoint := fmt.Sprintf("https://appengine.googleapis.com/v1/apps/%s/services/%s/versions/%s?view=FULL",
		project, service, version)

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
		return nil, errors.Errorf("failed to communicate with cloud run: %s", d)
	}

	var s appengineVersion
	if err := json.Unmarshal(d, &s); err != nil {
		return nil, err
	}

	return s.EnvVariables, nil
}
