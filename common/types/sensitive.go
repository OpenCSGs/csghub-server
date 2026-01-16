package types

type SensitiveRequestV2 interface {
	GetSensitiveFields() []SensitiveField
}

type SensitiveField struct {
	Name  string
	Value func() string
	// like nickname, chat, comment, etc. See sensitive.Scenario for more details.
	Scenario SensitiveScenario
}

type SensitiveScenario string

// for text
const (
	ScenarioNicknameDetection SensitiveScenario = "nickname_detection"
	ScenarioChatDetection     SensitiveScenario = "chat_detection"
	ScenarioCommentDetection  SensitiveScenario = "comment_detection"
)

// for llm text
const (
	ScenarioLLMQueryModeration SensitiveScenario = "llm_query_moderation"
	ScenarioLLMResModeration   SensitiveScenario = "llm_response_moderation"
)

// for image
const (
	ScenarioImageProfileCheck  SensitiveScenario = "profilePhotoCheck"
	ScenarioImageBaseLineCheck SensitiveScenario = "baselineCheck"
)

func (s SensitiveScenario) FromString(scenario string) (SensitiveScenario, bool) {
	switch scenario {
	case "nickname_detection":
		return ScenarioNicknameDetection, true
	case "chat_detection":
		return ScenarioChatDetection, true
	case "comment_detection":
		return ScenarioCommentDetection, true
	case "profilePhotoCheck":
		return ScenarioImageProfileCheck, true
	case "baselineCheck":
		return ScenarioImageBaseLineCheck, true
	case "llm_response_moderation":
		return ScenarioLLMResModeration, true
	case "llm_query_moderation":
		return ScenarioLLMQueryModeration, true
	default:
		return SensitiveScenario(""), false
	}
}
