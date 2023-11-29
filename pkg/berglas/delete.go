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
	"runtime"

	secretspb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/berglas/pkg/berglas/logging"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/iterator"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type deleteRequest interface {
	isDeleteRequest()
}

// StorageDeleteRequest is used as input to delete a secret from Cloud Storage.
type StorageDeleteRequest struct {
	// Bucket is the name of the bucket where the secret lives.
	Bucket string

	// Object is the name of the secret in Cloud Storage.
	Object string
}

func (r *StorageDeleteRequest) isDeleteRequest() {}

// DeleteRequest is an alias for StorageDeleteRequest for
// backwards-compatibility. New clients should use StorageDeleteRequest.
type DeleteRequest = StorageDeleteRequest

// SecretManagerDeleteRequest is used as input to delete a secret from Secret
// Manager.
type SecretManagerDeleteRequest struct {
	// Project is the ID or number of the project from which to delete the secret.
	Project string

	// Name is the name of the secret to delete.
	Name string
}

func (r *SecretManagerDeleteRequest) isDeleteRequest() {}

// Delete is a top-level package function for deleting a secret. For large
// volumes of secrets, please create a client instead.
func Delete(ctx context.Context, i deleteRequest) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Delete(ctx, i)
}

// Delete deletes a secret. When given a SecretManagerDeleteRequest, this
// deletes a secret from Secret Manager. When given a StorageDeleteRequest, this
// deletes a secret stored in Cloud Storage.
func (c *Client) Delete(ctx context.Context, i deleteRequest) error {
	if i == nil {
		return fmt.Errorf("missing request")
	}

	switch t := i.(type) {
	case *SecretManagerDeleteRequest:
		return c.secretManagerDelete(ctx, t)
	case *StorageDeleteRequest:
		return c.storageDelete(ctx, t)
	default:
		return fmt.Errorf("unknown delete type %T", t)
	}
}

func (c *Client) secretManagerDelete(ctx context.Context, i *SecretManagerDeleteRequest) error {
	project := i.Project
	if project == "" {
		return fmt.Errorf("missing project")
	}

	name := i.Name
	if name == "" {
		return fmt.Errorf("missing secret name")
	}

	logger := logging.FromContext(ctx).With(
		"project", project,
		"name", name,
	)

	logger.DebugContext(ctx, "delete.start")
	defer logger.DebugContext(ctx, "delete.finish")

	if err := c.secretManagerClient.DeleteSecret(ctx, &secretspb.DeleteSecretRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s", project, name),
	}); err != nil {
		terr, ok := grpcstatus.FromError(err)
		if !ok || terr.Code() != grpccodes.NotFound {
			return fmt.Errorf("failed to delete secret: %w", err)
		}
	}
	return nil
}

func (c *Client) storageDelete(ctx context.Context, i *StorageDeleteRequest) error {
	bucket := i.Bucket
	if bucket == "" {
		return fmt.Errorf("missing bucket name")
	}

	object := i.Object
	if object == "" {
		return fmt.Errorf("missing object name")
	}

	logger := logging.FromContext(ctx).With(
		"bucket", bucket,
		"object", object,
	)

	logger.DebugContext(ctx, "delete.start")
	defer logger.DebugContext(ctx, "delete.finish")

	it := c.storageClient.
		Bucket(bucket).
		Objects(ctx, &storage.Query{
			Prefix:   object,
			Versions: true,
		})

	// Create a workerpool for parallel deletion of resources
	parallelism := int64(runtime.NumCPU() - 1)
	sem := semaphore.NewWeighted(parallelism)

	errCh := make(chan error)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger.DebugContext(ctx, "deleting secrets", "parallelism", parallelism)

L:
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			logger.DebugContext(ctx, "out of objects")
			break
		}
		if err != nil {
			logger.ErrorContext(ctx, "failed to get object", "error", err)

			select {
			case <-childCtx.Done():
				logger.DebugContext(ctx, "exiting because context finished")
			case errCh <- fmt.Errorf("failed to list secrets: %w", err):
				logger.DebugContext(ctx, "pushed error onto channel, canceling other jobs")
				cancel()
			default:
				logger.ErrorContext(ctx, "received error, but channel blocked", "error", err)
			}
		}

		// Don't queue more tasks if a failure has been encountered already
		select {
		case <-childCtx.Done():
			logger.DebugContext(ctx, "child context is finished, exiting")
			break L
		default:
			logger := logger.With("generation", obj.Generation)
			logger.DebugContext(ctx, "queueing delete worker")

			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("failed to acquire semaphore: %w", err)
			}

			go func() {
				defer sem.Release(1)

				if err := c.storageClient.
					Bucket(bucket).
					Object(object).
					Generation(obj.Generation).
					Delete(childCtx); err != nil {
					logger.ErrorContext(ctx, "worker failed to delete object", "error", err)

					select {
					case <-childCtx.Done():
					case errCh <- fmt.Errorf("failed to delete generation: %w", err):
						logger.DebugContext(ctx, "worker pushed error onto channel, canceling other jobs")
						cancel()
					default:
						logger.ErrorContext(ctx, "worker received error but channel blocked", "error", err)
						cancel()
					}
				}
			}()
		}
	}

	// Wait for jobs to finish
	logger.DebugContext(ctx, "waiting for delete jobs to finish")
	if err := sem.Acquire(ctx, parallelism); err != nil {
		return fmt.Errorf("failed to wait for jobs to finish: %w", err)
	}

	select {
	case err := <-errCh:
		return fmt.Errorf("failed to delete secret: %w", err)
	default:
		return nil
	}
}
