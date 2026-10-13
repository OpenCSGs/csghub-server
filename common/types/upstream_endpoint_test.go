package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpstreamConfig_SDKBaseURL(t *testing.T) {
	// Declared chat on a base URL: the openai SDK base carries /v1 and
	// the SDK appends the chat completions resource path.
	chatBase := UpstreamConfig{
		URL:                "https://upstream.example.com",
		SupportedEndpoints: map[ApiFormat]string{ApiFormatChat: ""},
	}
	assert.Equal(t, "https://upstream.example.com/v1", chatBase.SDKBaseURL(ApiFormatChat))

	// Declared chat inferred from a legacy full URL: the resource tail is
	// stripped, keeping the /v1 segment.
	chatFull := UpstreamConfig{URL: "https://upstream.example.com/v1/chat/completions"}
	assert.Equal(t, "https://upstream.example.com/v1", chatFull.SDKBaseURL(ApiFormatChat))

	// Declared responses on a full URL: only the resource tail is
	// stripped from the path.
	responsesFull := UpstreamConfig{
		URL:                "https://upstream.example.com/v1/responses",
		IsFullURL:          true,
		SupportedEndpoints: map[ApiFormat]string{ApiFormatResponses: ""},
	}
	assert.Equal(t, "https://upstream.example.com/v1", responsesFull.SDKBaseURL(ApiFormatResponses))

	// Declared messages on a base URL: the anthropic SDK appends
	// v1/messages itself, so the base must not carry /v1.
	messagesBase := UpstreamConfig{
		URL:                "https://api.anthropic.example.com/v1",
		SupportedEndpoints: map[ApiFormat]string{ApiFormatMessages: ""},
	}
	assert.Equal(t, "https://api.anthropic.example.com", messagesBase.SDKBaseURL(ApiFormatMessages))

	// A per-protocol override acts as the SDK base; a non-standard tail
	// is kept with a warning.
	chatOverride := UpstreamConfig{
		URL:                "https://upstream.example.com",
		SupportedEndpoints: map[ApiFormat]string{ApiFormatChat: "https://override.example.com/chat"},
	}
	assert.Equal(t, "https://override.example.com/chat", chatOverride.SDKBaseURL(ApiFormatChat))

	// Formats without an SDK convention fall back to the URL as-is.
	systemOne := UpstreamConfig{URL: "https://openrouter.example.com/api/v1/systemone"}
	assert.Equal(t, "https://openrouter.example.com/api/v1/systemone", systemOne.SDKBaseURL(ApiFormatSystemOne))
}

func TestUpstreamConfig_SDKBaseURLPreservesQuery(t *testing.T) {
	// Query-bearing endpoints (e.g. Azure-style api-version) keep their
	// query on the derived SDK base while the protocol path is adjusted
	// on the URL path only.
	messagesFull := UpstreamConfig{
		URL:                "https://host/v1/messages?api-version=x",
		IsFullURL:          true,
		SupportedEndpoints: map[ApiFormat]string{ApiFormatMessages: ""},
	}
	assert.Equal(t, "https://host?api-version=x", messagesFull.SDKBaseURL(ApiFormatMessages))

	responsesBase := UpstreamConfig{
		URL:                "https://host?api-version=x",
		SupportedEndpoints: map[ApiFormat]string{ApiFormatResponses: ""},
	}
	assert.Equal(t, "https://host/v1?api-version=x", responsesBase.SDKBaseURL(ApiFormatResponses))
}

func TestParseAuthHeaders(t *testing.T) {
	headers, err := ParseAuthHeaders("")
	assert.NoError(t, err)
	assert.Nil(t, headers)

	headers, err = ParseAuthHeaders("Bearer tok")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Authorization": "Bearer tok"}, headers)

	headers, err = ParseAuthHeaders(`{"x-api-key":"k","anthropic-version":"2023-06-01"}`)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"x-api-key": "k", "anthropic-version": "2023-06-01"}, headers)

	// Neither JSON nor a Bearer token: an error, so callers can log and
	// proceed without resolved headers.
	headers, err = ParseAuthHeaders("not-json-not-bearer")
	assert.Error(t, err)
	assert.Nil(t, headers)
}

func TestUpstreamConfig_InternalHost(t *testing.T) {
	assert.Empty(t, UpstreamConfig{}.InternalHost())
	uc := UpstreamConfig{Metadata: &UpstreamMetadata{
		InternalModelInfo: &InternalModelInfo{Host: "deploy.example.com"},
	}}
	assert.Equal(t, "deploy.example.com", uc.InternalHost())
}
