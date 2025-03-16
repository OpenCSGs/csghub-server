//go:build wireinject && !ee && !saas
// +build wireinject,!ee,!saas

package router

import (
	"github.com/google/wire"
	"opencsg.com/csghub-server/common/config"
)

func InitializeServer(config *config.Config) (*ServerImpl, error) {
	wire.Build(
		BaseServerSet,
		NewServer,
	)
	return &ServerImpl{}, nil
}
