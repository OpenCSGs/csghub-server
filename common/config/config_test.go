package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConfigAlipayIdentityInitialization keeps initialization enabled unless explicitly disabled.
func TestConfigAlipayIdentityInitialization(t *testing.T) {
	previousFile := configFile
	SetConfigFile("")
	t.Cleanup(func() { SetConfigFile(previousFile) })
	for _, tc := range []struct {
		name, value string
		want        bool
	}{
		{name: "default", want: true},
		{name: "enabled", value: "true", want: true},
		{name: "disabled", value: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const key = "OPENCSG_USER_SERVER_ENABLE_ALIPAY_IDENTITY"
			t.Setenv(key, tc.value)
			if tc.value == "" {
				require.NoError(t, os.Unsetenv(key))
			}
			cfg, err := loadConfig()
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.User.EnableAlipayIdentity)
		})
	}
}

func TestConfig_loadConfig(t *testing.T) {
	t.Run("frontend URL has no default", func(t *testing.T) {
		SetConfigFile("")
		previous, wasSet := os.LookupEnv("STARHUB_SERVER_FRONTEND_URL")
		require.NoError(t, os.Unsetenv("STARHUB_SERVER_FRONTEND_URL"))
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv("STARHUB_SERVER_FRONTEND_URL", previous)
			}
		})

		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Empty(t, cfg.Frontend.URL)
	})

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
		require.Equal(t, 32, cfg.AIGateway.HealthCheckProbeWorkers)
		require.Equal(t, 4, cfg.AIGateway.HealthCheckPersistenceWorkers)
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

func TestConfigCasdoorInvitationQuota(t *testing.T) {
	previous := configFile
	t.Cleanup(func() { SetConfigFile(previous) })
	const key = "STARHUB_SERVER_CASDOOR_INVITATION_QUOTA"
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
	SetConfigFile("")
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, 50, cfg.Casdoor.InvitationQuota)

	path := filepath.Join(t.TempDir(), "casdoor.toml")
	require.NoError(t, os.WriteFile(path, []byte("[casdoor]\ninvitation_quota = 75\n"), 0600))
	SetConfigFile(path)
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, 75, cfg.Casdoor.InvitationQuota)

	t.Setenv(key, "100")
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, 100, cfg.Casdoor.InvitationQuota)
}

// TestConfigCORSAllowedOrigins verifies the CORS whitelist default, env and toml parsing.
func TestConfigCORSAllowedOrigins(t *testing.T) {
	t.Run("env list", func(t *testing.T) {
		SetConfigFile("")
		t.Setenv("STARHUB_SERVER_CORS_ALLOWED_ORIGINS", "https://hub.opencsg.com,https://*.opencsg.com")
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Equal(t, []string{"https://hub.opencsg.com", "https://*.opencsg.com"}, cfg.CORS.AllowedOrigins)
	})

	t.Run("toml list", func(t *testing.T) {
		previous := configFile
		t.Cleanup(func() { SetConfigFile(previous) })
		path := filepath.Join(t.TempDir(), "cors.toml")
		require.NoError(t, os.WriteFile(path, []byte("[cors]\nallowed_origins = [\"https://example.com\"]\n"), 0600))
		SetConfigFile(path)
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Equal(t, []string{"https://example.com"}, cfg.CORS.AllowedOrigins)
	})

	t.Run("default list is empty and falls back to derivation", func(t *testing.T) {
		SetConfigFile("")
		previous, wasSet := os.LookupEnv("STARHUB_SERVER_CORS_ALLOWED_ORIGINS")
		require.NoError(t, os.Unsetenv("STARHUB_SERVER_CORS_ALLOWED_ORIGINS"))
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv("STARHUB_SERVER_CORS_ALLOWED_ORIGINS", previous)
			}
		})
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Empty(t, cfg.CORS.AllowedOrigins)
	})
}

func TestRouterGenerationDefaultsAndOverrides(t *testing.T) {
	SetConfigFile("")
	for _, key := range []string{"OPENCSG_AIGATEWAY_ROUTER_GENERATION_ENABLED", "OPENCSG_AIGATEWAY_ROUTER_CACHE_ENVIRONMENT", "OPENCSG_AIGATEWAY_ROUTER_CACHE_NAMESPACE"} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.True(t, cfg.AIGateway.SemanticRouter.GenerationEnabled)
	require.Equal(t, "default", cfg.AIGateway.SemanticRouter.CacheEnvironment)
	require.Equal(t, "csghub", cfg.AIGateway.SemanticRouter.CacheNamespace)
	t.Setenv("OPENCSG_AIGATEWAY_ROUTER_GENERATION_ENABLED", "false")
	t.Setenv("OPENCSG_AIGATEWAY_ROUTER_CACHE_ENVIRONMENT", "stg")
	t.Setenv("OPENCSG_AIGATEWAY_ROUTER_CACHE_NAMESPACE", "isolated")
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.False(t, cfg.AIGateway.SemanticRouter.GenerationEnabled)
	require.Equal(t, "stg", cfg.AIGateway.SemanticRouter.CacheEnvironment)
	require.Equal(t, "isolated", cfg.AIGateway.SemanticRouter.CacheNamespace)
}
