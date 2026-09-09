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

// DefaultProtocolCapabilities returns the built-in capability profile for each
// protocol.  These defaults can be overridden at runtime by upstream metadata
// (see handler/protocol/adapt.go).
var DefaultProtocolCapabilities = map[Protocol]ProtocolCapability{
ProtocolChat: {
			Protocol:         ProtocolChat,
			Streaming:        true,
			Tools:            true,
			Vision:           true,
			Thinking:         true, // support via reasoning_effort translation
			PromptCaching:    false,
			StructuredOutput: true,
		},
	ProtocolResponses: {
		Protocol:         ProtocolResponses,
		Streaming:        true,
		Tools:            true,
		Vision:           true,
		Thinking:         true,
		PromptCaching:    false,
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
