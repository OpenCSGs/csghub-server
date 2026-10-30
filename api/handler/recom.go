package handler

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

// RecomHandler handles requests for repo recommendation
type RecomHandler struct {
	c component.RecomComponent
}

func NewRecomHandler(cfg *config.Config) (*RecomHandler, error) {
	c, err := component.NewRecomComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create RecomComponent: %w", err)
	}
	return &RecomHandler{
		c: c,
	}, nil
}

// SetOpWeight creates a weight update handler bound to the repository type fixed by its route.
// @Security     ApiKey
// @Summary      set op weight for repo recommendation
// @Description  Shared implementation for recommendation updates; each administrator route fixes the repository type and the component validates the stored type before writing.
// @Tags         Recommendation
// @Accept       json
// @Produce      json
// @Param        body  body handler.SetOpWeight.SetOpWeightReq true "json request body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
func (h *RecomHandler) SetOpWeight(repoType types.RepositoryType) gin.HandlerFunc {
	type SetOpWeightReq struct {
		RepoID int64 `json:"repo_id" binding:"required"`
		Weight int64 `json:"weight"`
	}

	return func(ctx *gin.Context) {
		var req SetOpWeightReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			httpbase.BadRequest(ctx, fmt.Errorf("unmarshal body failed, %w", err).Error())
			return
		}

		err := h.c.SetOpWeight(ctx.Request.Context(), repoType, req.RepoID, req.Weight)
		if err != nil {
			if errors.Is(err, errorx.ErrForbidden) {
				httpbase.ForbiddenError(ctx, err)
				return
			}
			slog.ErrorContext(ctx.Request.Context(), "failed to set op weight", slog.Int64("repo_id", req.RepoID), slog.Int64("weight", req.Weight),
				slog.Any("error", err))
			httpbase.ServerError(ctx, fmt.Errorf("failed to set op weight for repository %d, err:%w", req.RepoID, err))
			return
		}
		httpbase.OK(ctx, nil)
	}
}
