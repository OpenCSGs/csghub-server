package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewFinetuneHandler(config *config.Config) (*FinetuneHandler, error) {
	ftComp, err := component.NewFinetuneComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &FinetuneHandler{
		ftComp:    ftComp,
		sensitive: sc,
	}, nil
}

type FinetuneHandler struct {
	ftComp    component.FinetuneComponent
	sensitive component.SensitiveComponent
}

// create finetune  godoc
// @Security     ApiKey
// @Summary      run fineune with model and dataset
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        body body types.FinetuneReq true "body setting of finetune"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes [post]
func (h *FinetuneHandler) RunFinetuneJob(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	var req types.FinetuneReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad finetune request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to check sensitive request for create finetune", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	if req.LearningRate <= 0 {
		req.LearningRate = 0.0001
	}
	req.Username = currentUser
	finetune, err := h.ftComp.CreateFinetuneJob(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to create finetune job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, finetune)
}

// get finetune  godoc
// @Security     ApiKey
// @Summary      get Finetune job by id
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.EvaluationRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes/{id} [get]
func (h *FinetuneHandler) GetFinetuneJob(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format for get finetune", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.FinetineGetReq{}
	req.ID = id
	req.Username = currentUser
	finetune, err := h.ftComp.GetFinetuneJob(ctx.Request.Context(), *req)
	if err != nil {
		slog.Error("Failed to get finetune job by id", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, finetune)
}

// deleteFinetune  godoc
// @Security     ApiKey
// @Summary      delete finetune job by id
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes/{id} [delete]
func (h *FinetuneHandler) DeleteFinetuneJob(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format for delete finetune", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.ArgoWorkFlowDeleteReq{}
	req.ID = id
	req.Username = currentUser
	err = h.ftComp.DeleteFinetuneJob(ctx.Request.Context(), *req)
	if err != nil {
		slog.Error("failed to delete finetune job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
