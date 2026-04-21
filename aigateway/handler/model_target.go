package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type resolvedModelTarget struct {
	Model     *types.Model
	TargetReq commonType.EndpointReq
	Target    string
	Host      string
	ModelName string
}

type modelTargetError struct {
	Status    int
	APIError  types.Error
	Cause     error
	Model     *types.Model
	TargetReq commonType.EndpointReq
	Target    string
	Host      string
}

func (e *modelTargetError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.APIError.Message
}

func (h *OpenAIHandlerImpl) resolveModelTarget(ctx context.Context, username, modelID string) (*resolvedModelTarget, error) {
	model, err := h.openaiComponent.GetModelByID(ctx, username, modelID)
	if err != nil {
		return nil, &modelTargetError{
			Status: http.StatusInternalServerError,
			APIError: types.Error{
				Code:    "internal_error",
				Message: err.Error(),
				Type:    "internal_error",
			},
			Cause: err,
		}
	}
	if model == nil {
		return nil, &modelTargetError{
			Status: http.StatusBadRequest,
			APIError: types.Error{
				Code:    "model_not_found",
				Message: fmt.Sprintf("model '%s' not found", modelID),
				Type:    "invalid_request_error",
			},
		}
	}

	targetReq := commonType.EndpointReq{
		ClusterID: model.ClusterID,
		Target:    model.Endpoint,
		Host:      "",
		Endpoint:  model.Endpoint,
		SvcName:   model.SvcName,
	}

	target := ""
	host := ""
	modelName := ""
	if len(model.SvcName) > 0 {
		cluster, err := h.clusterComp.GetClusterByID(ctx, targetReq.ClusterID)
		if err != nil {
			return nil, &modelTargetError{
				Status: http.StatusBadRequest,
				APIError: types.Error{
					Code:    "cluster_not_found",
					Message: fmt.Sprintf("cluster '%s' not found", model.ClusterID),
					Type:    "invalid_request_error",
				},
				Cause:     err,
				Model:     model,
				TargetReq: targetReq,
			}
		}
		target, host, _ = common.ExtractDeployTargetAndHost(ctx, cluster, targetReq)
		modelName = model.CSGHubModelID
	} else {
		target = model.Endpoint
		modelName = model.ID
	}

	if len(target) < 1 {
		return nil, &modelTargetError{
			Status: http.StatusBadRequest,
			APIError: types.Error{
				Code:    "model_not_running",
				Message: fmt.Sprintf("model '%s' not running", modelID),
				Type:    "invalid_request_error",
			},
			Model:     model,
			TargetReq: targetReq,
			Target:    target,
			Host:      host,
		}
	}

	return &resolvedModelTarget{
		Model:     model,
		TargetReq: targetReq,
		Target:    target,
		Host:      host,
		ModelName: modelName,
	}, nil
}

func handleModelTargetError(c *gin.Context, ctx context.Context, modelID, logMessage string, err error) {
	var targetErr *modelTargetError
	if !errors.As(err, &targetErr) {
		slog.ErrorContext(ctx, logMessage, slog.String("model_id", modelID), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code:    "internal_error",
			Message: err.Error(),
			Type:    "internal_error",
		}})
		return
	}

	switch targetErr.APIError.Code {
	case "internal_error":
		slog.ErrorContext(ctx, "failed to get model by id", slog.String("model_id", modelID), slog.Any("error", targetErr.Cause))
	case "model_not_running":
		slog.ErrorContext(ctx, logMessage, slog.Any("model", targetErr.Model), slog.Any("targetReq", targetErr.TargetReq),
			slog.String("model_id", modelID), slog.String("target", targetErr.Target), slog.String("host", targetErr.Host))
	}

	c.JSON(targetErr.Status, gin.H{"error": targetErr.APIError})
}
