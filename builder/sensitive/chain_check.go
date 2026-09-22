package sensitive

import (
	"context"
	"fmt"
	"io"

	"opencsg.com/csghub-server/common/types"
	ss_type "opencsg.com/csghub-server/common/types/sensitive"
)

func (c *chainImpl) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*ss_type.CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassTextCheck(ctx, scenario, text)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &ss_type.CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*ss_type.CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &ss_type.CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*ss_type.CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassImageURLCheck(ctx, scenario, imageURL)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &ss_type.CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassImageStreamCheck(ctx context.Context, scenario types.SensitiveScenario, reader io.Reader) (*ss_type.CheckResult, error) {
	// If the reader is seekable, rewind it before each checker so every
	// checker receives the full, identical content. Without this, the first
	// checker consumes the stream and subsequent checkers read empty content.
	seeker, canSeek := reader.(io.Seeker)

	for _, checker := range c.checkers {
		if canSeek {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek image reader before checker: %w", err)
			}
		}
		res, err := checker.PassImageStreamCheck(ctx, scenario, reader)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &ss_type.CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassLLMCheck(ctx context.Context, req *types.LLMCheckRequest) (*ss_type.CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassLLMCheck(ctx, req)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &ss_type.CheckResult{IsSensitive: false}, nil
}

// SubmitMediaModeration delegates to the first checker in the chain that
// implements MediaSensitiveChecker (the Aliyun Green checker). AC-automaton
// checkers do not implement it and are skipped.
func (c *chainImpl) SubmitMediaModeration(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error) {
	for _, checker := range c.checkers {
		if mc, ok := checker.(ss_type.MediaSensitiveChecker); ok {
			return mc.SubmitMediaModeration(ctx, req)
		}
	}
	return nil, fmt.Errorf("no checker in the chain supports media moderation")
}

// QueryMediaModerationResult delegates to the first MediaSensitiveChecker.
func (c *chainImpl) QueryMediaModerationResult(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationResult, error) {
	for _, checker := range c.checkers {
		if mc, ok := checker.(ss_type.MediaSensitiveChecker); ok {
			return mc.QueryMediaModerationResult(ctx, req)
		}
	}
	return nil, fmt.Errorf("no checker in the chain supports media moderation")
}
