package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
)

func (h *OpenAIHandlerImpl) newResponsesTokenCounter(modelTarget *resolvedModelTarget) token.ResponsesTokenCounter {
	if modelTarget == nil || modelTarget.Model == nil {
		return token.NewResponsesTokenCounter(nil)
	}
	tokenizer := token.NewTokenizerImpl(
		modelTarget.Target,
		modelTarget.Host,
		modelTarget.ModelName,
		modelTarget.Model.ImageID,
		modelTarget.Model.Provider,
	)
	return token.NewResponsesTokenCounter(tokenizer)
}

func (h *OpenAIHandlerImpl) recordResponsesUsage(c *gin.Context, counter token.ResponsesTokenCounter, nsUUID string, modelTarget *resolvedModelTarget, apikey string) {
	if counter == nil || modelTarget == nil || modelTarget.Model == nil {
		return
	}
	baseCtx := context.WithoutCancel(c.Request.Context())
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(baseCtx, "panic in responses usage post-process", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(baseCtx, 3*time.Second)
		defer cancel()

		tokenUsage, err := counter.Usage(usageCtx)
		if err != nil {
			slog.ErrorContext(usageCtx, "failed to get responses token usage", slog.Any("error", err))
			return
		}
		if err := h.openaiComponent.CommitUsageLimit(usageCtx, nsUUID, modelTarget.Model, counter); err != nil {
			slog.ErrorContext(usageCtx, "failed to commit responses usage limit", slog.Any("error", err))
		}
		if tokenUsage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, tokenUsage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record responses usage", slog.Any("error", err))
			}
		}
	}()
}
