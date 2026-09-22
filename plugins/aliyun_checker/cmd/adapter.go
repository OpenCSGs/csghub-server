package main

import (
	"context"
	"fmt"
	"io"

	"opencsg.com/csghub-server/common/types"
	aliyunchecker "opencsg.com/csghub-server/plugins/aliyun_checker"
	"opencsg.com/csghub-server/plugins/aliyun_checker/internal/aliyun"
)

type aliyunCheckerAdapter struct {
	checker *aliyun.AliyunGreenChecker
}

func newAliyunCheckerAdapter(checker *aliyun.AliyunGreenChecker) aliyunchecker.Checker {
	return &aliyunCheckerAdapter{checker: checker}
}

func (a *aliyunCheckerAdapter) PassTextCheck(ctx context.Context, scenario, text string) (*aliyunchecker.CheckResult, error) {
	return checkResult(a.checker.PassTextCheck(ctx, types.SensitiveScenario(scenario), text))
}

func (a *aliyunCheckerAdapter) PassImageCheck(ctx context.Context, scenario, ossBucketName, ossObjectName string) (*aliyunchecker.CheckResult, error) {
	return checkResult(a.checker.PassImageCheck(ctx, types.SensitiveScenario(scenario), ossBucketName, ossObjectName))
}

func (a *aliyunCheckerAdapter) PassImageURLCheck(ctx context.Context, scenario, imageURL string) (*aliyunchecker.CheckResult, error) {
	return checkResult(a.checker.PassImageURLCheck(ctx, types.SensitiveScenario(scenario), imageURL))
}

func (a *aliyunCheckerAdapter) PassImageStreamCheck(ctx context.Context, scenario string, reader io.Reader) (*aliyunchecker.CheckResult, error) {
	return checkResult(a.checker.PassImageStreamCheck(ctx, types.SensitiveScenario(scenario), reader))
}

func (a *aliyunCheckerAdapter) PassLLMCheck(ctx context.Context, req *aliyunchecker.LLMCheckRequest) (*aliyunchecker.CheckResult, error) {
	if req == nil {
		return nil, fmt.Errorf("llm check request is required")
	}
	result, err := a.checker.PassLLMCheck(ctx, &types.LLMCheckRequest{
		Scenario:             types.SensitiveScenario(req.Scenario),
		Text:                 req.Text,
		SessionId:            req.SessionID,
		AccountId:            req.AccountID,
		MaxTokens:            int(req.MaxTokens),
		RawJSON:              req.RawJSON,
		Resumable:            req.Resumable,
		ModelName:            req.ModelName,
		Role:                 req.Role,
		Stream:               req.Stream,
		IsAppendSystemPromot: req.IsAppendSystemPromot,
	})
	return checkResult(result, err)
}

func (a *aliyunCheckerAdapter) SubmitMediaModeration(ctx context.Context, req *aliyunchecker.MediaModerationRequest) (*aliyunchecker.MediaModerationSubmission, error) {
	if req == nil {
		return nil, fmt.Errorf("media moderation request is required")
	}
	result, err := a.checker.SubmitMediaModeration(ctx, types.MediaModerationRequest{
		URL: req.URL, Type: types.MediaType(req.Type), DataID: req.DataID, Seed: req.Seed, TaskID: req.TaskID,
	})
	if err != nil {
		return nil, err
	}
	return &aliyunchecker.MediaModerationSubmission{
		DataID: result.DataID,
		Seed:   result.Seed,
		TaskID: result.TaskID,
	}, nil
}

func (a *aliyunCheckerAdapter) QueryMediaModerationResult(ctx context.Context, req *aliyunchecker.MediaModerationRequest) (*aliyunchecker.MediaModerationResult, error) {
	if req == nil {
		return nil, fmt.Errorf("media moderation request is required")
	}
	result, err := a.checker.QueryMediaModerationResult(ctx, types.MediaModerationRequest{
		URL: req.URL, Type: types.MediaType(req.Type), DataID: req.DataID, Seed: req.Seed, TaskID: req.TaskID,
	})
	if err != nil {
		return nil, err
	}
	return &aliyunchecker.MediaModerationResult{
		DataID: result.DataID,
		Status: result.Status,
		Reason: result.Reason,
	}, nil
}

func checkResult(result *aliyun.CheckResult, err error) (*aliyunchecker.CheckResult, error) {
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &aliyunchecker.CheckResult{}, nil
	}
	return &aliyunchecker.CheckResult{IsSensitive: result.IsSensitive, Reason: result.Reason}, nil
}
