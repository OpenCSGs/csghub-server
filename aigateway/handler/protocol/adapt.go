// Package protocol provides the unified routing resolver that decides how a
// client request should be executed against an upstream endpoint.  The core
// concept is a protocol matrix: when the client protocol matches the upstream
// protocol the request is forwarded natively; otherwise an adapter is selected
// to translate between protocols.  If no adapter exists the request is rejected
// with a clear error rather than silently dropping parameters.
//
// Strategy: Native > Adapter > Reject.
package protocol

import (
	"fmt"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

// ExecutionMode describes how a request should be executed.
type ExecutionMode string

const (
	ModeNative   ExecutionMode = "native"
	ModeAdapter  ExecutionMode = "adapter"
	ModeDisabled ExecutionMode = "disabled"
)

// AdapterKind identifies which adapter is needed when Mode == adapter.
type AdapterKind string

const (
	AdapterNone                AdapterKind = ""
	AdapterResponsesToChat     AdapterKind = "responses_to_chat"
	AdapterMessagesToChat      AdapterKind = "messages_to_chat"
	AdapterMessagesToResponses AdapterKind = "messages_to_responses"
	// Future adapters (not yet implemented):
	//   AdapterChatToMessages    AdapterKind = "chat_to_messages"
	//   AdapterResponsesToMessages AdapterKind = "responses_to_messages"
	//   AdapterChatToResponses   AdapterKind = "chat_to_responses"
)

// RoutingDecision is the output of ResolveRouting.
type RoutingDecision struct {
	Mode             ExecutionMode
	AdapterKind      AdapterKind
	BackendURL       string       // the URL to send the upstream request to
	UpstreamProtocol types.Protocol
	Reason           string
}

// RoutingTarget carries the information needed to detect the upstream protocol
// and compute the backend URL.
type RoutingTarget struct {
	ModelID          string
	Target           string // upstream URL
	CSGHubHosted     bool
	RuntimeFramework string
	ImageID          string
	UpstreamMetadata map[string]any
}

// adapterMatrix defines which adapter to use for each (client, upstream) pair.
// A zero AdapterKind means no adapter is available.
var adapterMatrix = map[types.Protocol]map[types.Protocol]AdapterKind{
	types.ProtocolChat: {
		types.ProtocolResponses: AdapterNone,
		types.ProtocolMessages:  AdapterNone,
	},
	types.ProtocolResponses: {
		types.ProtocolChat:     AdapterResponsesToChat,
		types.ProtocolMessages: AdapterNone,
	},
	types.ProtocolMessages: {
		types.ProtocolChat:      AdapterMessagesToChat,
		types.ProtocolResponses: AdapterMessagesToResponses,
	},
}

// ResolveRouting determines how a client request should be routed to the
// upstream endpoint.  The decision follows the Native > Adapter > Reject
// strategy.
func ResolveRouting(clientProtocol types.Protocol, target RoutingTarget) (RoutingDecision, error) {
	if strings.TrimSpace(target.Target) == "" && !target.CSGHubHosted {
		return RoutingDecision{}, fmt.Errorf("upstream target is empty")
	}

	upstreamProtocol := DetectUpstreamProtocol(target)

	// When the upstream protocol cannot be determined from metadata or URL
	// (step 4 fallback in DetectUpstreamProtocol), and the model is not
	// CSGHub-hosted, prefer the client's own protocol so the request takes
	// the native passthrough path.  This avoids wrongly adapting a Messages
	// request to Chat when the upstream is actually Anthropic-native (e.g.
	// an external endpoint whose URL does not end in /v1/messages).
	// CSGHub-hosted models always use vLLM/SGLang which speak Chat, so they
	// keep the Chat default regardless of the client protocol.
	if upstreamProtocol == types.ProtocolChat && !target.CSGHubHosted &&
		clientProtocol != types.ProtocolChat &&
		!hasExplicitProtocolMetadata(target) {
		if _, inferred := inferProtocolFromURL(target.Target); !inferred {
			upstreamProtocol = clientProtocol
		}
	}

	if clientProtocol == upstreamProtocol {
		return RoutingDecision{
			Mode:             ModeNative,
			BackendURL:       adaptBackendURL(target, upstreamProtocol),
			UpstreamProtocol: upstreamProtocol,
			Reason:           "protocol_match",
		}, nil
	}

	adapterKind := pickAdapter(clientProtocol, upstreamProtocol)
	if adapterKind == AdapterNone {
		return RoutingDecision{
			Mode:             ModeDisabled,
			UpstreamProtocol: upstreamProtocol,
			Reason:           fmt.Sprintf("no_adapter_for_%s_to_%s", clientProtocol, upstreamProtocol),
		}, nil
	}

	backendURL := adaptBackendURL(target, upstreamProtocol)
	return RoutingDecision{
		Mode:             ModeAdapter,
		AdapterKind:      adapterKind,
		BackendURL:       backendURL,
		UpstreamProtocol: upstreamProtocol,
		Reason:           fmt.Sprintf("adapter:%s", adapterKind),
	}, nil
}

// DetectUpstreamProtocol determines the protocol of an upstream endpoint.
// Priority: explicit metadata declaration > URL path inference >
// CSGHub runtime defaults > fallback to Chat.
//
// Note: the Chat fallback for non-CSGHub-hosted external upstreams whose
// protocol cannot be inferred from the URL is overridden in ResolveRouting,
// which prefers the client protocol (native passthrough) in that case.
func DetectUpstreamProtocol(target RoutingTarget) types.Protocol {
	// 1. Explicit declaration in upstream metadata.
	if target.UpstreamMetadata != nil {
		if p, ok := target.UpstreamMetadata["protocol"].(string); ok {
			p = strings.TrimSpace(strings.ToLower(p))
			switch types.Protocol(p) {
			case types.ProtocolChat, types.ProtocolResponses, types.ProtocolMessages:
				return types.Protocol(p)
			}
		}
	}

	// 2. URL path inference.
	if proto, ok := inferProtocolFromURL(target.Target); ok {
		return proto
	}

	// 3. CSGHub-hosted models default to Chat (vLLM/SGLang).
	if target.CSGHubHosted {
		return types.ProtocolChat
	}

	// 4. Fallback: Chat is the most widely compatible protocol.
	return types.ProtocolChat
}

// hasExplicitProtocolMetadata returns true when the upstream metadata contains
// a valid "protocol" key.  This is used by ResolveRouting to distinguish an
// explicit Chat declaration (which must be respected) from the Chat fallback
// default (which can be overridden to prefer native passthrough).
func hasExplicitProtocolMetadata(target RoutingTarget) bool {
	if target.UpstreamMetadata == nil {
		return false
	}
	p, ok := target.UpstreamMetadata["protocol"].(string)
	if !ok {
		return false
	}
	p = strings.TrimSpace(strings.ToLower(p))
	switch types.Protocol(p) {
	case types.ProtocolChat, types.ProtocolResponses, types.ProtocolMessages:
		return true
	}
	return false
}

// inferProtocolFromURL inspects the URL path to guess the protocol.
func inferProtocolFromURL(rawURL string) (types.Protocol, bool) {
	if strings.TrimSpace(rawURL) == "" {
		return "", false
	}
	// url.Parse interprets a bare host:port/path as scheme=host, so
	// prepend a scheme when absent to ensure the path is parsed correctly.
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case pathEndsWithSegments(path, "messages"), pathEndsWithSegments(path, "anthropic"):
		return types.ProtocolMessages, true
	case pathEndsWithSegments(path, "responses"):
		return types.ProtocolResponses, true
	case pathEndsWithSegments(path, "chat", "completions"):
		return types.ProtocolChat, true
	}
	return "", false
}

