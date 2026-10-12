package types

import "strings"

// ApiFormat identifies an upstream API format (protocol dialect) that an
// upstream can natively speak. It is the shared vocabulary for the
// supported_endpoints declaration, admin validation and gateway routing.
type ApiFormat string

const (
	// ApiFormatChat is the OpenAI Chat Completions API (/v1/chat/completions).
	ApiFormatChat ApiFormat = "chat"
	// ApiFormatResponses is the OpenAI Responses API (/v1/responses).
	ApiFormatResponses ApiFormat = "responses"
	// ApiFormatMessages is the Anthropic Messages API (/v1/messages).
	ApiFormatMessages ApiFormat = "messages"
	// ApiFormatGemini is the Google Gemini API. Declared for future use: it
	// is not yet routable (no adapter, no UI option) and must not be stored
	// in supported_endpoints until routing support lands.
	ApiFormatGemini ApiFormat = "gemini"
	// ApiFormatSystemOne is the SystemOne (jev) API (/v1/systemone), served
	// natively by the AIGateway SystemOne handler.
	ApiFormatSystemOne ApiFormat = "systemone"
)

// AllApiFormats lists every known API format, including ones without routing
// support yet. Do not iterate this for routing or validation — use
// RoutableApiFormats.
var AllApiFormats = []ApiFormat{
	ApiFormatChat,
	ApiFormatResponses,
	ApiFormatMessages,
	ApiFormatGemini,
	ApiFormatSystemOne,
}

// RoutableApiFormats lists the API formats the gateway can route natively or
// adapt today, in the canonical iteration order (chat first: it is the most
// widely compatible fallback for probe endpoints and backfill).
var RoutableApiFormats = []ApiFormat{
	ApiFormatChat,
	ApiFormatResponses,
	ApiFormatMessages,
	ApiFormatSystemOne,
}

// Valid reports whether f is a known API format.
func (f ApiFormat) Valid() bool {
	normalized := f.Normalized()
	for _, known := range AllApiFormats {
		if normalized == known {
			return true
		}
	}
	return false
}

// Routable reports whether f is a format the gateway can route today.
func (f ApiFormat) Routable() bool {
	normalized := f.Normalized()
	for _, known := range RoutableApiFormats {
		if normalized == known {
			return true
		}
	}
	return false
}

// Normalized lowercases and trims the format for case-insensitive comparison.
func (f ApiFormat) Normalized() ApiFormat {
	return ApiFormat(strings.ToLower(strings.TrimSpace(string(f))))
}

// ApiFormatOf normalizes a raw string to an ApiFormat; the second return is
// false when the value is not a known format.
func ApiFormatOf(raw string) (ApiFormat, bool) {
	f := ApiFormat(raw).Normalized()
	return f, f.Valid()
}

// Standard path segments used to build upstream URLs from a base URL. These
// are path SEGMENTS, not protocol names: chat's resource path is the two
// segments "chat" + "completions".
const (
	PathSegV1          = "v1"
	PathSegChat        = "chat"
	PathSegCompletions = "completions"
	PathSegResponses   = "responses"
	PathSegMessages    = "messages"
	PathSegAnthropic   = "anthropic"
	PathSegSystemOne   = "systemone"
)

// standardPathSegments maps each routable format to the path segments
// appended to a base URL (a /v1 base suffix is deduplicated by the callers).
var standardPathSegments = map[ApiFormat][]string{
	ApiFormatChat:      {PathSegV1, PathSegChat, PathSegCompletions},
	ApiFormatResponses: {PathSegV1, PathSegResponses},
	ApiFormatMessages:  {PathSegV1, PathSegMessages},
	ApiFormatSystemOne: {PathSegV1, PathSegSystemOne},
}

// StandardPathSegments returns the path segments appended to a base URL for
// the format, or nil for non-routable/unknown formats.
func (f ApiFormat) StandardPathSegments() []string {
	return standardPathSegments[f.Normalized()]
}
