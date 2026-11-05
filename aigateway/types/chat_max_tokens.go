package types

import (
	"encoding/json"
	"strings"
)

// modelMaxOutputTokens caps max_tokens for upstreams that reject a request
// outright when the caller asks for more output than the model can produce,
// rather than generating up to their own limit.
//
// It is a backstop for known offenders, not a model catalogue: a model that
// is absent is forwarded unchanged, which is the behaviour every model had
// before this table existed. Keys are the upstream model name, lowercased.
//
// qwen3.6-plus is served through Alibaba's DashScope compatible-mode
// endpoint, which validates the field strictly and answers
// `Range of max_tokens should be [1, 65536]` with HTTP 400. Agent clients
// routinely send their context window as max_tokens, so without the cap a
// request that the model could serve is refused before it starts.
var modelMaxOutputTokens = map[string]int{
	"qwen3.6-plus": 65536,
}

// CapMaxTokens lowers an oversized max_tokens in an outgoing chat body to
// what the model accepts. The body is returned untouched for a model with
// no known cap, which is every model but the few listed above, so the cost
// for an ordinary request is one map lookup and no parsing at all.
//
// Only an oversized value is changed, and only downwards: the cap is what
// the model can produce, so the capped request yields the same output the
// caller would have received had the upstream accepted the original value.
func CapMaxTokens(body []byte, modelName string) []byte {
	limit := modelMaxOutputTokens[strings.ToLower(strings.TrimSpace(modelName))]
	if limit <= 0 {
		return body
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	raw, ok := payload["max_tokens"]
	if !ok {
		return body
	}
	var requested int
	if err := json.Unmarshal(raw, &requested); err != nil || requested <= limit {
		return body
	}
	capped, err := json.Marshal(limit)
	if err != nil {
		return body
	}
	payload["max_tokens"] = capped
	patched, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return patched
}
