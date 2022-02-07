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

	"cloud.google.com/go/storage"
	"github.com/gammazero/workerpool"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	secretspb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
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

	logger := c.Logger().WithFields(logrus.Fields{
		"project": project,
		"name":    name,
	})

	logger.Debug("delete.start")
	defer logger.Debug("delete.finish")

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

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket": bucket,
		"object": object,
	})

	logger.Debug("delete.start")
	defer logger.Debug("delete.finish")

	it := c.storageClient.
		Bucket(bucket).
		Objects(ctx, &storage.Query{
			Prefix:   object,
			Versions: true,
		})

	// Create a workerpool for parallel deletion of resources
	ws := runtime.NumCPU() - 1
	wp := workerpool.New(ws)
	errCh := make(chan error)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger.WithField("parallelism", ws).Debug("deleting secrets")

L:
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			logger.Debug("out of objects")
			break
		}
		if err != nil {
			logger.WithError(err).Error("failed to get object")

			select {
			case <-childCtx.Done():
				logger.Debug("exiting because context finished")
			case errCh <- fmt.Errorf("failed to list secrets: %w", err):
				logger.Debug("pushed error onto channel, canceling other jobs")
				cancel()
			default:
				logger.WithError(err).Error("received error, but channel blocked")
			}
		}

		// Don't queue more tasks if a failure has been encountered already
		select {
		case <-childCtx.Done():
			logger.Debug("child context is finished, exiting")
			break L
		default:
			logger.WithField("generation", obj.Generation).
				Debug("queuing delete worker")

			wp.Submit(func() {
				err := c.storageClient.
					Bucket(bucket).
					Object(object).
					Generation(obj.Generation).
					Delete(childCtx)

				if err != nil {
					logger.
						WithError(err).
						WithField("generation", obj.Generation).
						Debug("worker failed to delete object")

					select {
					case <-childCtx.Done():
					case errCh <- fmt.Errorf("failed to delete generation: %w", err):
						logger.Debug("worker pushed error onto channel, canceling other jobs")
						cancel()
					default:
						logger.WithError(err).Error("worker received error but channel blocked")
						cancel()
					}
				}
			})
		}
	}

	// Wait for jobs to finish
	logger.Debug("waiting for delete jobs to finish")
	wp.StopWait()

	select {
	case err := <-errCh:
		return fmt.Errorf("failed to delete secret: %w", err)
	default:
		return nil
	}
}
