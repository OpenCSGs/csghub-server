package llm

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

// errInvalidSummary is returned when an upstream response carries no usable
// text content, matching the legacy chat client's error.
var errInvalidSummary = errors.New("summary of conversation is invalid")

// systemRole is the chat message role carrying system prompts.
const systemRole = "system"

// assistantRole is the chat message role for generated content.
const assistantRole = "assistant"

// defaultAnthropicMaxTokens fills the max_tokens field, which the Anthropic
// Messages API requires but internal callers usually leave unset.
const defaultAnthropicMaxTokens = 4096

// splitSystemMessages separates system-role messages from the rest. The
// system texts are joined because the responses and messages protocols carry
// a single system prompt per request.
func splitSystemMessages(msgs []types.LLMMessage) (string, []types.LLMMessage) {
	var parts []string
	rest := make([]types.LLMMessage, 0, len(msgs))
	for _, m := range msgs {
		if strings.EqualFold(m.Role, systemRole) {
			if strings.TrimSpace(m.Content) != "" {
				parts = append(parts, m.Content)
			}
			continue
		}
		rest = append(rest, m)
	}
	return strings.Join(parts, "\n\n"), rest
}

// emitChunk forwards one text delta to the stream channel as an openai chat
// chunk JSON line (types.LLMResponse) — the shape the streaming consumers
// (e.g. the prompt SSE handler) already parse.
func emitChunk(ctx context.Context, out chan<- string, content string) {
	sendLLMResponseChunk(ctx, out, types.LLMResponse{
		Choices: []types.LLMChoice{{
			Delta: types.LLMMessage{Role: assistantRole, Content: content},
		}},
	})
}

// streamErrorPayload mirrors the OpenAI mid-stream error convention
// ({"error":{...}}), so SSE consumers and OpenAI-compatible frontends can
// tell a broken stream apart from a clean end.
type streamErrorPayload struct {
	Error streamErrorBody `json:"error"`
}

type streamErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// emitStreamError forwards one mid-stream failure to the stream channel as an
// openai error JSON line. The send aborts on ctx cancellation (a consumer
// that walked away gets nothing anyway).
func emitStreamError(ctx context.Context, out chan<- string, err error) {
	data, mErr := json.Marshal(streamErrorPayload{
		Error: streamErrorBody{Message: err.Error(), Type: "upstream_stream_error"},
	})
	if mErr != nil {
		slog.Error("marshal llm stream error", slog.Any("error", mErr))
		return
	}
	select {
	case out <- string(data):
	case <-ctx.Done():
	}
}

// sendLLMResponseChunk marshals one openai chat chunk (types.LLMResponse) and
// forwards it to the stream channel. The send aborts on ctx cancellation so a
// consumer that stops reading cannot leak this goroutine.
func sendLLMResponseChunk(ctx context.Context, out chan<- string, chunk types.LLMResponse) {
	data, err := json.Marshal(chunk)
	if err != nil {
		slog.Error("marshal llm stream chunk", slog.Any("error", err))
		return
	}
	select {
	case out <- string(data):
	case <-ctx.Done():
	}
}

// hostOverrideDoer wraps an HTTP doer to set the virtual-host routing header
// from the upstream metadata, mirroring the gateway's host override.
type hostOverrideDoer struct {
	inner interface {
		Do(req *http.Request) (*http.Response, error)
	}
	host string
}

func (d hostOverrideDoer) Do(req *http.Request) (*http.Response, error) {
	req.Host = d.host
	return d.inner.Do(req)
}

// envDerivedHeaders are the headers the official SDKs copy from ambient
// environment variables into every request: openai-go reads
// OPENAI_API_KEY / OPENAI_ORG_ID / OPENAI_PROJECT_ID (Authorization,
// OpenAI-Organization, OpenAI-Project) and anthropic-sdk-go reads
// ANTHROPIC_API_KEY / ANTHROPIC_AUTH_TOKEN (X-Api-Key, Authorization).
var envDerivedHeaders = []string{
	"Authorization",
	"X-Api-Key",
	"OpenAI-Organization",
	"OpenAI-Project",
}

// stripEnvAuthDoer removes the SDK's env-derived headers from requests when
// the upstream config does not explicitly set them, so ambient platform
// credentials and organization/project identifiers are never sent to a
// configured upstream. Headers the config explicitly sets are always kept.
type stripEnvAuthDoer struct {
	inner interface {
		Do(req *http.Request) (*http.Response, error)
	}
	headers map[string]string
}

func (d stripEnvAuthDoer) Do(req *http.Request) (*http.Response, error) {
	for _, name := range envDerivedHeaders {
		if !headerSet(d.headers, name) {
			req.Header.Del(name)
		}
	}
	return d.inner.Do(req)
}

// headerSet reports whether the config header map sets the given header
// (case-insensitive).
func headerSet(headers map[string]string, name string) bool {
	for k := range headers {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// appendEndpointQueryOptions forwards the query parameters embedded in the
// upstream endpoint (e.g. Azure-style api-version) to the given sink. Both
// SDKs resolve the request path against the base URL and drop a query
// carried by the base itself, so the parameters must be set as explicit
// query options to land on the final URL exactly once.
func appendEndpointQueryOptions(endpoint string, add func(key, value string)) {
	u, err := url.Parse(endpoint)
	if err != nil || u.RawQuery == "" {
		return
	}
	for k, vs := range u.Query() {
		for _, v := range vs {
			add(k, v)
		}
	}
}
