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
	commontypes "opencsg.com/csghub-server/common/types"
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
	BackendURL       string // the URL to send the upstream request to
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
	// ProtocolOverride is an explicit protocol declaration from upstream config.
	// When non-empty, it takes priority over URL inference and defaults.
	ProtocolOverride string
	// SupportedEndpoints maps a declared API format (chat / responses /
	// messages) to an optional per-protocol endpoint URL. An empty value
	// means the main Target serves that protocol; a missing key means the
	// protocol is not declared.
	SupportedEndpoints map[commontypes.ApiFormat]string
	// IsFullURL marks the Target as a complete endpoint URL that must be used
	// as-is. When false the Target is a base URL and the client protocol path
	// is appended for declared formats.
	IsFullURL bool
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
	// System One is a self-contained evaluation protocol with no adapters in
	// either direction: a systemone request is only served by a systemone
	// upstream (native passthrough), and vice versa.
	types.ProtocolSystemOne: {},
}

// ResolveRouting determines how a client request should be routed to the
// upstream endpoint.  The decision follows the Native > Adapter > Reject
// strategy.
func ResolveRouting(clientProtocol types.Protocol, target RoutingTarget) (RoutingDecision, error) {
	if strings.TrimSpace(target.Target) == "" && !target.CSGHubHosted {
		return RoutingDecision{}, fmt.Errorf("upstream target is empty")
	}

	// Declared-format fast path: the upstream config explicitly lists the API
	// formats it natively speaks. When the client's protocol is one of them,
	// forward natively — the URL as-is when it is a full endpoint URL, or the
	// base URL with the client protocol path appended.
	supported := SupportedProtocolsOf(target)
	if supported[clientProtocol] {
		return RoutingDecision{
			Mode:             ModeNative,
			BackendURL:       nativeBackendURL(clientProtocol, target),
			UpstreamProtocol: clientProtocol,
			Reason:           "declared_format_match",
		}, nil
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
	// A non-empty SupportedEndpoints declaration is an explicit statement of
	// what the upstream speaks, so the passthrough must not override it.
	if upstreamProtocol == types.ProtocolChat && !target.CSGHubHosted &&
		clientProtocol != types.ProtocolChat &&
		len(supported) == 0 &&
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

	// Chat fallback: Chat (/v1/chat/completions) is the most widely compatible
	// protocol.  When the client requests Chat but the upstream is configured
	// as Responses or Messages, do not reject the request.  Instead rewrite
	// the upstream URL path to /v1/chat/completions and forward natively.
	// Most upstreams that expose /responses or /messages also accept
	// /chat/completions, so a direct proxy is sufficient without protocol
	// translation.
	if clientProtocol == types.ProtocolChat &&
		(upstreamProtocol == types.ProtocolResponses || upstreamProtocol == types.ProtocolMessages) {
		return RoutingDecision{
			Mode:             ModeNative,
			BackendURL:       rewriteToChatPath(target),
			UpstreamProtocol: upstreamProtocol,
			Reason:           "chat_fallback",
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
	// 1. Explicit declaration via protocol override.
	if target.ProtocolOverride != "" {
		p := strings.TrimSpace(strings.ToLower(target.ProtocolOverride))
		switch types.Protocol(p) {
		case types.ProtocolChat, types.ProtocolResponses, types.ProtocolMessages, types.ProtocolSystemOne:
			return types.Protocol(p)
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

// SupportedProtocolsOf returns the set of API formats the upstream explicitly
// declares: the keys of the SupportedEndpoints map unioned with the legacy
// metadata.protocol override. An empty set means the protocol is undeclared
// and must be inferred (legacy behavior).
//
// Coupling note: this set is EXPLICIT-ONLY (map keys + metadata.protocol).
// The component layer's UpstreamConfig.SupportsProtocol/
// legacyDeclaredEndpoints (common/types) additionally falls back to URL
// inference and the CSGHub chat default when classifying candidates for
// native-first selection. The split is deliberate: for undeclared upstreams
// the routing layer's own legacy machinery (DetectUpstreamProtocol → URL
// inference → CSGHub chat default, then the prefer-client-passthrough case)
// must stay authoritative, so SupportedProtocolsOf must not pre-empt it.
// Consequence: a legacy URL-inferable upstream is native-capable for
// selection but enters ResolveRouting through the legacy protocol_match /
// chat_fallback paths instead of the declared fast path — same final URL
// (appendProtocolPath derives it identically), different Reason string.
// Any change to either resolution order must be mirrored in the other.
func SupportedProtocolsOf(target RoutingTarget) map[types.Protocol]bool {
	set := make(map[types.Protocol]bool)
	for key := range target.SupportedEndpoints {
		if key.Normalized().Routable() {
			set[types.Protocol(key.Normalized())] = true
		}
	}
	if p := parseProtocolValue(target.ProtocolOverride); p != "" {
		set[p] = true
	}
	return set
}

// parseProtocolValue normalizes a declared protocol string to a known
// types.Protocol, returning "" for unknown values.
func parseProtocolValue(raw string) types.Protocol {
	format, ok := commontypes.ApiFormatOf(raw)
	if !ok || !format.Routable() {
		return ""
	}
	return types.Protocol(format)
}

// declaredEndpointValue returns the per-protocol endpoint override for the
// client protocol (trimmed; empty when the protocol is served by the main
// URL or not declared with an endpoint).
func declaredEndpointValue(target RoutingTarget, clientProtocol types.Protocol) string {
	for key, value := range target.SupportedEndpoints {
		if types.Protocol(key.Normalized()) == clientProtocol {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// appendProtocolPath resolves a URL (main or per-protocol endpoint) against
// the client protocol: the recognized protocol tail (if any) is stripped and
// the client protocol's standard path is appended (deduplicating a /v1
// prefix). Returns ok=false when the URL cannot be parsed or the protocol is
// unknown, in which case the input URL should be used as-is.
func appendProtocolPath(rawURL string, clientProtocol types.Protocol) (string, bool) {
	if clientProtocol == types.ProtocolSystemOne {
		// A SystemOne upstream URL is a complete business endpoint
		// (e.g. https://jev.internal/api/evaluate); never append a path.
		return rawURL, true
	}
	base := StripProtocolTail(rawURL)
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	segments := commontypes.ApiFormat(clientProtocol).StandardPathSegments()
	if len(segments) == 0 {
		return "", false
	}
	return appendEndpointPath(parsed, segments...), true
}

// nativeBackendURL builds the upstream URL for a natively supported client
// protocol. A per-protocol endpoint override wins: a URL already carrying the
// client protocol's path is used as-is, anything else is treated as a base
// URL. Otherwise a full-URL target is used as-is, and the main URL has its
// recognized protocol tail (if any) stripped before the client protocol's
// standard path is appended.
func nativeBackendURL(clientProtocol types.Protocol, target RoutingTarget) string {
	// CSGHub-hosted targets follow the hosted semantics: the URL is a bare
	// service root and the standard path is appended for the protocol
	// (identical to the legacy native-match path).
	if target.CSGHubHosted {
		return adaptBackendURL(target, clientProtocol)
	}
	if override := declaredEndpointValue(target, clientProtocol); override != "" {
		// The declared key tells us WHICH protocol the override serves, not
		// the URL's shape: the value may be a complete endpoint URL (pass
		// through untouched — e.g. DeepSeek's /anthropic/messages) or a
		// base URL (append the standard path — e.g. a DashScope apps
		// .../apps/anthropic namespace gets /v1/messages appended).
		// probeURLFor inspects the path to tell the two shapes apart and
		// shares those rules with the connection-test probe.
		if resolved, ok := commontypes.ApiFormat(clientProtocol).ProbeOverrideURL(override); ok {
			return resolved
		}
		return override
	}
	if target.IsFullURL {
		return target.Target
	}
	if resolved, ok := appendProtocolPath(target.Target, clientProtocol); ok {
		return resolved
	}
	return target.Target
}

// StripProtocolTail removes the recognized protocol terminal path from a URL
// so it can be used as a base URL: .../chat/completions, .../responses,
// .../messages and .../anthropic are all protocol namespaces rather than
// resource prefixes. URLs without a recognizable tail are returned unchanged.
//
// Note for providers that namespace their Anthropic API (e.g.
// https://api.deepseek.com/anthropic/messages): stripping the /messages tail
// keeps the /anthropic prefix in the base URL, so admins of such providers
// should use the full-URL mode instead of declaring base-URL formats.
func StripProtocolTail(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return rawURL
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		return rawURL
	}
	parts := strings.Split(path, "/")
	n := len(parts)
	var kept []string
	switch {
	case n >= 2 && strings.EqualFold(parts[n-2], commontypes.PathSegChat) && strings.EqualFold(parts[n-1], commontypes.PathSegCompletions):
		kept = parts[:n-2]
	case strings.EqualFold(parts[n-1], commontypes.PathSegResponses),
		strings.EqualFold(parts[n-1], commontypes.PathSegMessages):
		kept = parts[:n-1]
	default:
		return rawURL
	}
	parsed.Path = strings.Join(kept, "/")
	parsed.RawPath = ""
	return parsed.String()
}

// hasExplicitProtocolMetadata returns true when the upstream metadata contains
// a valid "protocol" key.  This is used by ResolveRouting to distinguish an
// explicit Chat declaration (which must be respected) from the Chat fallback
// default (which can be overridden to prefer native passthrough).
func hasExplicitProtocolMetadata(target RoutingTarget) bool {
	if target.ProtocolOverride == "" {
		return false
	}
	p := strings.TrimSpace(strings.ToLower(target.ProtocolOverride))
	switch types.Protocol(p) {
	case types.ProtocolChat, types.ProtocolResponses, types.ProtocolMessages, types.ProtocolSystemOne:
		return true
	}
	return false
}

// inferProtocolFromURL inspects the URL path to guess the protocol.  It
// delegates to the shared commontypes helper so admin CRUD, migration
// backfill and routing all agree on what a URL means.
func inferProtocolFromURL(rawURL string) (types.Protocol, bool) {
	p, ok := commontypes.InferProtocolFromURLPath(rawURL)
	return types.Protocol(p), ok
}

// adaptBackendURL rewrites the upstream URL to point at the correct path for
// the detected upstream protocol.  Full-URL targets and URLs that already
// carry a recognizable protocol path are used as-is; base URLs (CSGHub-hosted
// models, or upstreams with an explicit API-format declaration whose URL has
// no protocol path) get the standard path appended.
func adaptBackendURL(target RoutingTarget, upstreamProtocol types.Protocol) string {
	// If the URL already contains a recognizable path, trust it.
	if _, ok := inferProtocolFromURL(target.Target); ok {
		return target.Target
	}

	// Full-URL targets are used as-is by definition.
	if target.IsFullURL {
		return target.Target
	}

	// Append the standard path when the URL is a base URL: CSGHub-hosted
	// models always, and upstreams with an explicit API-format declaration
	// whose URL carries no protocol path (base-URL configuration).
	if !target.CSGHubHosted && len(SupportedProtocolsOf(target)) == 0 {
		return target.Target
	}
	parsed, err := url.Parse(target.Target)
	if err != nil || parsed.Host == "" {
		return target.Target
	}
	if upstreamProtocol == types.ProtocolSystemOne {
		// External SystemOne upstream URLs are complete business endpoints
		// (never append); hosted deploys expose a bare service root, so the
		// standard path is appended for them.
		if target.CSGHubHosted {
			return appendEndpointPath(parsed, commontypes.ApiFormatSystemOne.StandardPathSegments()...)
		}
		return target.Target
	}
	if segments := commontypes.ApiFormat(upstreamProtocol).StandardPathSegments(); len(segments) > 0 {
		return appendEndpointPath(parsed, segments...)
	}

	return target.Target
}

// rewriteToChatPath replaces the path of the upstream URL with
// /v1/chat/completions while preserving the scheme, host, port, query, and
// fragment.  This is used by the Chat fallback so that a request to a
// /responses or /messages upstream is proxied to the chat completions
// endpoint instead.
func rewriteToChatPath(target RoutingTarget) string {
	rawURL := target.Target
	if strings.TrimSpace(rawURL) == "" {
		return rawURL
	}
	// url.Parse interprets a bare host:port/path as scheme=host, so
	// prepend a scheme when absent to ensure the path is parsed correctly.
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return target.Target
	}
	// Replace only the terminal path segment so any prefix (e.g. /api/v1) is
	// preserved: /api/v1/responses -> /api/v1/chat/completions, not
	// /v1/chat/completions.
	//
	// A trailing /anthropic namespace segment directly before the terminal
	// /messages segment is dropped as well: providers such as DeepSeek expose
	// their Anthropic-compatible API under an /anthropic prefix
	// (https://api.deepseek.com/anthropic/messages) while the chat-completions
	// counterpart lives outside that namespace, so replacing the terminal
	// segment in place would yield /anthropic/chat/completions and 404.
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		parsed.Path = "/" + commontypes.PathSegV1 + "/" + commontypes.PathSegChat + "/" + commontypes.PathSegCompletions
	} else {
		parts := strings.Split(path, "/")
		// parsed.Path always starts with "/" here, so parts[0] is the empty
		// leading element and can never equal "anthropic"; two segments are
		// enough to inspect the one preceding the terminal segment.
		if n := len(parts); n >= 2 && parts[n-2] == commontypes.PathSegAnthropic && parts[n-1] == commontypes.PathSegMessages {
			parts = append(parts[:n-2], commontypes.PathSegChat+"/"+commontypes.PathSegCompletions)
		} else {
			parts[len(parts)-1] = commontypes.PathSegChat + "/" + commontypes.PathSegCompletions
		}
		parsed.Path = strings.Join(parts, "/")
	}
	parsed.RawPath = ""
	return parsed.String()
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
	// Keep the base URL's query (e.g. Azure's ?api-version=...): only the
	// path is rewritten; the fragment never survives a proxied request.
	return u.String()
}

// trimLeadingSegments removes leading segments from elems that already
// match the trailing segments of base.  This prevents path duplication
// when the base URL already contains a path prefix like /v1.
// For example: base=["v1"], elems=["v1","chat","completions"]
// → matching suffix of base is ["v1"], which is a prefix of elems
// → trim it, return ["chat","completions"].
func trimLeadingSegments(elems []string, base []string) []string {
	if len(base) == 0 || len(elems) == 0 {
		return elems
	}
	max := len(elems)
	if len(base) < max {
		max = len(base)
	}
	for n := max; n > 0; n-- {
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
	}
	return elems
}
