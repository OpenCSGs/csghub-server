//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/cmd/starhub-server/cmd/common"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/accesstoken"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	modelCtrl "git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/user"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/apiserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/httpbase"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/router"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"github.com/google/wire"
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
		gitserver.ProvideGitServer,
	)
	return &httpbase.GracefulServer{}, nil
}
