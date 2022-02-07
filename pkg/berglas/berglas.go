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

// Package berglas is the Go API for calling berglas.
package berglas

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	storagev1 "google.golang.org/api/storage/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Name, Version, ProjectURL, and UserAgent are used to uniquely identify this
	// package in logs and other binaries.
	Name       = "berglas"
	Version    = "0.6.2"
	ProjectURL = "https://github.com/GoogleCloudPlatform/berglas"
	UserAgent  = Name + "/" + Version + " (+" + ProjectURL + ")"
)

const (
	// CacheControl is the cache-control value to set on the GCS objects. This is
	// configured to use no caching, since users most likely want their secrets to
	// be immediately available.
	CacheControl = "private, no-cache, no-store, no-transform, max-age=0"

	// ChunkSize is the size in bytes of the chunks to upload.
	ChunkSize = 1024

	// MetadataIDKey is a key in the object metadata that identifies an object as
	// a secret. This is used when enumerating secrets in a bucket, in case
	// non-secrets also reside in the bucket.
	MetadataIDKey = "berglas-secret"

	// MetadataKMSKey is the key in the metadata where the name of the KMS key is
	// stored.
	MetadataKMSKey = "berglas-kms-key"
)

// Client is a berglas client
type Client struct {
	kmsClient           *kms.KeyManagementClient
	secretManagerClient *secretmanager.Client
	storageClient       *storage.Client
	storageIAMClient    *storagev1.Service

	loggerLock sync.RWMutex
	logger     *logrus.Logger
}

// New creates a new berglas client.
func New(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	opts = append(opts, option.WithUserAgent(UserAgent))

	var c Client

	kmsClient, err := kms.NewKeyManagementClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kms client: %w", err)
	}
	c.kmsClient = kmsClient

	secretManagerClient, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create secretManager client: %w", err)
	}
	c.secretManagerClient = secretManagerClient

	storageClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	c.storageClient = storageClient

	storageIAMClient, err := storagev1.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create storagev1 client: %w", err)
	}
	c.storageIAMClient = storageIAMClient

	c.logger = &logrus.Logger{
		Out:          os.Stderr,
		Formatter:    new(logrus.JSONFormatter),
		Hooks:        make(logrus.LevelHooks),
		Level:        logrus.FatalLevel,
		ReportCaller: true,
	}

	return &c, nil
}

// Secret represents a secret.
type Secret struct {
	// Parent is the resource container. For Cloud Storage secrets, this is the
	// bucket name. For Secret Manager secrets, this is the project ID.
	Parent string

	// Name of the secret.
	Name string

	// Plaintext value of the secret. This may be empty.
	Plaintext []byte

	// Version indicates a secret's version. Secret Manager only.
	Version string

	// UpdatedAt indicates when a secret was last updated.
	UpdatedAt time.Time

	// Generation and Metageneration indicates a secret's version. Cloud Storage
	// only.
	Generation, Metageneration int64

	// KMSKey is the key used to encrypt the secret key. Cloud Storage only.
	KMSKey string

	// Locations is the list of custom locations the secret is replicated to.
	// This is set to nil if the secret is automatically replicated instead.
	// Secret Manager only.
	Locations []string
}

// secretFromAttrs constructs a secret from the given object attributes and
// plaintext.
func secretFromAttrs(bucket string, attrs *storage.ObjectAttrs, plaintext []byte) *Secret {
	return &Secret{
		Parent:         bucket,
		Name:           attrs.Name,
		Generation:     attrs.Generation,
		Metageneration: attrs.Metageneration,
		UpdatedAt:      attrs.Updated,
		KMSKey:         attrs.Metadata[MetadataKMSKey],
		Plaintext:      plaintext,
	}
}

func timestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil || !ts.IsValid() {
		return time.Unix(0, 0).UTC()
	}
	return ts.AsTime().UTC()
}

// kmsKeyIncludesVersion returns true if the given KMS key reference includes
// a version.
func kmsKeyIncludesVersion(s string) bool {
	return strings.Count(s, "/") > 7
}

// kmsKeyTrimVersion trims the version from a KMS key reference if it exists.
func kmsKeyTrimVersion(s string) string {
	if !kmsKeyIncludesVersion(s) {
		return s
	}

	parts := strings.SplitN(s, "/", 9)
	return strings.Join(parts[0:8], "/")
}

// envelopeDecrypt decrypts the data with the dek, returning the plaintext and
// any errors that occur.
func envelopeDecrypt(dek, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher from dek: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm from dek: %w", err)
	}

	size := aesgcm.NonceSize()
	if len(data) < size {
		return nil, fmt.Errorf("malformed ciphertext")
	}
	nonce, ciphertext := data[:size], data[size:]

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext with dek: %w", err)
	}
	return plaintext, nil
}

// envelopeEncrypt generates a unique DEK and encrypts the plaintext with the
// given key. The encryption key and resulting ciphertext are returned.
func envelopeEncrypt(plaintext []byte) ([]byte, []byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random key bytes: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher from key: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gcm cipher: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random nonce bytes: %w", err)
	}

	// Encrypt the ciphertext with the DEK
	ciphertext := aesgcm.Seal(nonce, nonce, plaintext, nil)

	return key, ciphertext, nil
}
