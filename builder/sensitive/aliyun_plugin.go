package sensitive

import (
	"context"
	"fmt"
	"io"

	pluginmanager "opencsg.com/csghub-server/builder/plugins_manager"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	aliyunchecker "opencsg.com/csghub-server/plugins/aliyun_checker"
)

type pluginAliyunChecker struct {
	path       string
	pluginName string
	version    string
	checkerFor func(ctx context.Context) (aliyunchecker.Checker, error)
	config     *config.Config
}

var _ SensitiveChecker = (*pluginAliyunChecker)(nil)
var _ MediaSensitiveChecker = (*pluginAliyunChecker)(nil)

func newPluginAliyunChecker(cfg *config.Config) SensitiveChecker {
	checker := &pluginAliyunChecker{
		path:       cfg.PluginPath,
		pluginName: aliyunchecker.PluginName,
		version:    "v1.0.0",
		config:     cfg,
	}
	checker.checkerFor = checker.pluginChecker
	return checker
}

func (c *pluginAliyunChecker) SensitiveCheckEnv() []string {
	return []string{
		"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_ID=" + c.config.SensitiveCheck.AccessKeyID,
		"STARHUB_SERVER_SENSITIVE_CHECK_ACCESS_KEY_SECRET=" + c.config.SensitiveCheck.AccessKeySecret,
		"STARHUB_SERVER_SENSITIVE_CHECK_REGION=" + c.config.SensitiveCheck.Region,
		"STARHUB_SERVER_SENSITIVE_CHECK_ENDPOINT=" + c.config.SensitiveCheck.Endpoint,
		"STARHUB_SERVER_SENSITIVE_CHECK_OSS_BUCKET=" + c.config.SensitiveCheck.OSSBucket,
		"STARHUB_SERVER_SENSITIVE_CHECK_ENABLE_SSL=" + fmt.Sprintf("%v", c.config.SensitiveCheck.EnableSSL),
	}
}

func (c *pluginAliyunChecker) pluginChecker(ctx context.Context) (aliyunchecker.Checker, error) {
	api, err := pluginmanager.Default().Get(pluginmanager.Definition{
		Name:            aliyunchecker.PluginName,
		Command:         []string{fmt.Sprintf("%s/%s/%s/%s", c.path, c.pluginName, c.version, c.pluginName)},
		HandshakeConfig: aliyunchecker.HandshakeConfig,
		Plugin:          aliyunchecker.NewPlugin(nil),
		Env:             c.SensitiveCheckEnv(),
	})
	if err != nil {
		return nil, fmt.Errorf("get aliyun checker plugin: %w", err)
	}

	client, ok := api.(*aliyunchecker.Client)
	if !ok {
		return nil, fmt.Errorf("plugin %s returned unsupported API type %T", aliyunchecker.PluginName, api)
	}
	return client, nil
}

func (c *pluginAliyunChecker) checker(ctx context.Context) (aliyunchecker.Checker, error) {
	return c.checkerFor(ctx)
}

func (c *pluginAliyunChecker) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error) {
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.PassTextCheck(ctx, string(scenario), text)
	return checkResultFromAliyunPlugin(result), err
}

func (c *pluginAliyunChecker) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.PassImageCheck(ctx, string(scenario), ossBucketName, ossObjectName)
	return checkResultFromAliyunPlugin(result), err
}

func (c *pluginAliyunChecker) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error) {
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.PassImageURLCheck(ctx, string(scenario), imageURL)
	return checkResultFromAliyunPlugin(result), err
}

func (c *pluginAliyunChecker) PassImageStreamCheck(ctx context.Context, scenario types.SensitiveScenario, reader io.Reader) (*CheckResult, error) {
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.PassImageStreamCheck(ctx, string(scenario), reader)
	return checkResultFromAliyunPlugin(result), err
}

func (c *pluginAliyunChecker) PassLLMCheck(ctx context.Context, req *types.LLMCheckRequest) (*CheckResult, error) {
	if req == nil {
		return nil, fmt.Errorf("llm check request is required")
	}
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.PassLLMCheck(ctx, &aliyunchecker.LLMCheckRequest{
		Scenario:             string(req.Scenario),
		Text:                 req.Text,
		SessionID:            req.SessionId,
		AccountID:            req.AccountId,
		MaxTokens:            int64(req.MaxTokens),
		RawJSON:              req.RawJSON,
		Resumable:            req.Resumable,
		ModelName:            req.ModelName,
		Role:                 req.Role,
		Stream:               req.Stream,
		IsAppendSystemPromot: req.IsAppendSystemPromot,
	})
	return checkResultFromAliyunPlugin(result), err
}

func (c *pluginAliyunChecker) SubmitMediaModeration(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error) {
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.SubmitMediaModeration(ctx, &aliyunchecker.MediaModerationRequest{
		URL: req.URL, Type: string(req.Type), DataID: req.DataID, Seed: req.Seed, TaskID: req.TaskID,
	})
	if err != nil {
		return nil, err
	}
	return &types.MediaModerationSubmission{
		DataID: result.DataID,
		Seed:   result.Seed,
		TaskID: result.TaskID,
	}, nil
}

func (c *pluginAliyunChecker) QueryMediaModerationResult(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationResult, error) {
	checker, err := c.checker(ctx)
	if err != nil {
		return nil, err
	}
	result, err := checker.QueryMediaModerationResult(ctx, &aliyunchecker.MediaModerationRequest{
		URL: req.URL, Type: string(req.Type), DataID: req.DataID, Seed: req.Seed, TaskID: req.TaskID,
	})
	if err != nil {
		return nil, err
	}
	return &types.MediaModerationResult{
		DataID: result.DataID,
		Status: result.Status,
		Reason: result.Reason,
	}, nil
}

func checkResultFromAliyunPlugin(result *aliyunchecker.CheckResult) *CheckResult {
	if result == nil {
		return &CheckResult{}
	}
	return &CheckResult{IsSensitive: result.IsSensitive, Reason: result.Reason}
}
