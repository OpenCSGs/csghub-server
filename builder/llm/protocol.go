package llm

import (
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// upstreamCall is the resolved native-call plan for the selected upstream.
type upstreamCall struct {
	// protocol is the API format the upstream natively serves for this
	// call: chat, responses (both openai-go SDK), or messages
	// (anthropic-sdk-go). The empty value marks an upstream without any
	// protocol declaration, which keeps the legacy passthrough: the URL is
	// used as-is with the openai chat wire format over manual HTTP.
	protocol types.ApiFormat
	// endpoint is the request target: the SDK base URL for the declared
	// protocols, or the URL as-is for the passthrough.
	endpoint string
	// headers carries the upstream auth headers.
	headers map[string]string
	// model is the upstream-requested model ID override ("" keeps the
	// caller-supplied model).
	model string
	// host is the optional virtual-host override from upstream metadata.
	host string
}

// callableProtocols lists the API formats an upstream can serve for internal
// llm.NewClient calls, in preference order: native openai chat needs no SDK
// round-trip, then the openai responses API, then the anthropic messages API.
// systemone (jev) and gemini are excluded on purpose: internal callers submit
// plain text tasks that cannot be expressed as a jev state+questions request.
var callableProtocols = []types.ApiFormat{
	types.ApiFormatChat,
	types.ApiFormatResponses,
	types.ApiFormatMessages,
}

// resolveUpstreamCall picks the upstream to call and derives its native
// protocol, request target, auth headers, and model override. The upstream
// preference mirrors LLMConfig.PopulateDerivedFields: healthy enabled
// upstreams first, then enabled ones, then upstreams[0] as a last resort.
func resolveUpstreamCall(cfg *database.LLMConfig) (*upstreamCall, error) {
	for _, u := range preferredUpstreams(cfg) {
		call, ok := buildUpstreamCall(u)
		if !ok {
			slog.Warn("skip upstream for internal llm call: none of its declared api formats is callable (chat/responses/messages)",
				slog.Int64("upstream_id", u.ID), slog.String("url", u.URL),
				slog.Any("supported_endpoints", u.SupportedEndpoints))
			continue
		}
		if call.model == "" {
			call.model = cfg.ModelName
		}
		return call, nil
	}
	return nil, fmt.Errorf("no upstream of llm config %q serves a callable api format (chat/responses/messages) for internal llm calls", cfg.ModelName)
}

// preferredUpstreams orders the config's upstreams by the same preference
// rules as LLMConfig.PopulateDerivedFields.
func preferredUpstreams(cfg *database.LLMConfig) []*database.Upstream {
	var healthy, enabled []*database.Upstream
	for i := range cfg.Upstreams {
		u := &cfg.Upstreams[i]
		if u.URL == "" {
			continue
		}
		switch {
		case u.Enabled && u.IsHealthy():
			healthy = append(healthy, u)
		case u.Enabled:
			enabled = append(enabled, u)
		}
	}
	if len(healthy)+len(enabled) == 0 && len(cfg.Upstreams) > 0 && cfg.Upstreams[0].URL != "" {
		// Last resort: upstreams[0] even when disabled, mirroring
		// PopulateDerivedFields.
		return []*database.Upstream{&cfg.Upstreams[0]}
	}
	return append(healthy, enabled...)
}

// buildUpstreamCall derives the native call plan for one upstream. It reports
// false when the upstream is not callable for internal calls (its declared
// formats are all outside callableProtocols).
func buildUpstreamCall(u *database.Upstream) (*upstreamCall, bool) {
	if u.URL == "" {
		return nil, false
	}
	uc := upstreamConfigOf(u)
	authHeaders, err := types.ParseAuthHeaders(u.AuthHeader)
	if err != nil {
		slog.Error("upstream auth header is neither a JSON header map nor a Bearer token, request will proceed without resolved headers",
			slog.Int64("upstream_id", u.ID), slog.Any("error", err))
	}
	call := &upstreamCall{
		headers: authHeaders,
		model:   u.ModelName,
		host:    uc.InternalHost(),
	}
	if !uc.HasProtocolDeclaration() {
		// Undeclared upstream: pass the URL through as-is with the openai
		// chat wire format over manual HTTP, preserving the legacy
		// behavior. The SDK path would append the standard resource path
		// and break non-standard endpoint URLs.
		call.endpoint = u.URL
		return call, true
	}
	for _, p := range callableProtocols {
		if !uc.SupportsProtocol(p) {
			continue
		}
		call.protocol = p
		call.endpoint = uc.SDKBaseURL(p)
		return call, true
	}
	return nil, false
}

// upstreamConfigOf adapts a database upstream to the shared UpstreamConfig
// helpers, mirroring Upstream.EffectiveSampleEndpoint.
func upstreamConfigOf(u *database.Upstream) types.UpstreamConfig {
	return types.UpstreamConfig{
		URL:                u.URL,
		IsFullURL:          u.IsFullURL,
		Source:             u.Source,
		SupportedEndpoints: u.SupportedEndpoints,
		Metadata:           u.Metadata,
	}
}
