package types

// Protocol identifies an LLM inference protocol supported by the AIGateway.
type Protocol string

const (
	// ProtocolChat is the OpenAI Chat Completions API (/v1/chat/completions).
	ProtocolChat Protocol = "chat"
	// ProtocolResponses is the OpenAI Responses API (/v1/responses).
	ProtocolResponses Protocol = "responses"
	// ProtocolMessages is the Anthropic Messages API (/v1/messages).
	ProtocolMessages Protocol = "messages"
)

// ProtocolCapability describes the feature set of a protocol implementation.
// It is used by the routing layer to decide whether a request can be served
// natively (protocol match) or via an adapter, and to reject requests that
// require unsupported capabilities rather than silently dropping parameters.
type ProtocolCapability struct {
	Protocol         Protocol
	Streaming        bool
	Tools            bool
	Vision           bool
	Thinking         bool // extended thinking / reasoning
	PromptCaching    bool // Anthropic cache_control or equivalent
	StructuredOutput bool // JSON schema / response_format
}

// DefaultProtocolCapabilities returns the built-in capability profile for
// each protocol. These are protocol-level defaults: the planner applies them
// by upstream protocol (see handler/plan/planner.go) and there is currently
// no per-upstream metadata override on top of them.
var DefaultProtocolCapabilities = map[Protocol]ProtocolCapability{
	ProtocolChat: {
		Protocol:  ProtocolChat,
		Streaming: true,
		Tools:     true,
		Vision:    true,
		Thinking:  true, // support via reasoning_effort translation
		// Chat upstreams cache prompts automatically (DeepSeek reports
		// prompt_cache_hit_tokens, OpenAI caches implicitly), so the
		// Anthropic cache_control hint counts as satisfied: the
		// messages→chat conversion drops the markers and the upstream
		// caches implicitly.
		PromptCaching:    true,
		StructuredOutput: true,
	},
	ProtocolResponses: {
		Protocol:  ProtocolResponses,
		Streaming: true,
		Tools:     true,
		Vision:    true,
		Thinking:  true,
		// The Responses upstream caches prompts automatically as well; the
		// messages→responses conversion drops cache_control markers.
		PromptCaching:    true,
		StructuredOutput: true,
	},
	ProtocolMessages: {
		Protocol:         ProtocolMessages,
		Streaming:        true,
		Tools:            true,
		Vision:           true,
		Thinking:         true,
		PromptCaching:    true,
		StructuredOutput: false,
	},
}

// CapabilityFor returns the default capability for the given protocol.
// If the protocol is unknown, a zero-value capability is returned.
func CapabilityFor(p Protocol) ProtocolCapability {
	cap, ok := DefaultProtocolCapabilities[p]
	if !ok {
		return ProtocolCapability{Protocol: p}
	}
	return cap
}
