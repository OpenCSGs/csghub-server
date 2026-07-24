package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component/adapter/ocr"
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/utils/trace"
)

const (
	maxOCRMultipartMemory = 32 << 20 // 32MB, matches EditImage
	maxOCRFileSize        = 20 << 20 // 20MB uploaded file limit
	// Bounds the whole request body (file + multipart overhead + other fields)
	// so oversized uploads are rejected before being read to disk.
	maxOCRRequestSize = maxOCRFileSize + 1<<20
)

var ocrAllowedContentTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
	"image/bmp":  {},
	"image/tiff": {},
}

// OCR godoc
// @Security     ApiKey
// @Summary      Extract text from an image with OCR
// @Description  Sends a multipart OCR request to a PaddleOCR-capable backend model and returns normalized text, pages and lines. PDF input is not supported yet; page_ranges is accepted for API stability but not forwarded upstream.
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        file formData file true "Image file (png, jpeg, webp, bmp, tiff)"
// @Param        page_ranges formData string false "Page range for multi-page input, e.g. 1,3-5 (reserved)"
// @Param        use_doc_orientation_classify formData bool false "Enable document orientation classification"
// @Param        use_doc_unwarping formData bool false "Enable document unwarping"
// @Param        use_textline_orientation formData bool false "Enable text line orientation classification"
// @Param        return_image formData bool false "Ask upstream to visualize results (images surface only in raw_result)"
// @Param        raw_response formData bool false "Include the raw upstream OCR result in the response"
// @Success      200  {object}  types.OCRResponse "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/ocr [post]
func (h *OpenAIHandlerImpl) OCR(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	ctx := c.Request.Context()
	requestID := trace.GetTraceIDInGinContext(c)
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	ocrReq, fileHeader, ok := h.parseOCRMultipartForm(c, preflight)
	if !ok {
		return
	}
	modelID := ocrReq.Model

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get ocr target address", err)
		return
	}
	if !supportsOCRTask(modelTarget.Model) {
		err := fmt.Errorf("model '%s' does not support OCR", modelID)
		preflight.RecordError(err, "unsupported_model")
		preflight.End()
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "unsupported_model",
			Message: err.Error(),
			Type:    "invalid_request_error",
		}})
		return
	}
	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceCtx, generationRecorder := h.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputText,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       modelID,
		ModelTarget:   modelTarget,
		Metadata: map[string]any{
			"aigateway.ocr.page_ranges":  ocrReq.PageRanges,
			"aigateway.ocr.return_image": ocrReq.ReturnImage,
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
			return
		}
	}

	adapter := h.ocrRegistry.GetAdapter(modelTarget.Model)
	if adapter == nil {
		finishModalGenerationTraceWithError(generationRecorder, fmt.Errorf("no ocr adapter for model '%s'", modelID), types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "unsupported_model",
			Message: fmt.Sprintf("no ocr adapter for model '%s'", modelID),
			Type:    "invalid_request_error",
		}})
		return
	}

	fileBytes, err := readOCRUpload(fileHeader)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to read ocr upload", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: "failed to read uploaded file", Type: "internal_error",
		}})
		return
	}

	bodyBytes, err := adapter.BuildUpstreamRequest(&ocr.UpstreamInput{
		FileBytes:                 fileBytes,
		FileType:                  ocr.FileTypeImage,
		UseDocOrientationClassify: ocrReq.UseDocOrientationClassify,
		UseDocUnwarping:           ocrReq.UseDocUnwarping,
		UseTextlineOrientation:    ocrReq.UseTextlineOrientation,
		Visualize:                 ocrReq.ReturnImage,
	})
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to build ocr upstream request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Del("Content-Length")

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return
	}

	proxyToApi := adapter.EndpointPath(modelTarget.Model)
	if modelTarget.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(modelTarget.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", modelTarget.ModelName))
		} else if uri.Path != "" && uri.Path != "/" {
			proxyToApi = uri.Path
		}
	}

	slog.InfoContext(ctx, "proxy ocr request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName))

	ocrCounter := token.NewOCRUsageCounter()
	w := wrapper.NewOCR(c.Writer, adapter, ocrCounter, &ocr.ResponseOptions{
		ModelID:     modelID,
		RawResponse: ocrReq.RawResponse,
		ReturnImage: ocrReq.ReturnImage,
	})
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	if err := w.Finalize(); err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		slog.ErrorContext(ctx, "failed to finalize ocr response", slog.Any("error", err))
		return
	}

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if w.StatusCode() < http.StatusBadRequest {
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
				Provider:   modelTarget.Model.Provider,
				Model:      modelTarget.ModelName,
				Usage:      usage,
				StatusCode: w.StatusCode(),
				Metadata:   metadata,
			})
			generationRecorder.End()
		}

		if w.StatusCode() < http.StatusBadRequest && usage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record ocr usage", slog.Any("error", err))
			}
		}
	}()
}

