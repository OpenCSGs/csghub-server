package serverhost

import (
	"github.com/google/wire"
	"opencsg.com/starhub-server/config"
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
