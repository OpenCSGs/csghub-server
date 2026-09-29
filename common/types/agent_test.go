package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSGClawHasCustomUI(t *testing.T) {
	tests := []struct {
		name     string
		metadata *map[string]any
		want     bool
		wantErr  string
	}{
		{name: "missing metadata"},
		{name: "enabled", metadata: &map[string]any{"provision_request": map[string]any{"custom_ui_space_id": float64(7)}}, want: true},
		{name: "invalid type", metadata: &map[string]any{"provision_request": map[string]any{"custom_ui_space_id": "7"}}, wantErr: "invalid provision_request.custom_ui_space_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CSGClawHasCustomUI(tt.metadata)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCSGClawAgentName(t *testing.T) {
	metadata := &map[string]any{
		"template_metadata": map[string]any{
			"agent_file": map[string]any{"name": "generic-assistant-lj1"},
		},
	}
	name, err := CSGClawAgentName(metadata)
	require.NoError(t, err)
	require.Equal(t, "generic-assistant-lj1", name)

	for _, invalidName := range []string{"../invalid", ".", "..", "bad\nname", "bad\rname", "bad\x00name", "name space", ""} {
		_, err = CSGClawAgentName(&map[string]any{"template_metadata": map[string]any{"agent_file": map[string]any{"name": invalidName}}})
		require.ErrorIs(t, err, ErrCSGClawAgentNameInvalid, invalidName)
	}
}
