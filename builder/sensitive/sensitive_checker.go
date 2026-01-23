package sensitive

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type SensitiveChecker interface {
	PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error)
	PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error)
	PassLLMCheck(ctx context.Context, scenario types.SensitiveScenario, text string, sessionId string, accountId string) (*CheckResult, error)
}

type ImageCheckReq struct {
	OSSBucketName string `json:"oss_bucket_name"`
	OSSObjectName string `json:"oss_object_name"`
}

type CheckResult struct {
	IsSensitive bool   `json:"is_sensitive"`
	Reason      string `json:"reason"`
}
