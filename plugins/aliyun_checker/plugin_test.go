package aliyunchecker

import (
	"os"
	"testing"

	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"
	pluginmanager "opencsg.com/csghub-server/builder/plugins_manager"
)

const testPluginEnv = "CSGHUB_TEST_PLUGIN"

func TestHelperPluginProcess(t *testing.T) {
	if os.Getenv(testPluginEnv) != "1" {
		return
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			PluginName: NewPlugin(&fakeChecker{}),
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

func TestPluginManagerConnectsToAliyunChecker(t *testing.T) {
	manager := pluginmanager.NewManager()
	defer func() {
		require.NoError(t, manager.Close())
	}()

	api, err := manager.Get(pluginmanager.Definition{
		Name: PluginName,
		Command: []string{
			os.Args[0], "-test.run", "^TestHelperPluginProcess$",
		},
		Env:             []string{testPluginEnv + "=1"},
		HandshakeConfig: HandshakeConfig,
		Plugin:          NewPlugin(nil),
	})
	require.NoError(t, err)

	client, ok := api.(*Client)
	require.True(t, ok)
	result, err := client.PassTextCheck(t.Context(), "comment_detection", "hello")
	require.NoError(t, err)
	require.False(t, result.IsSensitive)
}
