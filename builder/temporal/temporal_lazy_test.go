package temporal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

func TestNewLazyClientDoesNotRequireReachableServer(t *testing.T) {
	original := _client
	_client = &clientImpl{}
	t.Cleanup(func() {
		if _client.Client != nil {
			_client.Close()
		}
		_client = original
	})

	created, err := NewLazyClient(client.Options{HostPort: "127.0.0.1:1"}, "test")

	require.NoError(t, err)
	require.NotNil(t, created.Client)
}
