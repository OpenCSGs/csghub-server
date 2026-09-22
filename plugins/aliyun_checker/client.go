package aliyunchecker

import (
	"context"
	"errors"
	"io"

	v1 "opencsg.com/csghub-server/plugins/aliyun_checker/v1"
)

const streamChunkSize = 64 * 1024

type Client struct {
	checker v1.AliyunCheckerClient
}

func NewClient(checker v1.AliyunCheckerClient) *Client {
	return &Client{checker: checker}
}

func (c *Client) PassTextCheck(ctx context.Context, scenario, text string) (*CheckResult, error) {
	result, err := c.checker.PassTextCheck(ctx, &v1.PassTextCheckRequest{Scenario: scenario, Text: text})
	return checkResultFromProto(result), err
}

func (c *Client) PassImageCheck(ctx context.Context, scenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	result, err := c.checker.PassImageCheck(ctx, &v1.PassImageCheckRequest{
		Scenario:      scenario,
		OssBucketName: ossBucketName,
		OssObjectName: ossObjectName,
	})
	return checkResultFromProto(result), err
}

func (c *Client) PassImageURLCheck(ctx context.Context, scenario, imageURL string) (*CheckResult, error) {
	result, err := c.checker.PassImageURLCheck(ctx, &v1.PassImageURLCheckRequest{
		Scenario: scenario,
		ImageUrl: imageURL,
	})
	return checkResultFromProto(result), err
}

func (c *Client) PassImageStreamCheck(ctx context.Context, scenario string, reader io.Reader) (*CheckResult, error) {
	stream, err := c.checker.PassImageStreamCheck(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&v1.ImageStreamFrame{Frame: &v1.ImageStreamFrame_Header{
		Header: &v1.ImageStreamHeader{Scenario: scenario},
	}}); err != nil {
		return nil, err
	}

	chunk := make([]byte, streamChunkSize)
	for {
		n, readErr := reader.Read(chunk)
		if n > 0 {
			if err := stream.Send(&v1.ImageStreamFrame{Frame: &v1.ImageStreamFrame_Chunk{
				Chunk: chunk[:n],
			}}); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}

	result, err := stream.CloseAndRecv()
	return checkResultFromProto(result), err
}

func (c *Client) PassLLMCheck(ctx context.Context, req *LLMCheckRequest) (*CheckResult, error) {
	if req == nil {
		return nil, errors.New("llm check request is required")
	}
	result, err := c.checker.PassLLMCheck(ctx, &v1.PassLLMCheckRequest{
		Scenario:           req.Scenario,
		Text:               req.Text,
		SessionId:          req.SessionID,
		AccountId:          req.AccountID,
		MaxTokens:          req.MaxTokens,
		RawJson:            req.RawJSON,
		Resumable:          req.Resumable,
		ModelName:          req.ModelName,
		Role:               req.Role,
		Stream:             req.Stream,
		AppendSystemPromot: req.IsAppendSystemPromot,
	})
	return checkResultFromProto(result), err
}

func (c *Client) SubmitMediaModeration(ctx context.Context, req *MediaModerationRequest) (*MediaModerationSubmission, error) {
	if req == nil {
		return nil, errors.New("media moderation request is required")
	}
	result, err := c.checker.SubmitMediaModeration(ctx, &v1.MediaModerationRequest{
		Url: req.URL, Type: req.Type, DataId: req.DataID, Seed: req.Seed, TaskId: req.TaskID,
	})
	if err != nil {
		return nil, err
	}
	return &MediaModerationSubmission{
		DataID: result.GetDataId(),
		Seed:   result.GetSeed(),
		TaskID: result.GetTaskId(),
	}, nil
}

func (c *Client) QueryMediaModerationResult(ctx context.Context, req *MediaModerationRequest) (*MediaModerationResult, error) {
	if req == nil {
		return nil, errors.New("media moderation request is required")
	}
	result, err := c.checker.QueryMediaModerationResult(ctx, &v1.MediaModerationRequest{
		Url: req.URL, Type: req.Type, DataId: req.DataID, Seed: req.Seed, TaskId: req.TaskID,
	})
	if err != nil {
		return nil, err
	}
	return &MediaModerationResult{
		DataID: result.GetDataId(),
		Status: result.GetStatus(),
		Reason: result.GetReason(),
	}, nil
}

func checkResultFromProto(result *v1.CheckResult) *CheckResult {
	if result == nil {
		return &CheckResult{}
	}
	return &CheckResult{IsSensitive: result.GetIsSensitive(), Reason: result.GetReason()}
}
