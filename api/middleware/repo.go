package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func RepoType(t types.RepositoryType) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		slog.Debug("middleware RepoType called", "repo_type", t)
		common.SetRepoTypeContext(ctx, t)
		ctx.Next()
	}
}
