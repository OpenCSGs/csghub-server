package router

import (
	"github.com/google/wire"
	"opencsg.com/starhub-server/config"
	"opencsg.com/starhub-server/pkg/api/controller/accesstoken"
	"opencsg.com/starhub-server/pkg/api/controller/dataset"
	"opencsg.com/starhub-server/pkg/api/controller/member"
	"opencsg.com/starhub-server/pkg/api/controller/model"
	"opencsg.com/starhub-server/pkg/api/controller/organization"
	"opencsg.com/starhub-server/pkg/api/controller/sshkey"
	"opencsg.com/starhub-server/pkg/api/controller/user"
)

// WireSet provides a wire set for this package.
var WireSet = wire.NewSet(
	ProvideAPIHandler,
	ProvideGitHandler,
	ProvideRouter,
)

func ProvideAPIHandler(
	config *config.Config,
	modelCtrl *model.Controller,
	datasetCtrl *dataset.Controller,
	userCtrl *user.Controller,
	acCtrl *accesstoken.Controller,
	sshKeyCtrl *sshkey.Controller,
	orgCtrl *organization.Controller,
	memberCtrl *member.Controller,
) APIHandler {
	return NewAPIHandler(config, modelCtrl, datasetCtrl, userCtrl, acCtrl, sshKeyCtrl, orgCtrl, memberCtrl)
}

func ProvideGitHandler(
	config *config.Config,
	modelCtrl *model.Controller,
	datasetCtrl *dataset.Controller,
) GitHandler {
	return NewGitHandler(config, modelCtrl, datasetCtrl)
}

func ProvideRouter(api APIHandler, git GitHandler) *Router {
	return NewRouter(api, git)
}
