package rebac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsistencyValues(t *testing.T) {
	tests := []struct {
		value Consistency
		name  string
	}{
		{value: ConsistencyDefault, name: "default"},
		{value: ConsistencyMinimizeLatency, name: "minimize_latency"},
		{value: ConsistencyHigher, name: "higher"},
	}
	for _, test := range tests {
		require.True(t, test.value.Valid())
		require.Equal(t, test.name, test.value.String())
	}
	invalid := Consistency(255)
	require.False(t, invalid.Valid())
	require.Equal(t, "unknown", invalid.String())
}
