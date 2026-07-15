package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
)

// ListVoices godoc
// @Security     ApiKey
// @Summary      List available voices of a text-to-speech model
// @Description  Proxies the OpenAI-compatible voices listing request to the backend TTS model
// @Tags         AIGateway
// @Produce      json
// @Param        model query string true "Model ID"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/voices [get]
func (h *OpenAIHandlerImpl) ListVoices(c *gin.Context) {
	modelID := strings.TrimSpace(c.Query("model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model cannot be empty, pass it as the 'model' query parameter", Type: "invalid_request_error",
		}})
		return
	}
	modelTarget, ok := h.resolveVoicesTarget(c, modelID, false)
	if !ok {
		return
	}
	h.proxyVoicesRequest(c, modelID, modelTarget, voicesProxyPath(c, modelTarget.Model.Endpoint))
}

// UploadVoice godoc
// @Security     ApiKey
// @Summary      Upload a voice sample for voice cloning
// @Description  Proxies the multipart voice upload request to the backend TTS model. Only the deploy owner or platform admins are allowed.
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        audio_sample formData file true "Audio sample file"
// @Param        consent formData string true "Consent recording ID"
// @Param        name formData string true "Name for the new voice"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      403  {object}  error "Permission denied"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/voices [post]
func (h *OpenAIHandlerImpl) UploadVoice(c *gin.Context) {
	h.uploadOrUpdateVoice(c)
}

// UpdateVoice godoc
// @Security     ApiKey
// @Summary      Update an uploaded voice sample
// @Description  Overwrites an existing uploaded voice by name (the backend voice upload has overwrite semantics). Only the deploy owner or platform admins are allowed.
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        audio_sample formData file true "Audio sample file"
// @Param        consent formData string true "Consent recording ID"
// @Param        name formData string true "Name of the voice to update"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      403  {object}  error "Permission denied"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/voices [put]
func (h *OpenAIHandlerImpl) UpdateVoice(c *gin.Context) {
	// The backend only exposes POST /v1/audio/voices, which overwrites an
	// existing voice with the same name.
	c.Request.Method = http.MethodPost
	h.uploadOrUpdateVoice(c)
}

// maxVoiceUploadBytes bounds the multipart voice upload body (voice samples
// are short reference audio clips) to prevent memory/bandwidth exhaustion.
const maxVoiceUploadBytes = 64 << 20

func (h *OpenAIHandlerImpl) uploadOrUpdateVoice(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxVoiceUploadBytes)
	form, err := c.MultipartForm()
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: fmt.Sprintf("request body too large, limit is %d bytes", maxVoiceUploadBytes), Type: "invalid_request_error",
			}})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "invalid multipart form: " + err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	if form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "request must be multipart/form-data", Type: "invalid_request_error",
		}})
		return
	}
	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model cannot be empty", Type: "invalid_request_error",
		}})
		return
	}
	if len(form.File["audio_sample"]) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "audio_sample cannot be empty", Type: "invalid_request_error",
		}})
		return
	}

	modelTarget, ok := h.resolveVoicesTarget(c, modelID, true)
	if !ok {
		return
	}

	body, contentType, err := rewriteMultipartModelStreamWithOptions(form, modelTarget.ModelName, multipartRewriteOptions{})
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to rewrite voices request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}
	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

	h.proxyVoicesRequest(c, modelID, modelTarget, voicesProxyPath(c, modelTarget.Model.Endpoint))
}

