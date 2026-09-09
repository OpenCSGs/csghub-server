package sample

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointMatchesRoute(t *testing.T) {
	tests := []struct {
		name, endpoint, route string
		want                  bool
	}{
		{name: "direct", endpoint: "https://api.example.com/chat/completions", route: "/chat/completions", want: true},
		{name: "versioned", endpoint: "https://api.example.com/v1/chat/completions", route: "/chat/completions", want: true},
		{name: "prefixed trailing slash", endpoint: "https://api.example.com/compatible/v1/responses/", route: "/responses", want: true},
		{name: "speech does not match batch", endpoint: "https://api.example.com/v1/audio/speech/batch", route: "/audio/speech", want: false},
		{name: "messages direct", endpoint: "https://api.anthropic.com/v1/messages", route: "/v1/messages", want: true},
		{name: "messages prefixed", endpoint: "https://gateway.example.com/anthropic/v1/messages", route: "/v1/messages", want: true},
		{name: "messages without version", endpoint: "https://api.example.com/messages", route: "/v1/messages", want: false},
		{name: "messages count tokens", endpoint: "https://api.example.com/v1/messages/count_tokens", route: "/v1/messages", want: false},
		{name: "bedrock invoke", endpoint: "https://bedrock.example.com/model/claude/invoke", route: "/v1/messages", want: false},
		{name: "bare URL", endpoint: "api.example.com/v1/messages", route: "/v1/messages", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, endpointMatchesRoute(test.endpoint, test.route))
		})
	}
}

func TestEndpointBaseURL(t *testing.T) {
	baseURL, err := endpointBaseURL("https://dashscope.example/compatible-mode/v1/responses?key=value#fragment", "/responses")
	require.NoError(t, err)
	require.Equal(t, "https://dashscope.example/compatible-mode/v1", baseURL)
	baseURL, err = endpointBaseURL("https://api.example.com/chat/completions/", "/chat/completions")
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com", baseURL)
	_, err = endpointBaseURL("https://api.example.com/v1/embeddings", "/responses")
	require.Error(t, err)
}
