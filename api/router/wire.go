//go:build wireinject
// +build wireinject

package router

import (
	"github.com/google/wire"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/handler/callback"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/rpc"
)

var BaseServerSet = wire.NewSet(
	handler.NewGitHTTPHandler,
	handler.NewUserHandler,
	handler.NewOrganizationHandler,
	handler.NewRepoHandler,
	handler.NewModelHandler,
	handler.NewDatasetHandler,
	handler.NewMirrorHandler,
	handler.NewHFDatasetHandler,
	handler.NewListHandler,
	handler.NewEvaluationHandler,
	handler.NewCodeHandler,
	handler.NewSpaceHandler,
	handler.NewSpaceResourceHandler,
	handler.NewSpaceSdkHandler,
	handler.NewSSHKeyHandler,
	handler.NewTagHandler,
	handler.NewSensitiveHandler,
	handler.NewMirrorSourceHandler,
	handler.NewCollectionHandler,
	handler.NewClusterHandler,
	handler.NewEventHandler,
	handler.NewBroadcastHandler,
	handler.NewRuntimeArchitectureHandler,
	handler.NewSyncHandler,
	handler.NewSyncClientSettingHandler,
	handler.NewAccountingHandler,
	handler.NewRecomHandler,
	handler.NewTelemetryHandler,
	handler.NewPromptHandler,
	handler.NewInternalHandler,
	handler.NewDiscussionHandler,
	newMemoryStore,
	newUserProxyHandler,
	newDatasetViewerProxyHandler,
	rpc.NewUserSvcHttpClient,
	callback.NewGitCallbackHandler,
	NewBaseServer,
	middleware.NewMiddleware,
)
