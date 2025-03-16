package middleware

import (
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type Middleware struct {
	config            *config.Config
	userComponent     component.UserComponent
	mirrorComponent   component.MirrorComponent
	userServiceClient rpc.UserSvcClient
}

func NewMiddleware(config *config.Config) *Middleware {
	userComponent, err := component.NewUserComponent(config)
	if err != nil {
		panic(err)
	}
	mirrorComponent, err := component.NewMirrorComponent(config)
	if err != nil {
		panic(err)
	}
	userServiceClient := rpc.NewUserSvcHttpClient(config)
	return NewMiddlewareDI(config, userComponent, mirrorComponent, userServiceClient)
}

func NewMiddlewareDI(config *config.Config, userComponent component.UserComponent, mirrorComponent component.MirrorComponent, userServiceClient rpc.UserSvcClient) *Middleware {
	return &Middleware{
		config:            config,
		userComponent:     userComponent,
		mirrorComponent:   mirrorComponent,
		userServiceClient: userServiceClient,
	}
}
