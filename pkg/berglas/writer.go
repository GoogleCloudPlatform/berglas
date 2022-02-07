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
	"encoding/base64"
	"fmt"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// encryptAndWrite is a low-level function for encrypting and writing data.
func (c *Client) encryptAndWrite(
	ctx context.Context, bucket, object, key string, plaintext []byte,
	generation, metageneration int64) (*Secret, error) {

	logger := c.Logger().WithFields(logrus.Fields{
		"bucket":         bucket,
		"object":         object,
		"key":            key,
		"generation":     generation,
		"metageneration": metageneration,
	})

	logger.Debug("encryptAndWrite.start")
	defer logger.Debug("encryptAndWrite.finish")

	// Generate a unique DEK and encrypt the plaintext locally (useful for large
	// pieces of data).
	logger.Debug("generating envelope")
	dek, ciphertext, err := envelopeEncrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to perform envelope encryption: %w", err)
	}

	// Encrypt the plaintext using a KMS key
	logger.Debug("encrypting envelope")
	kmsResp, err := c.kmsClient.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:                        key,
		Plaintext:                   dek,
		AdditionalAuthenticatedData: []byte(object),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}
	encDEK := kmsResp.Ciphertext

	// Build the storage object contents. Contents will be of the format:
	//
	//    b64(kms_encrypted_dek):b64(dek_encrypted_plaintext)
	blob := fmt.Sprintf("%s:%s",
		base64.StdEncoding.EncodeToString(encDEK),
		base64.StdEncoding.EncodeToString(ciphertext))

	// If generation and metageneration are 0, then we should only create the
	// object if it does not exist. Otherwise, we should only perform an update if
	// the metagenerations match.
	var conds storage.Conditions
	if generation == 0 || metageneration == 0 {
		conds = storage.Conditions{
			DoesNotExist: true,
		}
	} else {
		conds = storage.Conditions{
			GenerationMatch:     generation,
			MetagenerationMatch: metageneration,
		}
	}

	// Create the writer
	iow := c.storageClient.
		Bucket(bucket).
		Object(object).
		If(conds).
		NewWriter(ctx)

	iow.ObjectAttrs.CacheControl = CacheControl
	iow.ChunkSize = ChunkSize

	if iow.Metadata == nil {
		iow.Metadata = make(map[string]string)
	}
	iow.Metadata[MetadataIDKey] = "1"
	iow.Metadata[MetadataKMSKey] = kmsKeyTrimVersion(key)

	// Write
	logger.WithField("metadata", iow.Metadata).Debug("writing object to storage")
	if _, err := iow.Write([]byte(blob)); err != nil {
		return nil, fmt.Errorf("failed to save encrypted ciphertext to storage: %w", err)
	}

	// Close and flush
	logger.Debug("finalizing writer")
	if err := iow.Close(); err != nil {
		logger.WithError(err).Error("failed to finalize writer")

		if terr, ok := err.(*googleapi.Error); ok {
			switch terr.Code {
			case http.StatusNotFound:
				return nil, fmt.Errorf("bucket does not exist")
			case http.StatusPreconditionFailed:
				if conds.DoesNotExist {
					return nil, errSecretAlreadyExists
				}
				return nil, errSecretModified
			}
		}

		return nil, fmt.Errorf("failed to write to bucket: %w", err)
	}

	return secretFromAttrs(bucket, iow.Attrs(), plaintext), nil
}
