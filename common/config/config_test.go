package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_loadConfig(t *testing.T) {
	t.Run("config env", func(t *testing.T) {
		SetConfigFile("")
		t.Setenv("STARHUB_SERVER_INSTANCE_ID", "foo")
		t.Setenv("STARHUB_SERVER_SERVER_PORT", "6789")
		t.Setenv("STARHUB_SERVER_POSTHOG_ENABLED", "true")
		t.Setenv("STARHUB_SERVER_POSTHOG_PROJECT_TOKEN", "phc_test")
		t.Setenv("STARHUB_SERVER_POSTHOG_API_HOST", "https://example.posthog.test")
		t.Setenv("STARHUB_SERVER_POSTHOG_ENVIRONMENT", "staging")
		cfg, err := loadConfig()
		require.Nil(t, err)

		require.Equal(t, "foo", cfg.InstanceID)
		require.Equal(t, 6789, cfg.APIServer.Port)
		require.True(t, cfg.PostHog.Enabled)
		require.Equal(t, "phc_test", cfg.PostHog.ProjectToken)
		require.Equal(t, "https://example.posthog.test", cfg.PostHog.APIHost)
		require.Equal(t, "staging", cfg.PostHog.Environment)
		require.False(t, cfg.Organization.EnableUnit)
		require.Equal(t, 1000, cfg.Rebac.OpenFGAListObjectMaxResult)
		require.Equal(t, 60, cfg.Search.RepositoryAccessListCacheTTL)
	})

	t.Run("repository access cache TTL env", func(t *testing.T) {
		SetConfigFile("")
		t.Setenv("STARHUB_SERVER_REPOSITORY_ACCESS_LIST_CACHE_TTL", "90")
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Equal(t, 90, cfg.Search.RepositoryAccessListCacheTTL)
	})

	t.Run("config file", func(t *testing.T) {
		SetConfigFile("test.toml")
		cfg, err := loadConfig()
		require.Nil(t, err)

		require.Equal(t, "bar", cfg.InstanceID)
		require.Equal(t, 4321, cfg.APIServer.Port)
		require.Equal(t, "ssh://git@localhost:2222", cfg.APIServer.SSHDomain)
	})

	t.Run("rebac list objects limit env", func(t *testing.T) {
		SetConfigFile("")
		t.Setenv("STARHUB_SERVER_REBAC_OPENFGA_LIST_OBJECT_MAX_RESULT", "1200")
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Equal(t, 1200, cfg.Rebac.OpenFGAListObjectMaxResult)
	})

	t.Run("file and env", func(t *testing.T) {
		SetConfigFile("test.toml")
		t.Setenv("STARHUB_SERVER_INSTANCE_ID", "foobar")
		cfg, err := loadConfig()
		require.Nil(t, err)

		require.Equal(t, "foobar", cfg.InstanceID)
		require.Equal(t, 4321, cfg.APIServer.Port)
		require.Equal(t, "ssh://git@localhost:2222", cfg.APIServer.SSHDomain)
		require.Equal(t, true, cfg.SensitiveCheck.EnableSSL)
	})

	t.Run("federation adapter env", func(t *testing.T) {
		SetConfigFile("")
		t.Setenv("STARHUB_SERVER_FEDERATION_ADAPTER_ENDPOINT", "https://10.10.3.100")
		t.Setenv("STARHUB_SERVER_FEDERATION_ADAPTER_PORT", "9101")
		t.Setenv("OPENCSG_CREDENTIAL_MASTER_KEY_BASE64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

		cfg, err := loadConfig()
		require.Nil(t, err)

		require.Equal(t, "https://10.10.3.100", cfg.FederationAdapter.Host)
		require.Equal(t, 9101, cfg.FederationAdapter.Port)
		require.Equal(t, "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", cfg.Credential.MasterKeyBase64)
	})
}

// TestConfigRebacTOML verifies the documented key and nonpositive values survive loading.
func TestConfigRebacTOML(t *testing.T) {
	previous := configFile
	t.Cleanup(func() { SetConfigFile(previous) })
	for _, value := range []string{"1200", "0", "-1"} {
		t.Run(value, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rebac.toml")
			require.NoError(t, os.WriteFile(path, []byte("[rebac]\nopenfga_list_object_max_result = "+value+"\n"), 0600))
			SetConfigFile(path)
			cfg, err := loadConfig()
			require.NoError(t, err)
			require.Equal(t, value, fmt.Sprint(cfg.Rebac.OpenFGAListObjectMaxResult))
		})
	}
}

// TestConfigRepositoryAccessCacheTTLTOML verifies the documented search cache key is loaded.
func TestConfigRepositoryAccessCacheTTLTOML(t *testing.T) {
	previous := configFile
	t.Cleanup(func() { SetConfigFile(previous) })
	path := filepath.Join(t.TempDir(), "search.toml")
	require.NoError(t, os.WriteFile(path, []byte("[search]\nrepository_access_list_cache_ttl = 75\n"), 0600))
	SetConfigFile(path)
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, 75, cfg.Search.RepositoryAccessListCacheTTL)
}
