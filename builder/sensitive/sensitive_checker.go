package sensitive

import "context"

type Scenario string

// for text
const (
	ScenarioNicknameDetection Scenario = "nickname_detection"
	ScenarioChatDetection     Scenario = "chat_detection"
	ScenarioCommentDetection  Scenario = "comment_detection"
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
	default:
		return Scenario(""), false
	}
}

type SensitiveChecker interface {
	PassTextCheck(ctx context.Context, scenario Scenario, text string) (bool, error)
	PassImageCheck(ctx context.Context, scenario Scenario, ossBucketName, ossObjectName string) (bool, error)
}
