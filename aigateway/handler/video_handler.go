package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"net/http"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// VideoHandlerImpl extends OpenAIHandlerImpl to serve the
// /v1/video/generations endpoint through the three-stage pipeline.
// GetVideo and GetVideoContent remain on OpenAIHandlerImpl as they do not
// go through the admission flow.
type VideoHandlerImpl struct {
	*OpenAIHandlerImpl
	videoPipeline *videoPipelineHandler
	orchestrator  *plan.Orchestrator
}

func NewVideoHandler(openai *OpenAIHandlerImpl) *VideoHandlerImpl {
	h := &VideoHandlerImpl{
		OpenAIHandlerImpl: openai,
		videoPipeline:     &videoPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// CreateVideo handles POST /v1/video/generations.
// @Summary      Create a video generation
// @Description  Creates an OpenAI-compatible text-to-video or image-to-video generation request.
// @Tags         AIGateway
// @Accept       json
// @Accept       multipart/form-data
// @Produce      json
// @Param        request body types.VideoGenerationRequest true "Video generation request"
// @Success      200 {object} types.VideoObject "OK"
// @Failure      400 {object} types.Error "Bad request"
// @Failure      404 {object} types.Error "Model not found"
// @Failure      500 {object} types.Error "Internal server error"
// @Router       /v1/video/generations [post]
func (h *VideoHandlerImpl) CreateVideo(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.videoPipeline, h.videoPipeline)
}

// videoPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/video/generations endpoint.
type videoPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

var (
	_ plan.MetadataExtractor = (*videoPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*videoPipelineHandler)(nil)
)

// --- Phase 1: Extract ---

func (h *videoPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	input, parseErr, ok := parseCreateVideoInput(c)
	if !ok {
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: parseErr.Error(), Type: "invalid_request_error",
			}})
		}
		return nil, parseErr
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "text-to-video",
		Model:      input.modelID,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  false,
		Headers:    c.Request.Header,
		ParsedBody: input,
	}, nil
}

// --- Phase 3: Execute ---

func (h *videoPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	ctx := c.Request.Context()
	nsUUID := meta.TenantID
	apikey := meta.APIKeyID
	requestID := commontrace.GetTraceIDInGinContext(c)

	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(meta.Model, p.ModelTarget)
		pt.End()
		plan.SetPreflightTracer(c, nil)
	}

	mt := p.ModelTarget
	input := meta.ParsedBody.(*createVideoInput)
	resolvedMT := &resolvedModelTarget{
		Model: mt.Model, Upstream: mt.Upstream, Target: mt.Target, Host: mt.Host, ModelName: mt.ModelName,
	}

	// Modal price guard — stays in Execute.
	if guardErr := checkModalRequestAllowed(mt.Model, input.adapterReq.Size); guardErr != nil {
		handleModelTargetError(c, ctx, input.modelID, "modal price guard rejected request", guardErr)
		return nil
	}

	traceCtx, generationRecorder := h.handler.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputVideo,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       input.modelID,
		ModelTarget:   resolvedMT,
		Metadata: map[string]any{
			llmtrace.TraceMetadataKeyVideoSize:     input.adapterReq.Size,
			llmtrace.TraceMetadataKeyVideoSeconds: input.adapterReq.Seconds,
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	adapter := h.handler.t2vRegistry.GetAdapter(mt.Model)
	if !validateCreateVideoAdapter(c, input, adapter, mt.Model) {
		finishModalGenerationTraceWithError(generationRecorder, fmt.Errorf("unsupported video model '%s'", input.modelID), types.TraceErrUpstreamUnavailable)
		return nil
	}

	// Content-safety check via CheckImagePrompts (stays in Execute — bypasses SensitivePolicy gate).
	// Balance check was already done by the Planner.
	result, err := h.handler.modComponent.CheckImagePrompts(ctx, input.adapterReq.Prompt, nsUUID)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		writeVideoAPIError(c, http.StatusInternalServerError, "moderation_error", "failed to check video prompts: "+err.Error(), "internal_error")
		return nil
	}
	if result != nil && result.IsSensitive {
		finishModalGenerationTraceWithError(generationRecorder, ErrSensitiveContent, types.TraceErrSensitivePrompt)
		writeVideoAPIError(c, http.StatusBadRequest, "content_policy_violation", "Input data may contain inappropriate content.", "invalid_request_error")
		return nil
	}

	providerReq, ok := buildCreateVideoProviderRequest(c, ctx, input, adapter, resolvedMT, generationRecorder)
	if !ok {
		return nil
	}
	capture, ok := proxyCreateVideoRequest(c, ctx, providerReq, resolvedMT, generationRecorder)
	if !ok {
		return nil
	}
	body := capture.Body()
	var videoResp *types.VideoObject
	if isSuccessfulStatus(capture.StatusCode()) {
		var videoProviderResp *text2video.ProviderResponse
		videoProviderResp, ok = parseCreateVideoProviderResponse(c, ctx, adapter, body, generationRecorder)
		if !ok {
			return nil
		}
		videoResp = videoProviderResp.Video
		if videoResp != nil && videoResp.ID != "" && h.handler.aiGenerationStore != nil {
			videoID, ok := h.handler.createVideoGenerationTask(c, ctx, nsUUID, apikey, input, resolvedMT, videoProviderResp, videoResp, generationRecorder)
			if !ok {
				return nil
			}
			normalized := *videoResp
			normalized.ID = videoID
			videoResp = &normalized
			body = normalizeVideoResponseBody(adapter, body, videoProviderResp.Video, videoID)
		}
	}
	if generationRecorder != nil {
		recordModalGenerationTraceCompletion(modalTraceCompletionInput{
			Recorder:   generationRecorder,
			Provider:   mt.Model.Provider,
			Model:      mt.ModelName,
			StatusCode: capture.StatusCode(),
			Metadata:   videoTraceCompletionMetadata(videoResp, input),
		})
		generationRecorder.End()
	}

	copyProxyResponse(c, capture.Header(), capture.StatusCode(), body)
	return nil
}

// --- Error handling ---

func (h *videoPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.RecordError(err, "plan_error")
		plan.SetPreflightTracer(c, nil)
	}

	frontendURL := ""
	if h.handler.config != nil {
		frontendURL = h.handler.config.Frontend.URL
	}
	handleOpenAIPlanError(c, meta, p, err, frontendURL)
}

