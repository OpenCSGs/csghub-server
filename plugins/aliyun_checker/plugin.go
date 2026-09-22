package aliyunchecker

import (
	"context"
	"errors"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	v1 "opencsg.com/csghub-server/plugins/aliyun_checker/v1"
)

const PluginName = "csghub_aliyun_checker"

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "CSGHUB_ALIYUN_CHECKER_PLUGIN",
	MagicCookieValue: "b0e63ec4-bcf4-4a7d-bec7-2a838b9a6a28",
}

type Plugin struct {
	checker Checker
}

func NewPlugin(checker Checker) *Plugin {
	return &Plugin{checker: checker}
}

func (p *Plugin) Server(*plugin.MuxBroker) (any, error) {
	return nil, errors.New("net/rpc is not supported")
}

func (p *Plugin) Client(*plugin.MuxBroker, *rpc.Client) (any, error) {
	return nil, errors.New("net/rpc is not supported")
}

func (p *Plugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	v1.RegisterAliyunCheckerServer(server, NewServer(p.checker))
	return nil
}

func (p *Plugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, conn *grpc.ClientConn) (any, error) {
	return NewClient(v1.NewAliyunCheckerClient(conn)), nil
}
