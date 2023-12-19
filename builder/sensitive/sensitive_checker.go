package sensitive

import "context"

type Scenario string

const (
	ScenarioNicknameDetection Scenario = "nickname_detection"
	ScenarioChatDetection     Scenario = "chat_detection"
	ScenarioCommentDetection  Scenario = "comment_detection"
)

func (s Scenario) FromString(scenario string) (Scenario, bool) {
	switch scenario {
	case "nickname_detection":
		return ScenarioNicknameDetection, true
	case "chat_detection":
		return ScenarioChatDetection, true
	case "comment_detection":
		return ScenarioCommentDetection, true
	default:
		return Scenario(""), false
	}
}

type SensitiveChecker interface {
	PassTextCheck(ctx context.Context, scenario Scenario, text string) (bool, error)
}
