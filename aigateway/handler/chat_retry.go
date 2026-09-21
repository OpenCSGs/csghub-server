package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/types"

	commontypes "opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

const defaultChatMaxFallbackAttempts = 2

// chatErrorBodyCaptureLimit caps how many bytes of a passthrough failed
// response are recorded, so a huge upstream error page cannot balloon
// gateway memory before the log-level truncation applies.
const chatErrorBodyCaptureLimit = 4 * 1024

type chatRetryResponseWriter struct {
	downstream    CommonResponseWriter
	headers       http.Header
	statusCode    int
	buffering     bool
	committed     bool
	streamStarted bool
	firstWriteAt  time.Time
	bufferedBody  bytes.Buffer
	// errorBody records body bytes of failed responses that were streamed
	// straight through to the client (client-argument errors such as 400
	// are not retried and never enter bufferedBody), so they stay available
	// for diagnostic logging after the attempt.
	errorBody bytes.Buffer
}

func newChatRetryResponseWriter(downstream CommonResponseWriter) *chatRetryResponseWriter {
	return &chatRetryResponseWriter{
		downstream: downstream,
		headers:    make(http.Header),
	}
}

func (w *chatRetryResponseWriter) Header() http.Header {
	return w.headers
}

func (w *chatRetryResponseWriter) WriteHeader(statusCode int) {
	if w.committed {
		return
	}
	w.statusCode = statusCode
	if types.ShouldAttemptFailureStatus(statusCode) {
		w.buffering = true
		return
	}
	w.commit(statusCode)
}

func (w *chatRetryResponseWriter) Write(data []byte) (int, error) {
	if !w.committed && !w.buffering {
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		if types.ShouldAttemptFailureStatus(w.statusCode) {
			w.buffering = true
		} else {
			w.commit(w.statusCode)
		}
	}
	if w.buffering {
		return w.bufferedBody.Write(data)
	}
	// Record a bounded copy of the failed response body for diagnostics
	// only; the client must still receive the original, untruncated data.
	if w.statusCode >= http.StatusBadRequest && w.errorBody.Len() < chatErrorBodyCaptureLimit {
		captured := data
		if remaining := chatErrorBodyCaptureLimit - w.errorBody.Len(); len(captured) > remaining {
			captured = captured[:remaining]
		}
		w.errorBody.Write(captured)
	}
	if len(data) > 0 {
		w.streamStarted = true
		if w.firstWriteAt.IsZero() {
			w.firstWriteAt = time.Now()
		}
	}
	return w.downstream.Write(data)
}

func (w *chatRetryResponseWriter) Flush() {
	if w.buffering {
		return
	}
	w.downstream.Flush()
}

func (w *chatRetryResponseWriter) StatusCode() int {
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

func (w *chatRetryResponseWriter) StreamStarted() bool {
	return w.streamStarted
}

func (w *chatRetryResponseWriter) FirstWriteAt() time.Time {
	return w.firstWriteAt
}

func (w *chatRetryResponseWriter) ReplayBufferedResponse() error {
	if !w.buffering {
		return nil
	}
	w.commit(w.StatusCode())
	if w.bufferedBody.Len() == 0 {
		return nil
	}
	_, err := w.downstream.Write(w.bufferedBody.Bytes())
	return err
}

// responseBodyForLog returns the upstream response body recorded for
// diagnostics: buffered failure responses keep their body in bufferedBody,
// while passthrough failure responses (e.g. 400) land in errorBody.
func (w *chatRetryResponseWriter) responseBodyForLog() string {
	if w.bufferedBody.Len() > 0 {
		return w.bufferedBody.String()
	}
	return w.errorBody.String()
}

func (w *chatRetryResponseWriter) commit(statusCode int) {
	if w.committed {
		return
	}
	copyHeader(w.downstream.Header(), w.headers)
	w.downstream.WriteHeader(statusCode)
	w.committed = true
	w.buffering = false
}

func copyHeader(dst, src http.Header) {
	for k, values := range src {
		dst.Del(k)
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}

func shouldRetryChatAttempt(statusCode int, streamStarted bool) bool {
	if streamStarted {
		return false
	}
	// Chat fallback targets are independent upstreams. Before any bytes are sent to
	// the client, any upstream failure can be caused by provider-specific auth,
	// routing, model mapping, quota, or transient availability issues, so we
	// continue trying the next configured upstream.
	return types.ShouldAttemptFailureStatus(statusCode)
}

func chatRetryReason(statusCode int) string {
	switch statusCode {
	case http.StatusBadGateway:
		return "status_502_or_connect_error"
	case http.StatusServiceUnavailable:
		return "status_503"
	case http.StatusGatewayTimeout:
		return "status_504_or_timeout"
	case http.StatusTooManyRequests:
		return "status_429"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "status_auth_error"
	case http.StatusNotFound:
		return "status_404_or_model_missing"
	case http.StatusBadRequest:
		return "status_400_or_request_incompatible"
	case http.StatusUnprocessableEntity:
		return "status_422_or_request_incompatible"
	case http.StatusRequestTimeout:
		return "status_408"
	case http.StatusTooEarly:
		return "status_425"
	case http.StatusConflict:
		return "status_409"
	default:
		if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError {
			return "status_4xx"
		}
		return "non_retryable"
	}
}

func sessionKeyDigest(sessionKey string) string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sessionKey))
	return hex.EncodeToString(sum[:8])
}

func normalizeChatMaxFallbackAttempts(maxFallbackAttempts int) int {
	if maxFallbackAttempts <= 0 {
		return defaultChatMaxFallbackAttempts
	}
	return maxFallbackAttempts
}

func buildChatAttemptTargets(primary commontypes.UpstreamConfig, upstreams []commontypes.UpstreamConfig, maxFallbackAttempts int) []commontypes.UpstreamConfig {
	if maxFallbackAttempts <= 0 || len(upstreams) <= 1 {
		return []commontypes.UpstreamConfig{}
	}
	candidates := make([]commontypes.UpstreamConfig, 0, len(upstreams)-1)
	for _, ep := range upstreams {
		url := strings.TrimSpace(ep.URL)
		if url == "" || ep.ID == primary.ID {
			continue
		}
		ep.URL = url
		candidates = append(candidates, ep)
		if len(candidates) >= maxFallbackAttempts {
			break
		}
	}
	return candidates
}

func resolveProxyPathFromModelEndpoint(endpoint string, modelName string) string {
	proxyPath := commonutils.ExtractURLPath(endpoint)
	if strings.TrimSpace(endpoint) != "" && proxyPath == "" {
		slog.Warn("endpoint has wrong struct", slog.String("model", modelName))
	}
	return proxyPath
}
