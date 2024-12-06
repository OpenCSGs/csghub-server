package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewEvaluationHandler(config *config.Config) (*EvaluationHandler, error) {
	wkf, err := component.NewEvaluationComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &EvaluationHandler{
		c:  wkf,
		sc: sc,
	}, nil
}

type EvaluationHandler struct {
	c  component.EvaluationComponent
	sc component.SensitiveComponent
}

// create evaluation  godoc
// @Security     ApiKey
// @Summary      run model evaluation
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.EvaluationReq true "body setting of evaluation"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations [post]
func (h *EvaluationHandler) RunEvaluation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	var req types.EvaluationReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err := h.sc.CheckRequestV2(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	req.Username = currentUser
	evaluation, err := h.c.CreateEvaluation(ctx, req)
	if err != nil {
		slog.Error("Failed to create evaluation job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, evaluation)
}

// get evaluation  godoc
// @Security     ApiKey
// @Summary      get model evaluation
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.EvaluationRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations/{id} [get]
func (h *EvaluationHandler) GetEvaluation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.EvaluationGetReq{}
	req.ID = id
	req.Username = currentUser
	evaluation, err := h.c.GetEvaluation(ctx, *req)
	if err != nil {
		slog.Error("Failed to get evaluation job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, evaluation)

}

// deleteEvaluation  godoc
// @Security     ApiKey
// @Summary      delete model evaluation
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations/{id} [delete]
func (h *EvaluationHandler) DeleteEvaluation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.EvaluationDelReq{}
	req.ID = id
	req.Username = currentUser
	err = h.c.DeleteEvaluation(ctx, *req)
	if err != nil {
		slog.Error("Failed to delete evaluation job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
