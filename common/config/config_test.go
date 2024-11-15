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

		require.Equal(t, "bar", cfg.InstanceID)
		require.Equal(t, 4321, cfg.APIServer.Port)
		require.Equal(t, "git@localhost:2222", cfg.APIServer.SSHDomain)
	})

	t.Run("file and env", func(t *testing.T) {
		SetConfigFile("test.toml")
		t.Setenv("STARHUB_SERVER_INSTANCE_ID", "foobar")
		cfg, err := LoadConfig()
		require.Nil(t, err)

		require.Equal(t, "foobar", cfg.InstanceID)
		require.Equal(t, 4321, cfg.APIServer.Port)
		require.Equal(t, "git@localhost:2222", cfg.APIServer.SSHDomain)
	})
}
