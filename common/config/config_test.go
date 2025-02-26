package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_LoadConfig(t *testing.T) {
	t.Run("config env", func(t *testing.T) {
		t.Setenv("STARHUB_SERVER_INSTANCE_ID", "foo")
		t.Setenv("STARHUB_SERVER_SERVER_PORT", "6789")
		cfg, err := LoadConfig()
		require.Nil(t, err)

		require.Equal(t, "foo", cfg.InstanceID)
		require.Equal(t, 6789, cfg.APIServer.Port)
		require.Equal(t, "git@localhost:2222", cfg.APIServer.SSHDomain)
	})

	t.Run("config file", func(t *testing.T) {
		SetConfigFile("test.toml")
		cfg, err := LoadConfig()
		require.Nil(t, err)

		require.Equal(t, 8090, cfg.APIServer.Port)
		require.Equal(t, "ssh://git@localhost:2222", cfg.APIServer.SSHDomain)
	})

	t.Run("file and env", func(t *testing.T) {
		SetConfigFile("test.toml")
		t.Setenv("STARHUB_SERVER_INSTANCE_ID", "foobar")
		cfg, err := LoadConfig()
		require.Nil(t, err)

		require.Equal(t, "foobar", cfg.InstanceID)
		require.Equal(t, 8090, cfg.APIServer.Port)
		require.Equal(t, "ssh://git@localhost:2222", cfg.APIServer.SSHDomain)
	})
}

func TestConfig_LoadConfigOnce(t *testing.T) {
	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.APIServer.Port)

	SetConfigFile("test.toml")
	cfg, err = LoadConfig()
	require.NoError(t, err)
	require.Equal(t, 8090, cfg.APIServer.Port)

}

func TestConfig_Update(t *testing.T) {
	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "", cfg.InstanceID)

	cfg, err = LoadConfig()
	require.NoError(t, err)
	cfg.InstanceID = "abc"

	cfg, err = LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "abc", cfg.InstanceID)

}
