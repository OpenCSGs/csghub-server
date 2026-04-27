package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	commontypes "opencsg.com/csghub-server/common/types"
)

const defaultChatMaxFallbackAttempts = 2

type chatRetryResponseWriter struct {
	downstream    CommonResponseWriter
	headers       http.Header
	statusCode    int
	buffering     bool
	committed     bool
	streamStarted bool
	bufferedBody  bytes.Buffer
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
	if isChatRetryableStatus(statusCode) {
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
		if isChatRetryableStatus(w.statusCode) {
			w.buffering = true
		} else {
			w.commit(w.statusCode)
		}
	}
	if w.buffering {
		return w.bufferedBody.Write(data)
	}
	if len(data) > 0 {
		w.streamStarted = true
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

func isChatRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func shouldRetryChatAttempt(statusCode int, streamStarted bool) bool {
	if streamStarted {
		return false
	}
	if statusCode >= 400 && statusCode < 500 {
		return false
	}
	return isChatRetryableStatus(statusCode)
}

func chatRetryReason(statusCode int) string {
	switch statusCode {
	case http.StatusBadGateway:
		return "status_502_or_connect_error"
	case http.StatusServiceUnavailable:
		return "status_503"
	case http.StatusGatewayTimeout:
		return "status_504_or_timeout"
	default:
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
	if maxFallbackAttempts < 0 {
		return defaultChatMaxFallbackAttempts
	}
	return maxFallbackAttempts
}

func buildChatAttemptTargets(primaryTarget string, defaultModelName string, endpoints []commontypes.UpstreamConfig, maxFallbackAttempts int) []chatAttemptTarget {
	primaryEndpoint := commontypes.UpstreamConfig{URL: strings.TrimSpace(primaryTarget)}
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.URL) == strings.TrimSpace(primaryTarget) {
			primaryEndpoint = endpoint
			break
		}
	}
	targets := []chatAttemptTarget{{
		Target:    strings.TrimSpace(primaryTarget),
		Endpoint:  primaryEndpoint,
		ModelName: resolveEndpointModelName(defaultModelName, primaryEndpoint),
	}}
	fallbacks := pickFallbackEndpoints(primaryTarget, defaultModelName, endpoints)
	if len(fallbacks) == 0 || maxFallbackAttempts == 0 {
		return targets
	}

	targets = append(targets, fallbacks...)
	maxTargets := normalizeChatMaxFallbackAttempts(maxFallbackAttempts) + 1
	if len(targets) > maxTargets {
		targets = targets[:maxTargets]
	}
	return targets
}

func pickFallbackEndpoints(primaryTarget string, defaultModelName string, endpoints []commontypes.UpstreamConfig) []chatAttemptTarget {
	primaryTarget = strings.TrimSpace(primaryTarget)
	candidates := make([]chatAttemptTarget, 0, len(endpoints))
	seen := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		url := strings.TrimSpace(endpoint.URL)
		if url == "" || !endpoint.Enabled || url == primaryTarget {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		candidates = append(candidates, chatAttemptTarget{
			Target:    url,
			Endpoint:  endpoint,
			ModelName: resolveEndpointModelName(defaultModelName, endpoint),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Target < candidates[j].Target
	})
	return candidates
}

func resolveProxyPathFromModelEndpoint(endpoint string, modelName string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		slog.Warn("endpoint has wrong struct", slog.String("model", modelName))
		return ""
	}
	return uri.Path
}
