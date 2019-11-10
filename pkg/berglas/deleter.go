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
	"runtime"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/berglas/pkg/logger"
	"github.com/gammazero/workerpool"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

// Delete is a top-level package function for creating a secret. For large
// volumes of secrets, please create a client instead.
func Delete(ctx context.Context, i *DeleteRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Delete(ctx, i)
}

// DeleteRequest is used as input to a get secret request.
type DeleteRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the secret in Cloud Storage.
	Object string

	// Logger is internal logger used for debugging purposes
	Logger logger.Logger
}

// Delete reads the contents of the secret from the bucket, decrypting the
// ciphertext using Cloud KMS.
func (c *Client) Delete(ctx context.Context, i *DeleteRequest) error {
	if i == nil {
		return errors.New("missing request")
	}

	bucket := i.Bucket
	if bucket == "" {
		return errors.New("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return errors.New("missing object name")
	}

	i.Logger.Logf("initialising storageClient iterator for bucket %s and object prefix %s...", bucket, object)
	it := c.storageClient.
		Bucket(bucket).
		Objects(ctx, &storage.Query{
			Prefix:   object,
			Versions: true,
		})

	// Create a workerpool for parallel deletion of resources
	i.Logger.Logf("creating workerpool for parallel deletion of resources...")
	wp := workerpool.New(runtime.NumCPU() - 1)
	errCh := make(chan error)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

L:
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			select {
			case <-childCtx.Done():
			case errCh <- errors.Wrap(err, "failed to list secrets"):
				cancel()
			default:
			}
		}

		// Don't queue more tasks if a failure has been encountered already
		select {
		case <-childCtx.Done():
			break L
		default:
			wp.Submit(func() {
				err := c.storageClient.
					Bucket(bucket).
					Object(object).
					Generation(obj.Generation).
					Delete(childCtx)

				if err != nil {
					select {
					case <-childCtx.Done():
					case errCh <- err:
						cancel()
					default:
						cancel()
					}
				}
			})
		}
	}

	wp.StopWait()

	select {
	case err := <-errCh:
		return errors.Wrap(err, "failed to delete secret")
	default:
		return nil
	}
}
