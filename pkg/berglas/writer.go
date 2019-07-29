package berglas

import (
	"context"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
)

func (c *Client) write(
	ctx context.Context, bucket, object, key, blob string, conds *storage.Conditions, plaintext []byte,
	preconditionFailureError string) (*Secret, error) {
	// Write the object with CAS
	iow := c.storageClient.
		Bucket(bucket).
		Object(object).
		If(*conds).
		NewWriter(ctx)
	iow.ObjectAttrs.CacheControl = CacheControl
	iow.ChunkSize = 1024

	if iow.Metadata == nil {
		iow.Metadata = make(map[string]string)
	}

	// Mark this as a secret
	iow.Metadata[MetadataIDKey] = "1"

	// If a specific key version was given, only store the key, not the key
	// version, because decrypt calls can't specify a key version.
	iow.Metadata[MetadataKMSKey] = kmsKeyTrimVersion(key)

	if _, err := iow.Write([]byte(blob)); err != nil {
		return nil, errors.Wrap(err, "failed save encrypted ciphertext to storage")
	}

	// Close, handling any errors
	if err := iow.Close(); err != nil {
		if terr, ok := err.(*googleapi.Error); ok {
			switch terr.Code {
			case http.StatusNotFound:
				return nil, errors.New("bucket does not exist")
			case http.StatusPreconditionFailed:
				return nil, errors.New(preconditionFailureError)
			}
		}

		return nil, errors.Wrap(err, "failed to close writer")
	}

	return &Secret{
		Name:           iow.Attrs().Name,
		Generation:     iow.Attrs().Generation,
		KMSKey:         iow.Attrs().Metadata[MetadataKMSKey],
		Metageneration: iow.Attrs().Metageneration,
		Plaintext:      plaintext,
		UpdatedAt:      iow.Attrs().Updated,
	}, nil
}
