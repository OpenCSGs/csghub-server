package rpc

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	utils "opencsg.com/csghub-server/common/utils/common"
)

const PRINT_STRING_LEN = 1000

type ModerationSvcClient interface {
	PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error)
	PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error)
	PassLLMRespCheck(ctx context.Context, req types.LLMCheckRequest) (*CheckResult, error)
	PassLLMPromptCheck(ctx context.Context, req types.LLMCheckRequest) (*CheckResult, error)
	SubmitRepoCheck(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
}

type CheckResult struct {
	IsSensitive bool   `json:"is_sensitive"`
	Reason      string `json:"reason"`
}

type ModerationSvcHttpClient struct {
	hc *HttpClient
}

func NewModerationSvcHttpClient(endpoint string, opts ...RequestOption) ModerationSvcClient {
	return &ModerationSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *ModerationSvcHttpClient) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error) {
	type CheckRequest struct {
		Scenario types.SensitiveScenario `json:"scenario"`
		Text     string                  `json:"text"`
	}

	req := &CheckRequest{
		Scenario: scenario,
		Text:     text,
	}
	const path = "/api/v1/text"
	var resp httpbase.R
	resp.Data = &CheckResult{}
	err := c.hc.Post(ctx, path, req, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call moderation service failed", slog.String("error", err.Error()), slog.Any("req", req))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "moderation service").
				Set("action", "text check").
				Set("detail", err.Error()))
	}
	return resp.Data.(*CheckResult), nil
}

// If sessionID is set, used to check stream response; if not set, check non-stream.
func (c *ModerationSvcHttpClient) PassLLMRespCheck(ctx context.Context, req types.LLMCheckRequest) (*CheckResult, error) {
	req.Scenario = types.ScenarioLLMResModeration
	const path = "/api/v1/llmresp"
	var resp httpbase.R
	resp.Data = &CheckResult{}
	err := c.hc.Post(ctx, path, req, &resp)
	if err != nil {
		req.Text = utils.TruncStringByRune(req.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "call moderation service failed", slog.String("error", err.Error()), slog.Any("req", req))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "moderation service").
				Set("action", "llm response check"))
	}
	return resp.Data.(*CheckResult), nil
}

func (c *ModerationSvcHttpClient) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	type CheckRequest struct {
		Scenario      types.SensitiveScenario `json:"scenario"`
		OssBucketName string                  `json:"oss_bucket_name"`
		OssObjectName string                  `json:"oss_object_name"`
	}

	req := &CheckRequest{
		Scenario:      scenario,
		OssBucketName: ossBucketName,
		OssObjectName: ossObjectName,
	}
	var resp httpbase.R
	resp.Data = &CheckResult{}
	const path = "/api/v1/image"
	err := c.hc.Post(ctx, path, req, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call moderation service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "moderation service").
				Set("action", "image check"))
	}
	return resp.Data.(*CheckResult), nil
}

func (c *ModerationSvcHttpClient) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error) {
	type CheckRequest struct {
		Scenario types.SensitiveScenario `json:"scenario"`
		ImageURL string                  `json:"image_url"`
	}

	req := &CheckRequest{
		Scenario: scenario,
		ImageURL: imageURL,
	}
	const path = "/api/v1/image"
	var resp httpbase.R
	resp.Data = &CheckResult{}
	err := c.hc.Post(ctx, path, req, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call moderation service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "moderation service").
				Set("action", "image url check"))
	}
	return resp.Data.(*CheckResult), nil
}

func (c *ModerationSvcHttpClient) SubmitRepoCheck(ctx context.Context, repoType types.RepositoryType, namespace, name string) error {
	type CheckRequest struct {
		RepoType  types.RepositoryType `json:"repo_type"`
		Namespace string               `json:"namespace"`
		Name      string               `json:"name"`
	}

	req := &CheckRequest{
		RepoType:  repoType,
		Namespace: namespace,
		Name:      name,
	}
	const path = "/api/v1/repo"
	var resp httpbase.R
	err := c.hc.Post(ctx, path, req, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call moderation service failed", slog.String("error", err.Error()), slog.Any("req", req))
		return errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "moderation service").
				Set("action", "submit repo check"))
	}
	return nil
}

func (c *ModerationSvcHttpClient) PassLLMPromptCheck(ctx context.Context, req types.LLMCheckRequest) (*CheckResult, error) {
	req.Scenario = types.ScenarioLLMQueryModeration
	const path = "/api/v1/llmprompt"
	var resp httpbase.R
	resp.Data = &CheckResult{}
	err := c.hc.Post(ctx, path, req, &resp)
	if err != nil {
		req.Text = utils.TruncStringByRune(req.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "call moderation service failed", slog.String("error", err.Error()), slog.Any("req", req))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "moderation service").
				Set("path", path))
	}
	return resp.Data.(*CheckResult), nil
}
