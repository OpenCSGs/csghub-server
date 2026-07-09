package types

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
