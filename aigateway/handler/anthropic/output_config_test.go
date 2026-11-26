package anthropic

import (
	"encoding/json"
	"testing"

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
