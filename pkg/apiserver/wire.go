package apiserver

import (
	"git-devops.opencsg.com/product/community/starhub-server/config"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/httpbase"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/router"
	"github.com/google/wire"
)

// WireSet provides a wire set for this package.
var WireSet = wire.NewSet(
	ProvideServerLogger,
	ProvideGracefulServer,
)

func ProvideServerLogger() log.Logger {
	return log.Clone(log.Namespace("server"))
}

// func ProvideServerOpt(
// 	config *config.Config,
// 	cache *cache.Cache,
// 	db *model.DB,
// 	logger log.Logger,
// 	sh *serverhost.ServerHost,
// ) *ServerOpt {
// 	return &ServerOpt{
// 		Port:          config.APIServer.Port,
// 		Logger:        logger,
// 		DB:            db,
// 		Cache:         cache,
// 		EnableSwagger: config.EnableSwagger,
// 		ServerHost:    sh,
// 	}
// }

// func ProvideGracefulServer(opt *ServerOpt) (server *httpbase.GracefulServer, err error) {
// 	return NewServer(opt)
// }

func ProvideGracefulServer(config *config.Config, logger log.Logger, router *router.Router) *httpbase.GracefulServer {
	return NewServer(config, logger, router)
}