// DeleteVoice godoc
// @Security     ApiKey
// @Summary      Delete an uploaded voice sample
// @Description  Proxies the voice deletion request to the backend TTS model. Only the deploy owner or platform admins are allowed.
// @Tags         AIGateway
// @Produce      json
// @Param        name path string true "Name of the voice to delete"
// @Param        model query string true "Model ID"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      403  {object}  error "Permission denied"
// @Failure      404  {object}  error "Model or voice not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/voices/{name} [delete]
func (h *OpenAIHandlerImpl) DeleteVoice(c *gin.Context) {
	modelID := strings.TrimSpace(c.Query("model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model cannot be empty, pass it as the 'model' query parameter", Type: "invalid_request_error",
		}})
		return
	}
	voiceName := strings.TrimSpace(c.Param("name"))
	if voiceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Voice name cannot be empty", Type: "invalid_request_error",
		}})
		return
	}

	modelTarget, ok := h.resolveVoicesTarget(c, modelID, true)
	if !ok {
		return
	}

	proxyToApi := voicesProxyPath(c, modelTarget.Model.Endpoint)
	if proxyToApi != "" {
		proxyToApi = strings.TrimSuffix(proxyToApi, "/") + "/" + url.PathEscape(voiceName)
	}
	h.proxyVoicesRequest(c, modelID, modelTarget, proxyToApi)
}

// resolveVoicesTarget resolves the model target for voices management
// requests. When requireManage is true, only the deploy owner or platform
// admins are allowed; other users get 403.
func (h *OpenAIHandlerImpl) resolveVoicesTarget(c *gin.Context, modelID string, requireManage bool) (*resolvedModelTarget, bool) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	ctx := c.Request.Context()

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, ctx, modelID, "failed to get voices target address", err)
		return nil, false
	}

	if requireManage {
		allowed, err := h.openaiComponent.CanManageModel(ctx, username, nsUUID, modelTarget.Model)
		if err != nil {
			slog.ErrorContext(ctx, "failed to check voice management permission", slog.String("model_id", modelID), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
				Code: "internal_error", Message: "failed to check permission: " + err.Error(), Type: "internal_error",
			}})
			return nil, false
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": types.Error{
				Code: "insufficient_permissions", Message: "only the model owner or admins can manage voices", Type: "invalid_request_error",
			}})
			return nil, false
		}
	}

	return modelTarget, true
}

// proxyVoicesRequest proxies the voices management request through. Voice
// management does not generate tokens, so no balance check or usage
// recording is performed.
func (h *OpenAIHandlerImpl) proxyVoicesRequest(c *gin.Context, modelID string, modelTarget *resolvedModelTarget, proxyToApi string) {
	username := httpbase.GetCurrentUser(c)
	ctx := c.Request.Context()

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return
	}

	slog.InfoContext(ctx, "proxy audio voices request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName), slog.Any("method", c.Request.Method))
	rp.ServeHTTP(voicesPassthroughWriter{w: c.Writer}, c.Request, proxyToApi, modelTarget.Host)
}

// voicesPassthroughWriter exposes only plain write/flush to the reverse
// proxy, hiding gin ResponseWriter extras such as CloseNotify whose
// underlying writer may not support them.
type voicesPassthroughWriter struct {
	w gin.ResponseWriter
}

func (w voicesPassthroughWriter) Header() http.Header {
	return w.w.Header()
}

func (w voicesPassthroughWriter) WriteHeader(statusCode int) {
	w.w.WriteHeader(statusCode)
}

func (w voicesPassthroughWriter) Write(data []byte) (int, error) {
	return w.w.Write(data)
}

func (w voicesPassthroughWriter) Flush() {
	w.w.Flush()
}

// voicesProxyPath derives the upstream path for voices requests. External
// model endpoints are configured with the full speech API path (e.g.
// https://host/v1/audio/speech), so the speech suffix is remapped to the
// voices path. CSGHub serverless endpoints have no path and pass the incoming
// request path through unchanged.
func voicesProxyPath(c *gin.Context, endpoint string) string {
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "endpoint has wrong struct", slog.String("endpoint", endpoint))
		return ""
	}
	path := uri.Path
	if path == "" || path == "/" {
		return ""
	}
	if strings.HasSuffix(path, "/audio/speech") {
		return strings.TrimSuffix(path, "/audio/speech") + "/audio/voices"
	}
	return path
}
