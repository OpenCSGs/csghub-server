package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func maxTokensOf(t *testing.T, body []byte) (int, bool) {
	t.Helper()
	var payload struct {
		MaxTokens *int `json:"max_tokens"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	if payload.MaxTokens == nil {
		return 0, false
	}
	return *payload.MaxTokens, true
}

func TestCapMaxTokensLowersAnOversizedValue(t *testing.T) {
	// The client sent its context window rather than an output budget,
	// which this upstream rejects with HTTP 400 when forwarded.
	body := []byte(`{"model":"qwen3.6-plus","max_tokens":131072,"messages":[]}`)

	capped := CapMaxTokens(body, "qwen3.6-plus")

	maxTokens, ok := maxTokensOf(t, capped)
	require.True(t, ok, "max_tokens must still be present")
	assert.Equal(t, 65536, maxTokens)
}

func TestCapMaxTokensLeavesAValueWithinTheCap(t *testing.T) {
	body := []byte(`{"model":"qwen3.6-plus","max_tokens":4096,"messages":[]}`)

	capped := CapMaxTokens(body, "qwen3.6-plus")

	assert.Equal(t, string(body), string(capped), "a request within the cap must not be rewritten at all")
}

func TestCapMaxTokensLeavesAModelWithNoCapAlone(t *testing.T) {
	body := []byte(`{"model":"deepseek-v4-flash","max_tokens":131072,"messages":[]}`)

	capped := CapMaxTokens(body, "deepseek-v4-flash")

	assert.Equal(t, string(body), string(capped))
}

func TestCapMaxTokensLeavesABodyWithoutTheFieldAlone(t *testing.T) {
	body := []byte(`{"model":"qwen3.6-plus","messages":[]}`)

	capped := CapMaxTokens(body, "qwen3.6-plus")

	assert.Equal(t, string(body), string(capped))
	_, ok := maxTokensOf(t, capped)
	assert.False(t, ok, "an absent field must not be introduced")
}

func TestCapMaxTokensMatchesTheModelNameCaseInsensitively(t *testing.T) {
	body := []byte(`{"max_tokens":131072}`)

	capped := CapMaxTokens(body, "  Qwen3.6-Plus ")

	maxTokens, _ := maxTokensOf(t, capped)
	assert.Equal(t, 65536, maxTokens)
}

func TestCapMaxTokensLeavesAnUnreadableBodyAlone(t *testing.T) {
	body := []byte(`not json`)

	assert.Equal(t, string(body), string(CapMaxTokens(body, "qwen3.6-plus")))
}
