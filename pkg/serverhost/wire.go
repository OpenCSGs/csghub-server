package serverhost

import (
	"git-devops.opencsg.com/product/community/starhub-server/config"
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	ProvideOpt,
	ProvideServerHost,
)

func ProvideOpt(config *config.Config) *Opt {
	return &Opt{
		API:      config.APIServer.ExternalHost,
		DocsHost: config.DocsHost,
	}
}

func ProvideServerHost(opt *Opt) (host *ServerHost, err error) {
	return NewServerHost(opt)
}
