package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAgentInstanceRelayRegistration(t *testing.T) {
	const agentID = "agt_0123456789abcdef01234567"

	tests := []struct {
		name    string
		relay   *AgentInstanceRelayRegistration
		wantErr string
	}{
		{name: "nil registration"},
		{name: "valid", relay: &AgentInstanceRelayRegistration{AgentID: agentID}},
		{name: "invalid prefix", relay: &AgentInstanceRelayRegistration{AgentID: "agent_0123456789abcdef01234567"}, wantErr: "relay agent_id is invalid"},
		{name: "short id", relay: &AgentInstanceRelayRegistration{AgentID: "agt_0123"}, wantErr: "relay agent_id is invalid"},
		{name: "uppercase hex", relay: &AgentInstanceRelayRegistration{AgentID: "agt_0123456789ABCDEF01234567"}, wantErr: "relay agent_id is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentInstanceRelayRegistration(tt.relay)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

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

func TestAgentReasoningEffortValid(t *testing.T) {
	for _, level := range []AgentReasoningEffort{
		AgentReasoningEffortAuto, AgentReasoningEffortNone, AgentReasoningEffortMinimal,
		AgentReasoningEffortLow, AgentReasoningEffortMedium, AgentReasoningEffortHigh, AgentReasoningEffortXHigh,
	} {
		require.True(t, level.Valid(), string(level))
	}
	for _, level := range []AgentReasoningEffort{"", "extreme", "AUTO", "Auto"} {
		require.False(t, level.Valid(), string(level))
	}
}
