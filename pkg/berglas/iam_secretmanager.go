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
	"fmt"

	"cloud.google.com/go/iam"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
)

const (
	iamSecretManagerAccessor = "roles/secretmanager.secretAccessor"
)

// secretManagerIAM returns an IAM storage handle to the given secret since one
// does not exist in the secrets library.
func (c *Client) secretManagerIAM(project, name string) *iam.Handle {
	return iam.InternalNewHandleClient(&secretManagerIAMClient{
		raw: c.secretManagerClient,
	}, fmt.Sprintf("projects/%s/secrets/%s", project, name))
}

// secretManagerIAMClient implements the iam.client interface.
type secretManagerIAMClient struct {
	raw *secretmanager.Client
}

func (c *secretManagerIAMClient) Get(ctx context.Context, resource string) (*iampb.Policy, error) {
	return c.GetWithVersion(ctx, resource, 1)
}

func (c *secretManagerIAMClient) GetWithVersion(ctx context.Context, resource string, version int32) (*iampb.Policy, error) {
	return c.raw.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: resource,
		Options: &iampb.GetPolicyOptions{
			RequestedPolicyVersion: version,
		},
	})
}

func (c *secretManagerIAMClient) Set(ctx context.Context, resource string, p *iampb.Policy) error {
	_, err := c.raw.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
		Resource: resource,
		Policy:   p,
	})
	return err
}

func (c *secretManagerIAMClient) Test(ctx context.Context, resource string, perms []string) ([]string, error) {
	list, err := c.raw.TestIamPermissions(ctx, &iampb.TestIamPermissionsRequest{
		Resource:    resource,
		Permissions: perms,
	})
	if err != nil {
		return nil, err
	}
	return list.Permissions, nil
}
