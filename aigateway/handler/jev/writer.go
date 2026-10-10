package jev

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	jevcomp "opencsg.com/csghub-server/aigateway/component/jev"
	"opencsg.com/csghub-server/aigateway/types"
)

// systemoneResponseWriter buffers the proxied upstream response.  System One
// is a synchronous JSON exchange and the gateway must inspect the whole
// response (status, decodability, protocol-required fields) before anything
// reaches the client, so the writer buffers and Finalize renders: valid 2xx
// bodies are forwarded byte-for-byte, upstream failures and malformed
// successes are rendered in the gateway's error envelope.
type systemoneResponseWriter struct {
	ginWriter gin.ResponseWriter

	header        http.Header
	statusCode    int
	headerWritten bool
	body          bytes.Buffer

	response  *types.JevResponse
	succeeded bool
	err       error
	errType   string
}

func newSystemOneResponseWriter(w gin.ResponseWriter) *systemoneResponseWriter {
	return &systemoneResponseWriter{
		ginWriter: w,
		header:    make(http.Header),
	}
}

// Header returns the buffered header map so proxy-set headers do not leak to
// the client before Finalize has validated the response.
func (w *systemoneResponseWriter) Header() http.Header {
	return w.header
}

func (w *systemoneResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *systemoneResponseWriter) WriteHeader(code int) {
	if w.headerWritten {
		return
	}
	w.statusCode = code
	w.headerWritten = true
}

// Flush is a no-op: the protocol has no streaming variant and nothing may be
// forwarded before Finalize commits.
func (w *systemoneResponseWriter) Flush() {
}

func (w *systemoneResponseWriter) StatusCode() int {
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

// Succeeded reports whether the upstream exchange was forwarded as a valid
// success response.
func (w *systemoneResponseWriter) Succeeded() bool {
	return w.succeeded
}

// Response returns the parsed view of a validated success response.
func (w *systemoneResponseWriter) Response() *types.JevResponse {
	return w.response
}

// Err exposes the finalized outcome for trace and metrics error recording.
func (w *systemoneResponseWriter) Err() error {
	return w.err
}

// ErrorType is the low-cardinality error type recorded on the metrics.
func (w *systemoneResponseWriter) ErrorType() string {
	return w.errType
}

// Usage returns the token usage extracted from a validated success response.
func (w *systemoneResponseWriter) Usage() *tokenUsage {
	return extractUsage(w.response)
}

// extractUsage converts the parsed upstream usage into the gateway token
// usage.  A missing usage block is tolerated here (Finalize rejects 2xx
// responses without one before reaching success): usage-dependent
// post-processing degrades to zero.
func extractUsage(resp *types.JevResponse) *tokenUsage {
	usage := &tokenUsage{}
	if resp == nil || resp.Usage == nil {
		return usage
	}
	usage.PromptTokens = resp.Usage.InputTokens
	usage.CompletionTokens = resp.Usage.OutputTokens
	usage.TotalTokens = resp.Usage.TotalTokens()
	return usage
}

// Finalize validates the buffered response and renders it to the client.
// Transport failures surface through the proxy's error handler as an empty
// bad-gateway status; a client disconnect surfaces as 499.
func (w *systemoneResponseWriter) Finalize() {
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
		w.renderError(status, errType, jevcomp.ExtractUpstreamErrorMessage(w.body.Bytes()))
	case w.statusCode >= 200 && w.statusCode < 300:
		w.finalizeSuccess()
	default:
		// Informational / redirect statuses are not part of the protocol.
		w.renderError(http.StatusBadGateway, ErrTypeAPI, "upstream returned an unexpected response status")
	}
}

// finalizeSuccess parses and validates a 2xx body, then forwards the raw
// upstream bytes unchanged.
func (w *systemoneResponseWriter) finalizeSuccess() {
	var resp types.JevResponse
	if err := json.Unmarshal(w.body.Bytes(), &resp); err != nil {
		w.renderError(http.StatusBadGateway, ErrTypeAPI, "upstream returned a non-decodable response")
		return
	}
	if err := jevcomp.ValidateSuccessResponse(&resp); err != nil {
		w.renderError(http.StatusBadGateway, ErrTypeAPI, err.Error())
		return
	}
	w.response = &resp
	w.succeeded = true
	w.forwardRaw()
}

// forwardRaw copies the buffered status, headers, and body to the gin
// writer.  Content-Length and Content-Encoding are dropped: the body was
// buffered in full and may have been decompressed by the proxy.
func (w *systemoneResponseWriter) forwardRaw() {
	hdr := w.ginWriter.Header()
	for k, vs := range w.header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Encoding") {
			continue
		}
		for _, v := range vs {
			hdr.Add(k, v)
		}
	}
	hdr.Set("Content-Type", "application/json")
	hdr.Del("Content-Length")
	hdr.Del("Content-Encoding")
	w.ginWriter.WriteHeader(w.statusCode)
	_, _ = w.ginWriter.Write(w.body.Bytes())
}

// renderError writes a gateway error envelope, resetting proxy-set headers
// that would conflict with the rendered body.
func (w *systemoneResponseWriter) renderError(status int, errType, message string) {
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
