package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

// RecomHandler handles requests for repo recommendation
type RecomHandler struct {
	c *component.RecomComponent
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

// SetOpWeight   godoc
// @Security     ApiKey
// @Summary      set op weight for repo recommendation
// @Tags         Recommendation
// @Accept       json
// @Produce      json
// @Param        body  body handler.SetOpWeight.SetOpWeightReq true "json request body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /recom/opweight [post]
func (h *RecomHandler) SetOpWeight(ctx *gin.Context) {
	type SetOpWeightReq struct {
		RepoID int64 `json:"repo_id" binding:"required"`
		Weight int64 `json:"weight" binding:"required"`
	}

	var req SetOpWeightReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("unmarshal body failed, %w", err).Error())
		return
	}

	err := h.c.SetOpWeight(ctx, req.RepoID, req.Weight)
	if err != nil {
		slog.Error("failed to set op weight", slog.Int64("repo_id", req.RepoID), slog.Int64("weight", req.Weight),
			slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to set op weight for repository %d, err:%w", req.RepoID, err))
		return
	}
	httpbase.OK(ctx, nil)
}
