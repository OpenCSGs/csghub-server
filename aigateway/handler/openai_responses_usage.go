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
	tokenizerTarget := modelTarget.TokenizerTarget
	if tokenizerTarget == "" {
		tokenizerTarget = modelTarget.Target
	}
	tokenizer := token.NewTokenizerImpl(
		tokenizerTarget,
		modelTarget.Host,
		modelTarget.ModelName,
		modelTarget.Model.ImageID,
		modelTarget.Model.Provider,
	)
	return token.NewResponsesTokenCounter(tokenizer)
}

func (h *OpenAIHandlerImpl) recordResponsesUsageWithTrace(c *gin.Context, counter token.ResponsesTokenCounter, preUsage *token.Usage, nsUUID string, modelTarget *resolvedModelTarget, apikey string, recorder *responsespkg.LLMLogRecorder, traceInput responsesTracePostProcessInput) {
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
		// Use the pre-computed usage from the sync path when available;
		// fall back to a fresh counter.Usage() call only when the sync
		// pre-compute failed (nil).
		tokenUsage := preUsage
		if tokenUsage == nil && counter != nil {
			var err error
			usageCtx, cancel := context.WithTimeout(baseCtx, 2*time.Second)
			tokenUsage, err = counter.Usage(usageCtx)
			cancel()
			if err != nil {
				slog.ErrorContext(baseCtx, "failed to get responses token usage",
					slog.String("step", "token_usage"),
					slog.String("model", modelTarget.ModelName),
					slog.String("provider", modelTarget.Model.Provider),
					slog.Any("error", err))
			}
		}
		if tokenUsage != nil && tokenUsage.Source != "" {
			slog.WarnContext(baseCtx, "responses usage fallback",
				slog.String("step", "token_usage"),
				slog.String("usage_source", tokenUsage.Source),
				slog.String("usage_reason", tokenUsage.SourceReason),
				slog.String("model", modelTarget.ModelName),
				slog.String("provider", modelTarget.Model.Provider),
				slog.Int64("prompt_tokens", tokenUsage.PromptTokens),
				slog.Int64("completion_tokens", tokenUsage.CompletionTokens),
				slog.Int64("total_tokens", tokenUsage.TotalTokens))
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
		if tokenUsage != nil {
			commitCtx, cancel := context.WithTimeout(baseCtx, time.Second)
			if err := h.openaiComponent.CommitUsageLimitFromUsage(commitCtx, nsUUID, modelTarget.Model, tokenUsage); err != nil {
				slog.ErrorContext(baseCtx, "failed to commit responses usage limit",
					slog.String("step", "commit_usage_limit"),
					slog.String("model", modelTarget.ModelName),
					slog.String("provider", modelTarget.Model.Provider),
					slog.Any("error", err))
			}
			cancel()
		}
		if tokenUsage != nil && isSuccessfulStatus(traceInput.StatusCode) {
			recordCtx, cancel := context.WithTimeout(baseCtx, time.Second)
			if err := h.openaiComponent.RecordUsageFromTokenUsage(recordCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, tokenUsage, apikey); err != nil {
				slog.ErrorContext(baseCtx, "failed to record responses usage",
					slog.String("step", "record_usage"),
					slog.String("model", modelTarget.ModelName),
					slog.String("provider", modelTarget.Model.Provider),
					slog.Any("error", err))
			}
			cancel()
		}
		if h.config != nil && h.config.AIGateway.EnableLLMLog && recorder != nil && h.llmLogPublisher != nil {
			record, recordErr := recorder.Record(tokenUsage)
			if recordErr != nil {
				slog.ErrorContext(baseCtx, "failed to build responses llmlog training record",
					slog.String("step", "build_llmlog_record"),
					slog.String("model", modelTarget.ModelName),
					slog.String("provider", modelTarget.Model.Provider),
					slog.Any("error", recordErr))
				return
			}
			payload, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				slog.ErrorContext(baseCtx, "failed to marshal responses llmlog training record",
					slog.String("step", "marshal_llmlog_record"),
					slog.String("model", modelTarget.ModelName),
					slog.String("provider", modelTarget.Model.Provider),
					slog.Any("error", marshalErr))
				return
			}
			if publishErr := h.llmLogPublisher.PublishTrainingLog(payload); publishErr != nil {
				slog.ErrorContext(baseCtx, "failed to publish responses llmlog training record",
					slog.String("step", "publish_llmlog_record"),
					slog.String("model", modelTarget.ModelName),
					slog.String("provider", modelTarget.Model.Provider),
					slog.Any("error", publishErr))
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
