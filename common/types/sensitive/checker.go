package sensitive

import (
	"context"
	"io"

	"opencsg.com/csghub-server/common/types"
)

type SensitiveChecker interface {
	PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error)
	PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error)
	PassImageStreamCheck(ctx context.Context, scenario types.SensitiveScenario, reader io.Reader) (*CheckResult, error)
	PassLLMCheck(ctx context.Context, req *types.LLMCheckRequest) (*CheckResult, error)
}

// MediaSensitiveChecker is implemented by checkers that support asynchronous
// audio/video moderation (submit + poll). AliyunGreenChecker implements it;
// the AC-automaton checkers do not. The chain type-asserts each checker to
// this interface so non-media checkers are unaffected.
type MediaSensitiveChecker interface {
	SubmitMediaModeration(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error)
	QueryMediaModerationResult(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationResult, error)
}

type ImageCheckReq struct {
	OSSBucketName string `json:"oss_bucket_name"`
	OSSObjectName string `json:"oss_object_name"`
}

type CheckResult struct {
	IsSensitive bool   `json:"is_sensitive"`
	Reason      string `json:"reason"`
}
