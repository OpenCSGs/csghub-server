package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/types"
)

func CreateMCPServerRoutes(
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	mcpServerHandler *handler.MCPServerHandler,
	repoCommonHandler *handler.RepoHandler) {
	mcpGroup := apiGroup.Group("/mcps")
	mcpGroup.Use(middleware.RepoType(types.MCPServerRepo))
	{
		// mcp server handler functions
		mcpGroup.GET("", mcpServerHandler.Index)
		mcpGroup.GET("/tools", mcpServerHandler.Properties)
		mcpGroup.GET("/:namespace/:name", mcpServerHandler.Show)

		mcpGroup.POST("", middlewareCollection.Auth.NeedLogin, mcpServerHandler.Create)
		mcpGroup.DELETE("/:namespace/:name", middlewareCollection.Auth.NeedLogin, mcpServerHandler.Delete)
		mcpGroup.PUT("/:namespace/:name", middlewareCollection.Auth.NeedLogin, mcpServerHandler.Update)
		mcpGroup.POST("/:namespace/:name/deploys", middlewareCollection.Auth.NeedLogin, mcpServerHandler.Deploy)

	}
	{
		// repo common handler functions
		mcpGroup.GET("/:namespace/:name/branches", repoCommonHandler.Branches)
		mcpGroup.GET("/:namespace/:name/tags", repoCommonHandler.Tags)
		mcpGroup.POST("/:namespace/:name/preupload/:revision", repoCommonHandler.Preupload)
		mcpGroup.POST("/:namespace/:name/tags/:category", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateTags)
		mcpGroup.GET("/:namespace/:name/last_commit", repoCommonHandler.LastCommit)
		mcpGroup.GET("/:namespace/:name/commit/:commit_id", repoCommonHandler.CommitWithDiff)
		mcpGroup.POST("/:namespace/:name/commit/:revision", repoCommonHandler.CommitFiles)
		mcpGroup.GET("/:namespace/:name/remote_diff", repoCommonHandler.RemoteDiff)
		mcpGroup.GET("/:namespace/:name/tree", repoCommonHandler.Tree)
		mcpGroup.GET("/:namespace/:name/refs/:ref/tree/*path", repoCommonHandler.TreeV2)
		mcpGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", repoCommonHandler.LogsTree)
		mcpGroup.GET("/:namespace/:name/commits", repoCommonHandler.Commits)
		mcpGroup.POST("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateFile)
		mcpGroup.GET("/:namespace/:name/raw/*file_path", repoCommonHandler.FileRaw)
		mcpGroup.GET("/:namespace/:name/blob/*file_path", repoCommonHandler.FileInfo)
		mcpGroup.GET("/:namespace/:name/download/*file_path", repoCommonHandler.DownloadFile)
		mcpGroup.GET("/:namespace/:name/resolve/*file_path", repoCommonHandler.ResolveDownload)
		mcpGroup.PUT("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateFile)
		mcpGroup.POST("/:namespace/:name/update_downloads", repoCommonHandler.UpdateDownloads)
		mcpGroup.PUT("/:namespace/:name/incr_downloads", middlewareCollection.Auth.NeedLogin, repoCommonHandler.IncrDownloads)
		mcpGroup.POST("/:namespace/:name/upload_file", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UploadFile)
		mcpGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoCommonHandler.MirrorFromSaas)
	}
}
