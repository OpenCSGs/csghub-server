package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMirrorTrustedDomainsConfig(t *testing.T) {
	previous := configFile
	SetConfigFile("")
	t.Cleanup(func() { SetConfigFile(previous) })
	const key = "STARHUB_SERVER_MIRROR_TRUSTED_DOMAINS"
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"api.github.com", "github.com", "huggingface.co"}, cfg.Mirror.TrustedDomains)
	t.Setenv(key, "mirror.example,hf.co")
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"mirror.example", "hf.co"}, cfg.Mirror.TrustedDomains)
}
