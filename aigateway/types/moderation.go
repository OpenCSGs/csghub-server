package types

import "context"

type TextModerationPhase string

const (
	TextModerationPhasePrompt   TextModerationPhase = "prompt"
	TextModerationPhaseResponse TextModerationPhase = "response"
)

type TextModerationMode string

const (
	TextModerationModeNonStream TextModerationMode = "non_stream"
	TextModerationModeStream    TextModerationMode = "stream"
)

type TextModerationRequest struct {
	Content string
	Key     string
	Phase   TextModerationPhase
	Mode    TextModerationMode
}

// moderationSessionIDCtxKey keys the per-request moderation session id in
// the request context. The planner generates the id when the prompt check
// runs; response-side moderation reads it back from the request context.
type moderationSessionIDCtxKey struct{}

// WithModerationSessionID returns a context carrying the per-request
// moderation session id.
func WithModerationSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, moderationSessionIDCtxKey{}, sessionID)
}

// ModerationSessionIDFromContext returns the per-request moderation session
// id, or "" when the context carries none.
func ModerationSessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(moderationSessionIDCtxKey{}).(string)
	return id
}
