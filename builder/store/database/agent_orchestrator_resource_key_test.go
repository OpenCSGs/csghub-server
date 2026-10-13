package database

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatAgentRuntimeResourceKey(t *testing.T) {
	const suffix = "a1b2c3d4e5f6"

	key, err := formatAgentRuntimeResourceKey("Jun_Generic.Assistant~Codex", suffix)
	require.NoError(t, err)
	require.Equal(t, "aor-jun-generic-assistant-codex-"+suffix, key)

	key, err = formatAgentRuntimeResourceKey(strings.Repeat("a", 128), suffix)
	require.NoError(t, err)
	require.Len(t, key, agentRuntimeResourceNameMaxLength)
	require.Equal(t, "aor-", key[:len("aor-")])
	require.Equal(t, "-"+suffix, key[len(key)-len(suffix)-1:])

	key, err = formatAgentRuntimeResourceKey("___", suffix)
	require.NoError(t, err)
	require.Equal(t, "aor-agent-"+suffix, key)
}

func TestFormatAgentRuntimeResourceKeyRejectsInvalidSuffix(t *testing.T) {
	for _, suffix := range []string{"short", "A1B2C3D4E5F6", "a1b2c3d4e5f-"} {
		_, err := formatAgentRuntimeResourceKey("logical-agent", suffix)
		require.Error(t, err)
	}
}

func TestAgentAllocationOperationKeyUsesResourceKey(t *testing.T) {
	require.Equal(t,
		"agent-allocation/aor-logical-agent-a1b2c3d4e5f6/3",
		agentAllocationOperationKey("aor-logical-agent-a1b2c3d4e5f6", 3),
	)
}
