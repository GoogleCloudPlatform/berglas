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
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/iam"
	"github.com/GoogleCloudPlatform/berglas/pkg/retry"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
	storagev1 "google.golang.org/api/storage/v1"
	iampb "google.golang.org/genproto/googleapis/iam/v1"
)

const (
	iamObjectReader = "roles/storage.legacyObjectReader"
	iamKMSDecrypt   = "roles/cloudkms.cryptoKeyDecrypter"
)

// storageIAM returns an IAM storage handle to the given object since one does
// not exist in the storage libray.
func (c *Client) storageIAM(bucket, object string) *iam.Handle {
	return iam.InternalNewHandleClient(&iamClient{
		raw: c.storageIAMClient,
	}, bucket+"/"+object)
}

// iamClient implements the iam.client interface.
type iamClient struct {
	raw *storagev1.Service
}

func (c *iamClient) Get(ctx context.Context, resource string) (*iampb.Policy, error) {
	bucket, object, err := parseBucketObj(resource)
	if err != nil {
		return nil, err
	}

	call := c.raw.Objects.GetIamPolicy(bucket, object)
	setClientHeader(call.Header())

	rp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return iamFromStoragePolicy(rp), nil
}

func (c *iamClient) Set(ctx context.Context, resource string, p *iampb.Policy) error {
	bucket, object, err := parseBucketObj(resource)
	if err != nil {
		return err
	}

	rp := iamToStoragePolicy(p)
	call := c.raw.Objects.SetIamPolicy(bucket, object, rp)
	setClientHeader(call.Header())

	if _, err := call.Context(ctx).Do(); err != nil {
		return err
	}
	return nil
}

func (c *iamClient) Test(ctx context.Context, resource string, perms []string) ([]string, error) {
	bucket, object, err := parseBucketObj(resource)
	if err != nil {
		return nil, err
	}

	call := c.raw.Objects.TestIamPermissions(bucket, object, perms)
	setClientHeader(call.Header())

	res, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return res.Permissions, nil
}

// parseBucketObj parses a bucket, object tuple
func parseBucketObj(s string) (string, string, error) {
	s = strings.TrimPrefix(s, "gs://")

	ss := strings.SplitN(s, "/", 2)
	if len(ss) < 2 {
		return "", "", errors.Errorf("does not match bucket/object format: %s", s)
	}

	return ss[0], ss[1], nil
}

func iamToStoragePolicy(ip *iampb.Policy) *storagev1.Policy {
	return &storagev1.Policy{
		Bindings: iamToStorageBindings(ip.Bindings),
		Etag:     string(ip.Etag),
	}
}

func iamToStorageBindings(ibs []*iampb.Binding) []*storagev1.PolicyBindings {
	var rbs []*storagev1.PolicyBindings
	for _, ib := range ibs {
		rbs = append(rbs, &storagev1.PolicyBindings{
			Role:    ib.Role,
			Members: ib.Members,
		})
	}
	return rbs
}

func iamFromStoragePolicy(rp *storagev1.Policy) *iampb.Policy {
	return &iampb.Policy{
		Bindings: iamFromStorageBindings(rp.Bindings),
		Etag:     []byte(rp.Etag),
	}
}

func iamFromStorageBindings(rbs []*storagev1.PolicyBindings) []*iampb.Binding {
	var ibs []*iampb.Binding
	for _, rb := range rbs {
		ibs = append(ibs, &iampb.Binding{
			Role:    rb.Role,
			Members: rb.Members,
		})
	}
	return ibs
}

func setClientHeader(h http.Header) {
	h.Set("User-Agent", UserAgent)
}

func getIAMPolicyWithRetries(ctx context.Context, h *iam.Handle) (*iam.Policy, error) {
	var policy *iam.Policy
	var err error

	if err := retry.RetryFib(ctx, 500*time.Millisecond, 5, func() error {
		policy, err = h.Policy(ctx)
		if err != nil {
			if terr, ok := errors.Cause(err).(*googleapi.Error); ok {
				switch {
				case terr.Code == 412:
					// IAM returns 412 while propagating
					return retry.RetryableError(terr)
				case terr.Code >= 400 && terr.Code <= 499:
					return terr
				}
			}
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return policy, nil
}

func setIAMPolicyWithRetries(ctx context.Context, h *iam.Handle, newP *iam.Policy) error {
	if err := retry.RetryFib(ctx, 500*time.Millisecond, 5, func() error {
		// Get the current policy - this is required because IAM validates etags
		latestP, err := h.Policy(ctx)
		if err != nil {
			if terr, ok := errors.Cause(err).(*googleapi.Error); ok {
				switch {
				case terr.Code == 412:
					// IAM returns 412 while propagating
					return retry.RetryableError(terr)
				case terr.Code >= 400 && terr.Code <= 499:
					return terr
				}
			}
			return retry.RetryableError(err)
		}

		// Overwrite the existing policy with ours
		for _, r := range newP.Roles() {
			for _, m := range newP.Members(r) {
				latestP.Add(m, r)
			}
		}

		// Update the policy
		if err := h.SetPolicy(ctx, latestP); err != nil {
			if terr, ok := errors.Cause(err).(*googleapi.Error); ok {
				switch {
				case terr.Code == 412:
					// IAM returns 412 while propagating
					return retry.RetryableError(terr)
				case terr.Code >= 400 && terr.Code <= 499:
					return terr
				}
			}
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
