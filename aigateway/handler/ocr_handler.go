package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"opencsg.com/csghub-server/aigateway/component/adapter/ocr"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// OcrHandlerImpl extends OpenAIHandlerImpl to serve the /v1/ocr endpoint
// through the three-stage pipeline (Extract → Plan → Execute).
type OcrHandlerImpl struct {
	*OpenAIHandlerImpl
	ocrPipeline  *ocrPipelineHandler
	orchestrator *plan.Orchestrator
}

func NewOcrHandler(openai *OpenAIHandlerImpl) *OcrHandlerImpl {
	h := &OcrHandlerImpl{
		OpenAIHandlerImpl: openai,
		ocrPipeline:       &ocrPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// OCR handles POST /v1/ocr.
// @Summary      OCR
// @Description  Extract text from an image or document with OCR
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        file formData file true "Image or PDF file"
// @Success      200  {object}  types.OCRResponse "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/ocr [post]
func (h *OcrHandlerImpl) OCR(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.ocrPipeline, h.ocrPipeline)
}

// ocrPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/ocr endpoint.
type ocrPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

var (
	_ plan.MetadataExtractor = (*ocrPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*ocrPipelineHandler)(nil)
)

// ocrParsedBody carries the parsed OCR request and the uploaded file header
// through the pipeline phases.
type ocrParsedBody struct {
	Req        *types.OCRRequest
	FileHeader *multipart.FileHeader
}

// --- Phase 1: Extract ---

func (h *ocrPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOCRRequestSize)
	if err := c.Request.ParseMultipartForm(maxOCRMultipartMemory); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "invalid multipart form: " + err.Error(), Type: "invalid_request_error",
		}})
		return nil, err
	}
	form := c.Request.MultipartForm
	if form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "request must be multipart/form-data", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("request must be multipart/form-data")
	}

	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model cannot be empty", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("model cannot be empty")
	}

	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "File cannot be empty", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("file cannot be empty")
	}
	if len(files) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Only one file is allowed", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("only one file is allowed")
	}
	fileHeader := files[0]

	pageRanges := strings.TrimSpace(firstMultipartValue(form, "page_ranges"))
	if pageRanges != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "page_ranges is not supported", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("page_ranges is not supported")
	}

	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if _, allowed := ocrAllowedContentTypes[contentType]; !allowed {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: fmt.Sprintf("Unsupported file content type: %s", contentType), Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("unsupported content type %q", contentType)
	}
	if fileHeader.Size > maxOCRFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "File size exceeds the 20MB limit", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("file size %d exceeds limit %d", fileHeader.Size, maxOCRFileSize)
	}

	ocrReq := &types.OCRRequest{
		Model:                     modelID,
		PageRanges:                pageRanges,
		UseDocOrientationClassify: optionalMultipartBool(form, "use_doc_orientation_classify"),
		UseDocUnwarping:           optionalMultipartBool(form, "use_doc_unwarping"),
		UseTextlineOrientation:    optionalMultipartBool(form, "use_textline_orientation"),
		ReturnImage:               strings.EqualFold(firstMultipartValue(form, "return_image"), "true"),
		RawResponse:               strings.EqualFold(firstMultipartValue(form, "raw_response"), "true"),
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "ocr",
		Model:      modelID,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  false,
		Headers:    c.Request.Header,
		ParsedBody: &ocrParsedBody{Req: ocrReq, FileHeader: fileHeader},
	}, nil
}

// --- Phase 3: Execute ---

func (h *ocrPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
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
	parsed := meta.ParsedBody.(*ocrParsedBody)
	ocrReq := parsed.Req
	fileHeader := parsed.FileHeader

	// OCR adapter lookup — protocol-specific capability check.
	adapter := h.handler.ocrRegistry.GetAdapter(mt.Model)
	if adapter == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "unsupported_model", Message: fmt.Sprintf("model '%s' does not support OCR", meta.Model), Type: "invalid_request_error",
		}})
		return nil
	}
	fileType, _ := ocrFileTypeForContentType(fileHeader.Header.Get("Content-Type"))
	if !adapter.SupportsFileType(fileType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "PDF input requires a paddleocr-vl runtime", Type: "invalid_request_error",
		}})
		return nil
	}

	traceCtx, generationRecorder := h.handler.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputText,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       meta.Model,
		ModelTarget:   &resolvedModelTarget{
			Model: mt.Model, Upstream: mt.Upstream, Target: mt.Target, Host: mt.Host, ModelName: mt.ModelName,
		},
		Metadata: map[string]any{
			"aigateway.ocr.page_ranges":  ocrReq.PageRanges,
			"aigateway.ocr.return_image": ocrReq.ReturnImage,
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	fileBytes, err := readOCRUpload(fileHeader)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to read ocr upload", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: "failed to read uploaded file", Type: "internal_error",
		}})
		return nil
	}

	bodyBytes, err := adapter.BuildUpstreamRequest(&ocr.UpstreamInput{
		FileBytes:                 fileBytes,
		FileType:                  fileType,
		UseDocOrientationClassify: ocrReq.UseDocOrientationClassify,
		UseDocUnwarping:           ocrReq.UseDocUnwarping,
		UseTextlineOrientation:    ocrReq.UseTextlineOrientation,
		Visualize:                 ocrReq.ReturnImage,
		ReturnMarkdownImages:      ocrReq.ReturnImage,
	})
	if err != nil {
		if errors.Is(err, ocr.ErrUnsupportedOption) {
			finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
			}})
			return nil
		}
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to build ocr upstream request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return nil
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Del("Content-Length")

	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", mt.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(mt.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return nil
	}

	proxyToApi := adapter.EndpointPath(mt.Model)
	if mt.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(mt.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", mt.ModelName))
		} else if uri.Path != "" && uri.Path != "/" {
			proxyToApi = uri.Path
		}
	}

	slog.InfoContext(ctx, "proxy ocr request to model endpoint",
		slog.Any("target", mt.Target), slog.Any("host", mt.Host),
		slog.Any("user", meta.UserID), slog.Any("model_id", meta.Model))

	ocrCounter := token.NewOCRUsageCounter()
	w := wrapper.NewOCR(c.Writer, adapter, ocrCounter, &ocr.ResponseOptions{
		ModelID:     meta.Model,
		RawResponse: ocrReq.RawResponse,
		ReturnImage: ocrReq.ReturnImage,
	})
	rp.ServeHTTP(w, c.Request, proxyToApi, mt.Host)

	if err := w.Finalize(); err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		slog.ErrorContext(ctx, "failed to finalize ocr response", slog.Any("error", err))
		return nil
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in ocr usage recording", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if isSuccessfulStatus(w.StatusCode()) {
			var usageErr error
			usage, usageErr = ocrCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get ocr usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			metadata := map[string]any{}
			if resp := w.Response(); resp != nil {
				metadata["aigateway.ocr.pages"] = resp.Usage.Pages
				metadata["aigateway.ocr.images"] = resp.Usage.Images
			}
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   mt.Model.Provider,
				Model:      mt.ModelName,
				Usage:      usage,
				StatusCode: w.StatusCode(),
				Metadata:   metadata,
			})
			generationRecorder.End()
		}

		if isSuccessfulStatus(w.StatusCode()) && usage != nil {
			if err := h.handler.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, mt.Model, mt.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record ocr usage", slog.Any("error", err))
			}
		}
	}()

	return nil
}

// --- Error handling ---

func (h *ocrPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
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

