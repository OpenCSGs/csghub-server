package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/types"
)

func CreateMCPServerRoutes(
	apiGroup *gin.RouterGroup,
	authCollection middleware.AuthenticatorCollection,
	mcpServerHandler *handler.MCPServerHandler,
	repoCommonHandler *handler.RepoHandler) {
	mcpGroup := apiGroup.Group("/mcps")
	{
		// mcp server handler functions
		mcpGroup.GET("", mcpServerHandler.Index)
		mcpGroup.GET("/tools", mcpServerHandler.Properties)
		mcpGroup.GET("/:namespace/:name", mcpServerHandler.Show)

		mcpGroup.POST("", authCollection.NeedLogin, mcpServerHandler.Create)
		mcpGroup.DELETE("/:namespace/:name", authCollection.NeedLogin, mcpServerHandler.Delete)
		mcpGroup.PUT("/:namespace/:name", authCollection.NeedLogin, mcpServerHandler.Update)

		// repo common handler functions
		mcpGroup.GET("/:namespace/:name/branches", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.Branches)
		mcpGroup.GET("/:namespace/:name/tags", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.Tags)
		mcpGroup.POST("/:namespace/:name/tags/:category", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.UpdateTags)
		mcpGroup.GET("/:namespace/:name/last_commit", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.LastCommit)
		mcpGroup.GET("/:namespace/:name/commit/:commit_id", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.CommitWithDiff)
		mcpGroup.GET("/:namespace/:name/tree", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.Tree)
		mcpGroup.GET("/:namespace/:name/refs/:ref/tree/*path", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.TreeV2)
		mcpGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.LogsTree)
		mcpGroup.GET("/:namespace/:name/commits", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.Commits)
		mcpGroup.POST("/:namespace/:name/raw/*file_path", authCollection.NeedLogin, middleware.RepoType(types.MCPServerRepo), repoCommonHandler.CreateFile)
		mcpGroup.GET("/:namespace/:name/raw/*file_path", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.FileRaw)
		mcpGroup.GET("/:namespace/:name/blob/*file_path", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.FileInfo)
		mcpGroup.GET("/:namespace/:name/download/*file_path", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.DownloadFile)
		mcpGroup.GET("/:namespace/:name/resolve/*file_path", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.ResolveDownload)
		mcpGroup.PUT("/:namespace/:name/raw/*file_path", authCollection.NeedLogin, middleware.RepoType(types.MCPServerRepo), repoCommonHandler.UpdateFile)
		mcpGroup.POST("/:namespace/:name/update_downloads", middleware.RepoType(types.MCPServerRepo), repoCommonHandler.UpdateDownloads)
		mcpGroup.PUT("/:namespace/:name/incr_downloads", authCollection.NeedLogin, middleware.RepoType(types.MCPServerRepo), repoCommonHandler.IncrDownloads)
		mcpGroup.POST("/:namespace/:name/upload_file", authCollection.NeedLogin, middleware.RepoType(types.MCPServerRepo), repoCommonHandler.UploadFile)
	}
}
