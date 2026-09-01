//go:build !ee && !saas

package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewReturnsNoOpInCE(t *testing.T) {
	publisher, err := New(Config{
		Enabled:      true,
		ProjectToken: "ignored",
		APIHost:      "https://example.com",
	})

	require.NoError(t, err)
	require.NoError(t, publisher.Capture(Event{Name: "ignored"}))
	require.NoError(t, publisher.Close())
}
