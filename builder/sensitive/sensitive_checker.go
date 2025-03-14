package sensitive

import "context"

type Scenario string

// for text
const (
	ScenarioNicknameDetection Scenario = "nickname_detection"
	ScenarioChatDetection     Scenario = "chat_detection"
	ScenarioCommentDetection  Scenario = "comment_detection"
)

// for llm response
const (
	ScenarioLLMResModeration Scenario = "llm_response_moderation"
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
	default:
		return Scenario(""), false
	}
}

type SensitiveChecker interface {
	PassTextCheck(ctx context.Context, scenario Scenario, text string) (*CheckResult, error)
	PassImageCheck(ctx context.Context, scenario Scenario, ossBucketName, ossObjectName string) (*CheckResult, error)
	PassStreamCheck(ctx context.Context, scenario Scenario, text, id string) (*CheckResult, error)
}

type CheckResult struct {
	IsSensitive bool   `json:"is_sensitive"`
	Reason      string `json:"reason"`
}
