//go:build !ee && !saas

package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func useAdvancedMiddleware(r *gin.Engine, config *config.Config) {}
func createAdvancedRoutes(apiGroup *gin.RouterGroup, adminGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, config *config.Config, mqFactory bldmq.MessageQueueFactory) error {
	repoHandler, err := handler.NewRepoHandler(config)
	if err != nil {
		return fmt.Errorf("failed to create repo handler: %w", err)
	}
	createRepoRoutes(apiGroup, middlewareCollection, repoHandler)

	return nil
}

func createExtendedUserRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxyHandler *handler.InternalServiceProxyHandler) {
}

func createXnetRoutes(_ *gin.Engine, _ middleware.MiddlewareCollection, _ *config.Config) error {
	return nil
}

func createComputingRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *config.Config, _ *handler.ClusterHandler) error {
	return nil
}

func createRepoRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, repoHandler *handler.RepoHandler) {
	modelsGroup := apiGroup.Group("/models")
	modelsGroup.Use(middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists)
	modelsGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoHandler.MirrorFromSaas)

	datasetsGroup := apiGroup.Group("/datasets")
	datasetsGroup.Use(middleware.RepoType(types.DatasetRepo), middlewareCollection.Repo.RepoExists)
	datasetsGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoHandler.MirrorFromSaas)

	codesGroup := apiGroup.Group("/codes")
	codesGroup.Use(middleware.RepoType(types.CodeRepo), middlewareCollection.Repo.RepoExists)
	codesGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoHandler.MirrorFromSaas)

	spacesGroup := apiGroup.Group("/spaces")
	spacesGroup.Use(middleware.RepoType(types.SpaceRepo), middlewareCollection.Repo.RepoExists)
	spacesGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoHandler.MirrorFromSaas)

	promptGroup := apiGroup.Group("/prompts")
	promptGroup.Use(middleware.RepoType(types.PromptRepo), middlewareCollection.Repo.RepoExists)
	promptGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoHandler.MirrorFromSaas)

	mcpGroup := apiGroup.Group("/mcps")
	mcpGroup.Use(middleware.RepoType(types.MCPServerRepo), middlewareCollection.Repo.RepoExists)
	mcpGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoHandler.MirrorFromSaas)
}
