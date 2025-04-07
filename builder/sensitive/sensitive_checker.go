package sensitive

import "context"

type Scenario string

// for text
const (
	ScenarioNicknameDetection Scenario = "nickname_detection"
	ScenarioChatDetection     Scenario = "chat_detection"
	ScenarioCommentDetection  Scenario = "comment_detection"
)

// for llm text
const (
	ScenarioLLMQueryModeration Scenario = "llm_query_moderation"
	ScenarioLLMResModeration   Scenario = "llm_response_moderation"
)

// for image
const (
	ScenarioImageProfileCheck  Scenario = "profilePhotoCheck"
	ScenarioImageBaseLineCheck Scenario = "baselineCheck"
)

func (s Scenario) FromString(scenario string) (Scenario, bool) {
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
		return Scenario(""), false
	}
}

type SensitiveChecker interface {
	PassTextCheck(ctx context.Context, scenario Scenario, text string) (*CheckResult, error)
	PassImageCheck(ctx context.Context, scenario Scenario, ossBucketName, ossObjectName string) (*CheckResult, error)
	PassLLMCheck(ctx context.Context, scenario Scenario, text string, sessionId string, accountId string) (*CheckResult, error)
}

type CheckResult struct {
	IsSensitive bool   `json:"is_sensitive"`
	Reason      string `json:"reason"`
}
