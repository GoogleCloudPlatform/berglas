package berglas

import (
	"context"
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/berglas/v2/pkg/berglas/logging"
)

// Replace parses a berglas reference and replaces it. See Client.Replace for
// more details and examples.
func Replace(ctx context.Context, key string) error {
	client, err := New(ctx)
	if err != nil {
		return err
	}
	return client.Replace(ctx, key)
}

// Replace parses a berglas reference from the environment variable at the
// given environment variable name. If parsing and extraction is successful,
// this function replaces the value of the environment variable to the resolved
// secret reference.
func (c *Client) Replace(ctx context.Context, key string) error {
	value := os.Getenv(key)

	logger := logging.FromContext(ctx).With(
		"key", key,
		"reference", value,
	)

	logger.DebugContext(ctx, "replacevalue.start")
	defer logger.DebugContext(ctx, "replacevalue.finish")

	plaintext, err := c.Resolve(ctx, value)
	if err != nil {
		return err
	}

	if err := os.Setenv(key, string(plaintext)); err != nil {
		return fmt.Errorf("failed to set %s: %w", key, err)
	}
	return nil
}
