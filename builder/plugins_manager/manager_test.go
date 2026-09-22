package pluginmanager

import (
	"context"
	"net/rpc"
	"testing"

	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeGRPCPlugin struct{}

func (fakeGRPCPlugin) Server(*plugin.MuxBroker) (any, error) { return nil, nil }

func (fakeGRPCPlugin) Client(*plugin.MuxBroker, *rpc.Client) (any, error) { return nil, nil }

func (fakeGRPCPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error { return nil }

func (fakeGRPCPlugin) GRPCClient(context.Context, *plugin.GRPCBroker, *grpc.ClientConn) (any, error) {
	return nil, nil
}

func TestManagerGetValidatesDefinition(t *testing.T) {
	manager := NewManager()
	tests := []struct {
		name       string
		definition Definition
		errorText  string
	}{
		{name: "missing name", definition: Definition{}, errorText: "plugin name is required"},
		{name: "missing plugin", definition: Definition{Name: "test"}, errorText: "plugin implementation is required for test"},
		{name: "missing command", definition: Definition{Name: "test", Plugin: fakeGRPCPlugin{}}, errorText: "plugin command is required for test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := manager.Get(test.definition)
			require.EqualError(t, err, test.errorText)
		})
	}
	require.NoError(t, manager.Close())
}
