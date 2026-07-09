package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
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

func (h *OpenAIHandlerImpl) recordResponsesUsageWithTrace(c *gin.Context, counter token.ResponsesTokenCounter, nsUUID string, modelTarget *resolvedModelTarget, apikey string, recorder *responsespkg.LLMLogRecorder, traceInput responsesTracePostProcessInput) {
	if modelTarget == nil || modelTarget.Model == nil {
		return
	}
	baseCtx := context.WithoutCancel(c.Request.Context())
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(baseCtx, "panic in responses usage post-process", slog.Any("panic", r))
				if traceInput.Recorder != nil {
					traceInput.Recorder.End()
				}
			}
		}()
		usageCtx, cancel := context.WithTimeout(baseCtx, 3*time.Second)
		defer cancel()

		var tokenUsage *token.Usage
		if counter != nil {
			var err error
			tokenUsage, err = counter.Usage(usageCtx)
			if err != nil {
				slog.ErrorContext(usageCtx, "failed to get responses token usage", slog.Any("error", err))
			}
		}
		if traceInput.Recorder != nil {
			var inputMsgs, outputMsgs []types.GenerationMessage
			if recorder != nil {
				in, out := recorder.Messages()
				inputMsgs = llmlogMessagesToGenerationMessages(in)
				outputMsgs = llmlogMessagesToGenerationMessages(out)
			}
			recordResponsesTraceCompletion(traceInput, modelTarget.Model.Provider, modelTarget.ModelName, tokenUsage, inputMsgs, outputMsgs, recorderTraceInfo(recorder))
			traceInput.Recorder.End()
		}
		if counter != nil {
			if err := h.openaiComponent.CommitUsageLimit(usageCtx, nsUUID, modelTarget.Model, counter); err != nil {
				slog.ErrorContext(usageCtx, "failed to commit responses usage limit", slog.Any("error", err))
			}
		}
		if tokenUsage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, tokenUsage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record responses usage", slog.Any("error", err))
			}
		}
		if h.config != nil && h.config.AIGateway.EnableLLMLog && recorder != nil && h.llmLogPublisher != nil {
			record, recordErr := recorder.Record(tokenUsage)
			if recordErr != nil {
				slog.ErrorContext(usageCtx, "failed to build responses llmlog training record", slog.Any("error", recordErr))
				return
			}
			payload, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				slog.ErrorContext(usageCtx, "failed to marshal responses llmlog training record", slog.Any("error", marshalErr))
				return
			}
			if publishErr := h.llmLogPublisher.PublishTrainingLog(payload); publishErr != nil {
				slog.ErrorContext(usageCtx, "failed to publish responses llmlog training record", slog.Any("error", publishErr))
			}
		}
	}()
}

func recorderTraceInfo(recorder *responsespkg.LLMLogRecorder) commontypes.LLMLogTraceInfo {
	if recorder == nil {
		return commontypes.LLMLogTraceInfo{}
	}
	return recorder.TraceInfo()
}