// parseOCRMultipartForm validates the multipart request and returns the
// normalized request values and the uploaded file header. It writes the error
// response and returns ok=false on any validation failure.
func (h *OpenAIHandlerImpl) parseOCRMultipartForm(c *gin.Context, preflight *preflightTrace) (*types.OCRRequest, *multipart.FileHeader, bool) {
	fail := func(err error, message string) (*types.OCRRequest, *multipart.FileHeader, bool) {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: message,
			Type:    "invalid_request_error",
		}})
		return nil, nil, false
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOCRRequestSize)
	if err := c.Request.ParseMultipartForm(maxOCRMultipartMemory); err != nil {
		return fail(fmt.Errorf("invalid multipart form: %w", err), "invalid multipart form: "+err.Error())
	}
	form := c.Request.MultipartForm
	if form == nil {
		return fail(fmt.Errorf("request must be multipart/form-data"), "request must be multipart/form-data")
	}

	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		return fail(fmt.Errorf("model cannot be empty"), "Model cannot be empty")
	}

	files := form.File["file"]
	if len(files) == 0 {
		return fail(fmt.Errorf("file cannot be empty"), "File cannot be empty")
	}
	if len(files) > 1 {
		return fail(fmt.Errorf("only one file is allowed"), "Only one file is allowed")
	}
	fileHeader := files[0]

	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if contentType == "application/pdf" {
		return fail(fmt.Errorf("pdf input is not supported yet"), "PDF input is not supported yet")
	}
	if _, allowed := ocrAllowedContentTypes[contentType]; !allowed {
		return fail(fmt.Errorf("unsupported content type %q", contentType), fmt.Sprintf("Unsupported file content type: %s", contentType))
	}
	if fileHeader.Size > maxOCRFileSize {
		return fail(fmt.Errorf("file size %d exceeds limit %d", fileHeader.Size, maxOCRFileSize), "File size exceeds the 20MB limit")
	}

	return &types.OCRRequest{
		Model:                     modelID,
		PageRanges:                strings.TrimSpace(firstMultipartValue(form, "page_ranges")),
		UseDocOrientationClassify: optionalMultipartBool(form, "use_doc_orientation_classify"),
		UseDocUnwarping:           optionalMultipartBool(form, "use_doc_unwarping"),
		UseTextlineOrientation:    optionalMultipartBool(form, "use_textline_orientation"),
		ReturnImage:               strings.EqualFold(firstMultipartValue(form, "return_image"), "true"),
		RawResponse:               strings.EqualFold(firstMultipartValue(form, "raw_response"), "true"),
	}, fileHeader, true
}

func optionalMultipartBool(form *multipart.Form, key string) *bool {
	raw := strings.TrimSpace(firstMultipartValue(form, key))
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func readOCRUpload(fileHeader *multipart.FileHeader) ([]byte, error) {
	f, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	return io.ReadAll(f)
}

// supportsOCRTask reports whether the model can serve OCR requests. Logic is
// delegated to the OCR adapter registry so handler and adapter stay aligned.
func supportsOCRTask(model *types.Model) bool {
	if model == nil {
		return false
	}
	return ocr.NewPaddleXAdapter().CanHandle(model)
}
