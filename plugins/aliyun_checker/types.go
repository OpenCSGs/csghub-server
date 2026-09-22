package aliyunchecker

import (
	"context"
	"io"
)

type Checker interface {
	PassTextCheck(ctx context.Context, scenario, text string) (*CheckResult, error)
	PassImageCheck(ctx context.Context, scenario, ossBucketName, ossObjectName string) (*CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario, imageURL string) (*CheckResult, error)
	PassImageStreamCheck(ctx context.Context, scenario string, reader io.Reader) (*CheckResult, error)
	PassLLMCheck(ctx context.Context, req *LLMCheckRequest) (*CheckResult, error)
	SubmitMediaModeration(ctx context.Context, req *MediaModerationRequest) (*MediaModerationSubmission, error)
	QueryMediaModerationResult(ctx context.Context, req *MediaModerationRequest) (*MediaModerationResult, error)
}

type CheckResult struct {
	IsSensitive bool
	Reason      string
}

type MediaModerationRequest struct {
	URL    string
	Type   string
	DataID string
	Seed   string
	TaskID string
}

type MediaModerationSubmission struct {
	DataID string
	Seed   string
	TaskID string
}

type MediaModerationResult struct {
	DataID string
	Status string
	Reason string
}

type LLMCheckRequest struct {
	Scenario             string
	Text                 string
	SessionID            string
	AccountID            string
	MaxTokens            int64
	RawJSON              string
	Resumable            bool
	ModelName            string
	Role                 string
	Stream               bool
	IsAppendSystemPromot bool
}
