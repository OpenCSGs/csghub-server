package decisions

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	decisioncomp "opencsg.com/csghub-server/aigateway/component/decisions"
	"opencsg.com/csghub-server/aigateway/types"
)

// decisionsResponseWriter buffers the proxied upstream response. Decisions
// is a synchronous JSON exchange and the gateway must inspect the whole
// response (status, decodability, protocol-required fields) before anything
// reaches the client, so the writer buffers and Finalize renders: valid 2xx
// bodies are forwarded byte-for-byte, upstream failures and malformed
// successes are rendered in the gateway's error envelope.
type decisionsResponseWriter struct {
	ginWriter gin.ResponseWriter

	header        http.Header
	statusCode    int
	headerWritten bool
	body          bytes.Buffer

	response  *types.DecisionResponse
	succeeded bool
	err       error
	errType   string
}

func newDecisionsResponseWriter(w gin.ResponseWriter) *decisionsResponseWriter {
	return &decisionsResponseWriter{
		ginWriter: w,
		header:    make(http.Header),
	}
}

// Header returns the buffered header map so proxy-set headers do not leak to
// the client before Finalize has validated the response.
func (w *decisionsResponseWriter) Header() http.Header {
	return w.header
}

func (w *decisionsResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *decisionsResponseWriter) WriteHeader(code int) {
	// 1xx informational responses (e.g. 103 Early Hints, which the reverse
	// proxy forwards through Got1xxResponse -> WriteHeader) are hints, not
	// the final response: they must not lock the recorded status or the
	// following 200 would be discarded and Finalize would render a 502.
	// The decisions protocol has no protocol upgrades, so every 1xx is
	// non-terminal here.
	if code >= 100 && code < 200 {
		return
	}
	if w.headerWritten {
		return
	}
	w.statusCode = code
	w.headerWritten = true
}

// Flush is a no-op: the protocol has no streaming variant and nothing may be
// forwarded before Finalize commits.
func (w *decisionsResponseWriter) Flush() {
}

