package component

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestParseRuntimeProfileFile(t *testing.T) {
	t.Parallel()

	t.Run("loads csgclaw profile", func(t *testing.T) {
		path, err := agentRuntimeProfilePath("csgclaw.json")
		require.NoError(t, err)
		profile, name, err := parseRuntimeProfileFile(path)
		require.NoError(t, err)
		require.Equal(t, csgclawRuntimeProfileName, name)
		require.Equal(t, "csgclaw", profile.AgentType)
		require.NotEmpty(t, profile.Image)
		require.NotEmpty(t, profile.ContentSHA)
	})

	t.Run("loads csgclaw-workspace profile", func(t *testing.T) {
		path, err := agentRuntimeProfilePath("csgclaw-workspace.json")
		require.NoError(t, err)
		profile, name, err := parseRuntimeProfileFile(path)
		require.NoError(t, err)
		require.Equal(t, csgclawWorkspaceRuntimeProfileName, name)
		require.Equal(t, "csgclaw-workspace", profile.AgentType)
		require.NotContains(t, profile.DefaultEnv, "CSGCLAW_SANDBOX_IMAGE")
		require.NotEqual(t, "", profile.ContentSHA)
	})

	t.Run("rejects agent_type that does not match filename", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "csgclaw-workspace.json")
		require.NoError(t, os.WriteFile(path, []byte(`{
  "agent_type": "csgclaw",
  "version": "v1",
  "image": "registry/csgclaw:v1",
  "port": 18080
}`), 0o644))
		_, _, err := parseRuntimeProfileFile(path)
		require.ErrorContains(t, err, "invalid csgclaw-workspace runtime profile")
	})
}

func TestValidateSandboxRuntimeProfile(t *testing.T) {
	t.Parallel()

	profile := &CSGClawRuntimeProfile{
		AgentType: "csgclaw-workspace",
		Version:   "v0.6.0",
		Image:     "registry/csgclaw:v0.6.0",
		Port:      18080,
	}
	require.NoError(t, validateSandboxRuntimeProfile(profile, "csgclaw-workspace"))
	require.ErrorContains(t, validateSandboxRuntimeProfile(profile, "csgclaw"), "invalid csgclaw runtime profile")
}

func TestSandboxRuntimeProfileName(t *testing.T) {
	t.Parallel()
	require.Equal(t, "sandbox_runtime.csgclaw-workspace", sandboxRuntimeProfileName("csgclaw-workspace"))
}

func TestSyncAgentRuntimeProfilesLoadsAllJSONFiles(t *testing.T) {
	ctx := context.Background()
	store := mockdatabase.NewMockAgentConfigStore(t)
	created := map[string]map[string]any{}
	store.EXPECT().GetByName(ctx, mock.Anything).Return(nil, nil)
	store.EXPECT().Create(ctx, mock.Anything).Run(func(_ context.Context, config *database.AgentConfig) {
		created[config.Name] = config.Config
	}).Return(nil)

	require.NoError(t, syncAgentRuntimeProfiles(ctx, store))
	require.Equal(t, "csgclaw", created[csgclawRuntimeProfileName]["agent_type"])
	require.Equal(t, "csgclaw-workspace", created[csgclawWorkspaceRuntimeProfileName]["agent_type"])
	require.NotContains(t, created[csgclawWorkspaceRuntimeProfileName]["default_env"], "CSGCLAW_SANDBOX_IMAGE")
}

func validCSGClawProfile() CSGClawRuntimeProfile {
	return CSGClawRuntimeProfile{
		AgentType: "csgclaw",
		Version:   "v0.7.0",
		Image:     "registry.example.com/csgclaw:v1",
		Port:      18080,
		Command:   []string{"/sbin/tini", "--"},
	}
}

func TestValidateSandboxRuntimeProfile_CustomUI(t *testing.T) {
	validSHA := strings.Repeat("9f", 32)

	t.Run("nil custom_ui is allowed", func(t *testing.T) {
		profile := validCSGClawProfile()
		require.NoError(t, validateSandboxRuntimeProfile(&profile, "csgclaw"))
	})

	t.Run("valid custom_ui", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "https://bucket.example.com/agent-ui/v0.8.0/bundle.tar.gz",
			SHA256: validSHA,
			SDK:    types.GRADIO.Name,
		}
		require.NoError(t, validateSandboxRuntimeProfile(&profile, "csgclaw"))
	})

	t.Run("streamlit sdk is allowed", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "https://bucket.example.com/bundle.tar.gz",
			SHA256: validSHA,
			SDK:    types.STREAMLIT.Name,
		}
		require.NoError(t, validateSandboxRuntimeProfile(&profile, "csgclaw"))
	})

	t.Run("non-https url rejected", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "http://bucket.example.com/bundle.tar.gz",
			SHA256: validSHA,
			SDK:    types.GRADIO.Name,
		}
		require.ErrorContains(t, validateSandboxRuntimeProfile(&profile, "csgclaw"), "https")
	})

	t.Run("empty url rejected", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			SHA256: validSHA,
			SDK:    types.GRADIO.Name,
		}
		require.Error(t, validateSandboxRuntimeProfile(&profile, "csgclaw"))
	})

	t.Run("short sha256 rejected", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "https://bucket.example.com/bundle.tar.gz",
			SHA256: "9f2c",
			SDK:    types.GRADIO.Name,
		}
		require.ErrorContains(t, validateSandboxRuntimeProfile(&profile, "csgclaw"), "sha256")
	})

	t.Run("non-hex sha256 rejected", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "https://bucket.example.com/bundle.tar.gz",
			SHA256: strings.Repeat("zz", 32),
			SDK:    types.GRADIO.Name,
		}
		require.ErrorContains(t, validateSandboxRuntimeProfile(&profile, "csgclaw"), "sha256")
	})

	t.Run("unsupported sdk rejected", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "https://bucket.example.com/bundle.tar.gz",
			SHA256: validSHA,
			SDK:    types.DOCKER.Name,
		}
		require.ErrorContains(t, validateSandboxRuntimeProfile(&profile, "csgclaw"), "sdk")
	})

	t.Run("empty sdk rejected", func(t *testing.T) {
		profile := validCSGClawProfile()
		profile.CustomUI = &CSGClawCustomUI{
			URL:    "https://bucket.example.com/bundle.tar.gz",
			SHA256: validSHA,
		}
		require.ErrorContains(t, validateSandboxRuntimeProfile(&profile, "csgclaw"), "sdk")
	})
}
