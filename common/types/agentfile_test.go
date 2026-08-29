package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testAgentFileContent = `
name = 'gitlab-assistant'
role = 'worker'
description = 'GitLab assistant'
runtime_kind = 'openclaw'
updated_at = '2026-06-29T02:13:29Z'
version = '2026.6.29.0'
schema_version = 'agentfile/v1'
tags = ['self-hosted']

[image]
ref = 'registry.example.com/opencsghq/openclaw-glab:2026.7.3.0'
[[image.env]]
name = 'GITLAB_TOKEN'
required = true
secret = true
description = 'GitLab personal access token'
[[image.env]]
name = 'GITLAB_BASE_URL'
required = true
default = 'https://git-devops.opencsg.com'
`

func TestParseAgentFile(t *testing.T) {
	agentFile, err := ParseAgentFile(testAgentFileContent)
	require.NoError(t, err)
	require.Equal(t, "gitlab-assistant", agentFile.Name)
	require.Equal(t, "worker", agentFile.Role)
	require.Equal(t, "openclaw", agentFile.RuntimeKind)
	require.Equal(t, "agentfile/v1", agentFile.SchemaVersion)
	require.Equal(t, []string{"self-hosted"}, agentFile.Tags)
	require.Equal(t, "registry.example.com/opencsghq/openclaw-glab:2026.7.3.0", agentFile.Image.Ref)
	require.Len(t, agentFile.Image.Env, 2)
	require.Equal(t, "GITLAB_TOKEN", agentFile.Image.Env[0].Name)
	require.True(t, agentFile.Image.Env[0].Required)
	require.True(t, agentFile.Image.Env[0].Secret)
	require.Equal(t, "https://git-devops.opencsg.com", agentFile.Image.Env[1].Default)
}

func TestParseAgentFile_Invalid(t *testing.T) {
	_, err := ParseAgentFile("not = [valid")
	require.Error(t, err)
}

func TestParseAgentFile_MissingName(t *testing.T) {
	_, err := ParseAgentFile("runtime_kind = 'codex'")
	require.ErrorContains(t, err, "name")
}

func TestSupportedSandboxRuntimeKinds(t *testing.T) {
	require.True(t, SupportedSandboxRuntimeKinds["codex"])
	require.False(t, SupportedSandboxRuntimeKinds["openclaw"])
}