func (w *decisionsResponseWriter) StatusCode() int {
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

// Succeeded reports whether the upstream exchange was forwarded as a valid
// success response.
func (w *decisionsResponseWriter) Succeeded() bool {
	return w.succeeded
}

// Response returns the parsed view of a validated success response.
func (w *decisionsResponseWriter) Response() *types.DecisionResponse {
	return w.response
}

// Err exposes the finalized outcome for trace and metrics error recording.
func (w *decisionsResponseWriter) Err() error {
	return w.err
}

// ErrorType is the low-cardinality error type recorded on the metrics.
func (w *decisionsResponseWriter) ErrorType() string {
	return w.errType
}

// Usage returns the token usage extracted from a validated success response.
func (w *decisionsResponseWriter) Usage() *tokenUsage {
	return extractUsage(w.response)
}

// extractUsage converts the parsed upstream usage into the gateway token
// usage.  A missing usage block is tolerated here (Finalize rejects 2xx
// responses without one before reaching success): usage-dependent
// post-processing degrades to zero.
func extractUsage(resp *types.DecisionResponse) *tokenUsage {
	usage := &tokenUsage{}
	if resp == nil || resp.Usage == nil {
		return usage
	}
	usage.PromptTokens = resp.Usage.InputTokens
	usage.CompletionTokens = resp.Usage.OutputTokens
	usage.TotalTokens = resp.Usage.EffectiveTotalTokens()
	usage.CachedPromptTokens = resp.Usage.InputTokensDetails.CachedTokens
	usage.CacheCreationPromptTokens = resp.Usage.InputTokensDetails.CacheWriteTokens
	usage.ReasoningTokens = resp.Usage.OutputTokensDetails.ReasoningTokens
	if resp.Usage.ComputeUnits != nil {
		usage.ComputeUnits = *resp.Usage.ComputeUnits
	}
	return usage
}

// Finalize validates the buffered response and renders it to the client.
// Transport failures surface through the proxy's error handler as an empty
// bad-gateway status; a client disconnect surfaces as 499.
func (w *decisionsResponseWriter) Finalize() {
	switch {
	case w.statusCode == 0:
		// The proxy failed before producing any response (unreachable
		// upstream, request build failure).
		w.renderError(http.StatusBadGateway, ErrTypeAPI, "failed to reach the upstream endpoint")
	case w.statusCode == statusClientClosedRequest:
		// The client went away mid-call: propagate the proxy's marker
		// status unchanged instead of rewriting it — there is no client
		// left to read a rendered error.  The outcome is still recorded
		// (empty error type: a disconnect is not an upstream error class)
		// so the trace span is ended and metrics see the outcome.
		w.succeeded = false
		w.err = errors.New("client disconnected")
		w.errType = ""
		w.forwardRaw()
	case w.statusCode >= 400:
		status, errType := mapUpstreamStatus(w.statusCode)
		w.renderError(status, errType, decisioncomp.ExtractUpstreamErrorMessage(w.body.Bytes()))
	case w.statusCode >= 200 && w.statusCode < 300:
		w.finalizeSuccess()
	default:
		// Informational / redirect statuses are not part of the protocol.
		w.renderError(http.StatusBadGateway, ErrTypeAPI, "upstream returned an unexpected response status")
	}
}

// finalizeSuccess parses and validates a 2xx body, then forwards the raw
// upstream bytes unchanged.
func (w *decisionsResponseWriter) finalizeSuccess() {
	var resp types.DecisionResponse
	if err := json.Unmarshal(w.body.Bytes(), &resp); err != nil {
		w.renderError(http.StatusBadGateway, ErrTypeAPI, "upstream returned a non-decodable response")
		return
	}
	if err := decisioncomp.ValidateSuccessResponse(&resp); err != nil {
		w.renderError(http.StatusBadGateway, ErrTypeAPI, err.Error())
		return
	}
	w.response = &resp
	w.succeeded = true
	w.forwardRaw()
}

// forwardRaw copies the buffered status, headers, and body to the gin
// writer.  The body is forwarded byte-for-byte (no transformation), so the
// upstream's headers — Content-Type, Content-Encoding, vendor metadata — are
// preserved as-is; only Content-Length is recomputed from the written bytes
// (equal to the upstream value because the body is unchanged, and correct if
// the proxy decompressed it).  Content-Type falls back to application/json
// only when the upstream provided none.
func (w *decisionsResponseWriter) forwardRaw() {
	hdr := w.ginWriter.Header()
	for k, vs := range w.header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vs {
			hdr.Add(k, v)
		}
	}
	if hdr.Get("Content-Type") == "" {
		hdr.Set("Content-Type", "application/json")
	}
	hdr.Set("Content-Length", strconv.Itoa(w.body.Len()))
	w.ginWriter.WriteHeader(w.statusCode)
	_, _ = w.ginWriter.Write(w.body.Bytes())
}

// renderError writes a gateway error envelope, resetting proxy-set headers
// that would conflict with the rendered body.
func (w *decisionsResponseWriter) renderError(status int, errType, message string) {
	w.succeeded = false
	w.err = errors.New(message)
	w.errType = errType

	hdr := w.ginWriter.Header()
	hdr.Set("Content-Type", "application/json")
	hdr.Del("Content-Length")
	hdr.Del("Content-Encoding")
	hdr.Del("Transfer-Encoding")
	w.ginWriter.WriteHeader(status)
	_ = json.NewEncoder(w.ginWriter).Encode(gin.H{"error": types.Error{
		Code:    errType,
		Message: message,
		Type:    errType,
	}})
}

// statusClientClosedRequest mirrors the proxy's marker for a client that
// disconnected mid-call (commonutils.StatusClientClosedRequest, 499).
const statusClientClosedRequest = 499
