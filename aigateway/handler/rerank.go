package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/utils/trace"
)

// Rerank proxies rerank requests to text-ranking model endpoints
// (vllm / TEI / llama.cpp all serve a Jina-compatible /rerank API).
func (h *OpenAIHandlerImpl) Rerank(c *gin.Context) {
	ctx := c.Request.Context()
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	requestID := trace.GetTraceIDInGinContext(c)
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	var req RerankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Model == "" {
		preflight.RecordError(fmt.Errorf("model cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model cannot be empty"})
		return
	}
	if req.Query == "" {
		preflight.RecordError(fmt.Errorf("query cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query cannot be empty"})
		return
	}
	if len(req.Documents) == 0 {
		preflight.RecordError(fmt.Errorf("documents cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Documents cannot be empty"})
		return
	}
	modelID := req.Model
	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get rerank target address", err)
		return
	}

	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	// Check balance before processing request
	if err := h.openaiComponent.CheckBalance(c.Request.Context(), nsUUID); err != nil {
		h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
		return
	}

	req.Model = modelTarget.ModelName
	data, _ := json.Marshal(req)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}
	slog.InfoContext(c, "proxy rerank request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID))
	proxyToAPI := resolveProxyPathFromModelEndpoint(modelTarget.Model.Endpoint, modelTarget.ModelName)
	if proxyToAPI == "" {
		// "/rerank" is served by vllm, TEI and llama.cpp alike, while
		// "/v1/rerank" is missing from TEI
		proxyToAPI = "/rerank"
	}
	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}

	tokenCounter := h.tokenCounterFactory.NewEmbedding(token.CreateParam{
		Endpoint: modelTarget.Target,
		Host:     modelTarget.Host,
		Model:    modelTarget.ModelName,
		ImageID:  modelTarget.Model.ImageID,
		Provider: modelTarget.Model.Provider,
	})
	w := NewResponseWriterWrapperRerank(c.Writer, tokenCounter)
	// tokenizer fallback input in case the engine returns no usage info
	tokenCounter.Input(req.Query + "\n" + strings.Join(req.Documents, "\n"))

	rp.ServeHTTP(w, c.Request, proxyToAPI, modelTarget.Host)
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		w.CaptureRerankUsage()

		err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, tokenCounter, apikey)
		if err != nil {
			slog.ErrorContext(c, "failed to record rerank token usage", "error", err)
		}
	}()
}
