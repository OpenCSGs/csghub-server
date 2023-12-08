//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"opencsg.com/starhub-server/cmd/starhub-server/cmd/common"
	"opencsg.com/starhub-server/pkg/api/controller/accesstoken"
	"opencsg.com/starhub-server/pkg/api/controller/dataset"
	"opencsg.com/starhub-server/pkg/api/controller/member"
	modelCtrl "opencsg.com/starhub-server/pkg/api/controller/model"
	"opencsg.com/starhub-server/pkg/api/controller/organization"
	"opencsg.com/starhub-server/pkg/api/controller/sshkey"
	"opencsg.com/starhub-server/pkg/api/controller/user"
	"opencsg.com/starhub-server/pkg/apiserver"
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/httpbase"
	"opencsg.com/starhub-server/pkg/model"
	"opencsg.com/starhub-server/pkg/router"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
)

func initAPIServer(ctx context.Context) (*httpbase.GracefulServer, error) {
	wire.Build(
		common.ProvideConfig,
		apiserver.WireSet,
		cache.WireSet,
		model.WireSet,
		database.WireSet,
		dataset.ProvideController,
		modelCtrl.ProvideController,
		router.WireSet,
		user.ProvideController,
		accesstoken.ProvideController,
		sshkey.ProvideController,
		organization.ProvideController,
		member.ProvideController,
		gitserver.ProvideGitServer,
	)
	return &httpbase.GracefulServer{}, nil
}
