//go:build !ee && !saas

package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// MirrorFromSaas godoc
// @Security     ApiKey
// @Summary      Mirror repo from OpenCSG Saas(only on-premises)
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Success      200  {object}  types.Response{data=database.Mirror} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror_from_saas [post]
func (h *RepoHandler) MirrorFromSaas(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	if !strings.HasPrefix(namespace, types.OpenCSGPrefix) {
		httpbase.BadRequest(ctx, "Repo could not be mirrored")
		return
	}
	err = h.c.MirrorFromSaas(ctx.Request.Context(), namespace, name, currentUser, repoType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create mirror for", slog.String("repo_type", string(repoType)), slog.String("path", fmt.Sprintf("%s/%s", namespace, name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
