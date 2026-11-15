package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPArchiveEnabledByDefaultAndExplicitOptOut(t *testing.T) {
	previous := configFile
	SetConfigFile("")
	t.Cleanup(func() { SetConfigFile(previous) })
	t.Setenv("OPENCSG_HTTP_ARCHIVE_ENABLE", "")
	require.NoError(t, os.Unsetenv("OPENCSG_HTTP_ARCHIVE_ENABLE"))
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.True(t, cfg.HTTPArchive.Enable)
	t.Setenv("OPENCSG_HTTP_ARCHIVE_ENABLE", "false")
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.False(t, cfg.HTTPArchive.Enable)
}
