package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSGClawGuestUsageMaxCount(t *testing.T) {
	tests := []struct {
		name     string
		metadata *map[string]any
		want     *int64
	}{
		{name: "missing metadata"},
		{name: "missing provision request", metadata: &map[string]any{}},
		{name: "unlimited", metadata: guestUsageMetadata(map[string]any{"mode": "unlimited", "max_count": float64(9)})},
		{name: "empty mode", metadata: guestUsageMetadata(map[string]any{"max_count": float64(9)})},
		{name: "malformed", metadata: guestUsageMetadata("invalid")},
		{name: "limited zero", metadata: guestUsageMetadata(map[string]any{"mode": "limited", "max_count": float64(0)}), want: int64Pointer(0)},
		{name: "limited positive", metadata: guestUsageMetadata(map[string]any{"mode": "limited", "max_count": float64(3)}), want: int64Pointer(3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, CSGClawGuestUsageMaxCount(tt.metadata))
		})
	}
}

func guestUsageMetadata(limit any) *map[string]any {
	metadata := map[string]any{
		"provision_request": map[string]any{"guest_usage_limit": limit},
	}
	return &metadata
}

func int64Pointer(value int64) *int64 {
	return &value
}
