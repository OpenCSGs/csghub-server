package middleware

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/store/database"
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

func RepoMapping(repo_type types.RepositoryType) gin.HandlerFunc {
	mirrorStore := database.NewMirrorStore()
	return func(ctx *gin.Context) {
		slog.Debug("middleware RepoMapping called")
		common.SetRepoTypeContext(ctx, repo_type)
		namespace := ctx.Param("namespace")
		name := ctx.Param("name")
		branch := ctx.Param("branch")
		if branch == "" {
			branch = ctx.Param("ref")
		}
		mapping := GetMapping(ctx)
		if mapping == types.CSGHubMapping {
			ctx.Next()
			return
		}
		mirror, err := mirrorStore.FindWithMapping(ctx, repo_type, namespace, name, mapping)
		//if found mirror, that means this is a synced source, otherwise it's may a user-upload repo
		if err == nil {
			repo_id := strings.Split(mirror.Repository.Path, "/")
			//set the real namespace, the name was unchange
			slog.Info("namespace changed: ", "namespace", repo_id[0])
			ctx.Set("namespace_mapped", repo_id[0])
			ctx.Set("name_mapped", repo_id[1])
			// for modelscope, the default branch is master, we should mapp it to real branch
			if (branch == "main" || branch == "master") && mirror.Repository.DefaultBranch != branch {
				ctx.Set("branch_mapped", mirror.Repository.DefaultBranch)
			}
			ctx.Next()
			return
		}
		ctx.Next()
	}
}

func GetMapping(ctx *gin.Context) types.Mapping {
	rawRp := ctx.Query("mirror")
	if rawRp == "" {
		return types.AutoMapping
	}
	return types.Mapping(rawRp)
}