// adaptBackendURL rewrites the upstream URL to point at the correct path for
// the detected upstream protocol.  For external upstreams the URL is already
// correct (it was configured with the right path).  For CSGHub-hosted models
// the URL may be a bare host:port, so we append the standard path.
func adaptBackendURL(target RoutingTarget, upstreamProtocol types.Protocol) string {
	// If the URL already contains a recognizable path, trust it.
	if _, ok := inferProtocolFromURL(target.Target); ok {
		return target.Target
	}

	// CSGHub-hosted: append the standard path for the upstream protocol.
	if target.CSGHubHosted {
		parsed, err := url.Parse(target.Target)
		if err != nil || parsed.Host == "" {
			return target.Target
		}
		switch upstreamProtocol {
		case types.ProtocolChat:
			return appendEndpointPath(parsed, "v1", "chat", "completions")
		case types.ProtocolResponses:
			return appendEndpointPath(parsed, "v1", "responses")
		case types.ProtocolMessages:
			return appendEndpointPath(parsed, "v1", "messages")
		}
	}

	return target.Target
}

func pickAdapter(clientProtocol, upstreamProtocol types.Protocol) AdapterKind {
	row, ok := adapterMatrix[clientProtocol]
	if !ok {
		return AdapterNone
	}
	kind, ok := row[upstreamProtocol]
	if !ok {
		return AdapterNone
	}
	return kind
}

func pathEndsWithSegments(path string, segments ...string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < len(segments) {
		return false
	}
	offset := len(parts) - len(segments)
	for idx, segment := range segments {
		if parts[offset+idx] != segment {
			return false
		}
	}
	return true
}

func appendEndpointPath(base *url.URL, segments ...string) string {
	u := *base
	// Avoid duplicating path segments that are already present in the base
	// path.  For example, if base.Path is "/v1" and segments are
	// ["v1", "chat", "completions"], the result should be
	// "/v1/chat/completions" not "/v1/v1/chat/completions".
	path := strings.TrimRight(u.Path, "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	trimmed := trimLeadingSegments(segments, pathParts)
	u.Path = commonutils.JoinURLPath(u.Path, trimmed...)
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// trimLeadingSegments removes leading segments from elems that already
// match the trailing segments of base.  This prevents path duplication
// when the base URL already contains a path prefix like /v1.
func trimLeadingSegments(elems []string, base []string) []string {
	if len(base) == 0 || len(elems) == 0 {
		return elems
	}
	// Find the longest matching suffix of base that is also a prefix of elems.
	// For example: base=["v1","api"], elems=["v1","chat","completions"]
	// → matching suffix of base is ["v1"], which is prefix of elems
	// → trim it, return ["chat","completions"]
	// But: base=["v1"], elems=["v1","chat","completions"]
	// → matching suffix of base is ["v1"] = prefix of elems
	// → trim it, return ["chat","completions"]
	// And: base=["v1"], elems=["v1","chat","completions"]
	// → elems[0] = base[0] → trim
	// → result: ["chat","completions"]
	n := len(elems)
	for n > 0 && len(base) >= n {
		match := true
		for i := 0; i < n; i++ {
			if base[len(base)-n+i] != elems[i] {
				match = false
				break
			}
		}
		if match {
			return elems[n:]
		}
		n--
	}
	return elems
}
