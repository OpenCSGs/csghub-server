package anthropic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func decodeMessagesRequest(t *testing.T, extra string) *types.AnthropicMessagesRequest {
	t.Helper()
	req := &types.AnthropicMessagesRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}]`+extra+`}`), req))
	return req
}

func TestMessagesAdaptersApplyOutputConfigEffort(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		want  string // "" means no effort is sent
	}{
		{"adaptive low", `,"thinking":{"type":"adaptive"},"output_config":{"effort":"low"}`, "low"},
		{"adaptive high", `,"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}`, "high"},
		{"adaptive max", `,"thinking":{"type":"adaptive"},"output_config":{"effort":"max"}`, "high"},
		{"adaptive xhigh", `,"thinking":{"type":"adaptive"},"output_config":{"effort":"xhigh"}`, "high"},
		{"effort wins over budget", `,"thinking":{"type":"enabled","budget_tokens":1024},"output_config":{"effort":"high"}`, "high"},
		{"unknown effort keeps default", `,"thinking":{"type":"adaptive"},"output_config":{"effort":"bogus"}`, "medium"},
		{"thinking disabled", `,"thinking":{"type":"disabled"},"output_config":{"effort":"high"}`, ""},
		{"no thinking", `,"output_config":{"effort":"high"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := decodeMessagesRequest(t, tt.extra)

			chatReq, err := messagesToChatRequest(req, "upstream-model", true)
			require.NoError(t, err)
			respReq, err := messagesToResponsesRequest(req, "upstream-model")
			require.NoError(t, err)

			if tt.want == "" {
				assert.NotContains(t, chatReq, "reasoning_effort")
				assert.Empty(t, respReq.Reasoning)
				return
			}
			assert.Equal(t, tt.want, chatReq["reasoning_effort"])
			assert.JSONEq(t, `{"effort":"`+tt.want+`"}`, string(respReq.Reasoning))
		})
	}
}

func TestMessagesAdaptersForwardOutputConfigFormat(t *testing.T) {
	req := decodeMessagesRequest(t, `,"output_config":{"format":{"type":"json_schema","schema":{"type":"object","properties":{"a":{"type":"string"}}}}}`)
	schema := `{"type":"object","properties":{"a":{"type":"string"}}}`

	chatReq, err := messagesToChatRequest(req, "upstream-model", false)
	require.NoError(t, err)
	chatFormat, err := json.Marshal(chatReq["response_format"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"json_schema","json_schema":{"name":"response","schema":`+schema+`}}`, string(chatFormat))

	respReq, err := messagesToResponsesRequest(req, "upstream-model")
	require.NoError(t, err)
	assert.JSONEq(t, `{"format":{"type":"json_schema","name":"response","schema":`+schema+`,"strict":false}}`, string(respReq.Text))
}

// An upstream may complete normally without honouring the schema; that reply
// must not reach the client as a successful message.
func TestMessagesAdaptersRejectReplyBreakingOutputSchema(t *testing.T) {
	req := decodeMessagesRequest(t, `,"output_config":{"format":{"type":"json_schema","schema":{"type":"object","properties":{"summary":{"type":"string"},"note":{"type":"string"}},"required":["summary"]}}}`)
	schema, err := newOutputSchema(req)
	require.NoError(t, err)

	chatBody := func(text string) string {
		return `{"id":"c","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":` + jsonString(text) + `},"finish_reason":"stop"}]}`
	}
	responsesBody := func(text string) string {
		return `{"id":"r","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":` + jsonString(text) + `}]}]}`
	}
	chatChunk := func(text string) string {
		return `data: {"id":"c","choices":[{"index":0,"delta":{"content":` + jsonString(text) + `},"finish_reason":"stop"}]}` + "\n\n"
	}
	responsesEvents := func(text string) string {
		return "event: response.created\ndata: " + `{"type":"response.created","response":{"id":"r"}}` + "\n\n" +
			"event: response.output_text.delta\ndata: " + `{"type":"response.output_text.delta","delta":` + jsonString(text) + `}` + "\n\n" +
			"event: response.completed\ndata: " + `{"type":"response.completed","response":{"id":"r","status":"completed"}}` + "\n\n"
	}
	adapters := []struct {
		name   string
		stream bool
		body   func(string) string
	}{
		{"chat", false, chatBody},
		{"responses", false, responsesBody},
		{"chat stream", true, chatChunk},
		{"responses stream", true, responsesEvents},
	}
	for _, a := range adapters {
		for text, valid := range map[string]bool{`{"summary":"ok"}`: true, `{}`: false, `not json`: false} {
			t.Run(a.name+" "+text, func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				var w interface {
					Write([]byte) (int, error)
					Finalize() error
				}
				if strings.HasPrefix(a.name, "chat") {
					cw := newToChatResponseWriter(c.Writer, a.stream, "m", "m", nil)
					cw.schema, w = schema, cw
				} else {
					rw := newToResponsesResponseWriter(c.Writer, a.stream, "m", "m", nil)
					rw.schema, w = schema, rw
				}
				_, err := w.Write([]byte(a.body(text)))
				require.NoError(t, err)
				require.NoError(t, w.Finalize())

				assert.Equal(t, !valid, strings.Contains(rec.Body.String(), outputSchemaMismatch), rec.Body.String())
				if a.stream {
					assert.Equal(t, valid, strings.Contains(rec.Body.String(), "message_stop"), rec.Body.String())
				} else if !valid {
					assert.Equal(t, http.StatusBadGateway, rec.Code)
				}
			})
		}
	}
}

func TestMessagesAdaptersUseStrictModeForStrictSchemas(t *testing.T) {
	req := decodeMessagesRequest(t, `,"output_config":{"format":{"type":"json_schema","schema":{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}}}`)

	chatReq, err := messagesToChatRequest(req, "m", false)
	require.NoError(t, err)
	body, err := json.Marshal(chatReq["response_format"])
	require.NoError(t, err)
	assert.Contains(t, string(body), `"strict":true`)

	respReq, err := messagesToResponsesRequest(req, "m")
	require.NoError(t, err)
	assert.Contains(t, string(respReq.Text), `"strict":true`)
}
