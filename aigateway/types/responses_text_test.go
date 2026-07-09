package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesPromptText(t *testing.T) {
	cases := []struct {
		name string
		req  *ResponsesRequest
		want string
	}{
		{
			name: "nil request returns empty",
			req:  nil,
			want: "",
		},
		{
			name: "empty request returns empty",
			req:  &ResponsesRequest{},
			want: "",
		},
		{
			name: "instructions only",
			req: &ResponsesRequest{
				Instructions: json.RawMessage(`"be concise"`),
			},
			want: "be concise\n",
		},
		{
			name: "non-string instructions ignored",
			req: &ResponsesRequest{
				Instructions: json.RawMessage(`{"text":"do not stringify me"}`),
				Input:        json.RawMessage(`"hello"`),
			},
			want: "hello",
		},
		{
			name: "string input only",
			req: &ResponsesRequest{
				Input: json.RawMessage(`"hello"`),
			},
			want: "hello",
		},
		{
			name: "instructions plus string input",
			req: &ResponsesRequest{
				Instructions: json.RawMessage(`"be concise"`),
				Input:        json.RawMessage(`"hello"`),
			},
			want: "be concise\nhello",
		},
		{
			name: "message array",
			req: &ResponsesRequest{
				Input: json.RawMessage(`[{"type":"message","role":"user","content":"hi"}]`),
			},
			want: "hi\n",
		},
		{
			name: "function_call item",
			req: &ResponsesRequest{
				Input: json.RawMessage(`[{"type":"function_call","name":"get_weather","arguments":"{\"city\":\"sf\"}"}]`),
			},
			want: "get_weather{\"city\":\"sf\"}\n",
		},
		{
			name: "function_call_output item",
			req: &ResponsesRequest{
				Input: json.RawMessage(`[{"type":"function_call_output","output":"sunny"}]`),
			},
			want: "sunny\n",
		},
		{
			name: "unsupported shape falls back to raw string",
			req: &ResponsesRequest{
				Input: json.RawMessage(`12345`),
			},
			want: "12345",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResponsesPromptText(tc.req)
			require.Equal(t, tc.want, got)
		})
	}
}
