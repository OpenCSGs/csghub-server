package sensitive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gwtype "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type OpenAILLMChecker struct {
	config    *config.Config
	llmClient *llm.Client
	parser    LLMResponseParser
}

func NewOpenAILLMChecker(cfg *config.Config) *OpenAILLMChecker {
	if cfg.SensitiveCheck.LLM.Endpoint == "" {
		panic("SensitiveCheck.LLM.Endpoint is empty")
	}
	if cfg.SensitiveCheck.LLM.GuardStreamModel == "" {
		panic("SensitiveCheck.LLM.GuardStreamModel is empty")
	}
	if cfg.SensitiveCheck.LLM.GuardModel == "" {
		panic("SensitiveCheck.LLM.GuardModel is empty")
	}
	return &OpenAILLMChecker{
		config:    cfg,
		llmClient: llm.NewClient(),
		parser:    NewChainParser(cfg.SensitiveCheck.LLM.SafetyRegex),
	}
}

func (c *OpenAILLMChecker) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error) {
	// Chunk text logic
	maxChars := c.config.SensitiveCheck.StreamContextCache.MaxChars
	if maxChars <= 0 {
		maxChars = 2000
	}

	req := &types.LLMCheckRequest{
		Text:      text,
		MaxTokens: c.config.SensitiveCheck.LLM.MaxTokens,
		ModelName: c.config.SensitiveCheck.LLM.GuardModel,
		Resumable: false,
		Stream:    false,
		Role:      string(gwtype.RoleUser),
	}
	if len(text) <= maxChars {
		return c.doCheck(ctx, req)
	}

	// Simple chunking for large text
	for i := 0; i < len(text); i += maxChars {
		end := i + maxChars
		if end > len(text) {
			end = len(text)
		}
		req.Text = text[i:end]
		res, err := c.doCheck(ctx, req)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			return res, nil
		}
	}

	return &CheckResult{IsSensitive: false}, nil
}

func (c *OpenAILLMChecker) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	// Not supported by text LLM, default to pass
	return &CheckResult{IsSensitive: false}, nil
}

func (c *OpenAILLMChecker) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error) {
	// Not supported by text LLM, default to pass
	return &CheckResult{IsSensitive: false}, nil
}

func (c *OpenAILLMChecker) PassLLMCheck(ctx context.Context, req *types.LLMCheckRequest) (*CheckResult, error) {
	return c.doCheck(ctx, req)
}

func (c *OpenAILLMChecker) doCheck(ctx context.Context, req *types.LLMCheckRequest) (*CheckResult, error) {
	if req.Text == "" {
		return &CheckResult{IsSensitive: false}, nil
	}

	// API Request
	reqBody := types.LLMReqBody{
		Model: req.ModelName,
		Messages: []types.LLMMessage{
			{Role: req.Role, Content: req.Text},
		},
		Stream:      false,
		Temperature: c.config.SensitiveCheck.LLM.Temperature,
		MaxTokens:   req.MaxTokens,
		RawJSON:     req.RawJSON,
	}

	endpoint := c.config.SensitiveCheck.LLM.Endpoint
	headers := make(map[string]string)
	headers["x-session-id"] = req.SessionId
	headers["x-resumable"] = fmt.Sprintf("%v", req.Resumable)
	if c.config.SensitiveCheck.LLM.APIKey != "" {
		headers["Authorization"] = "Bearer " + c.config.SensitiveCheck.LLM.APIKey
	}

	// Retry mechanism for 429
	var content string
	var err error
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(c.config.SensitiveCheck.LLM.TimeoutMS)*time.Millisecond)
		content, err = c.llmClient.Chat(timeoutCtx, endpoint, "", headers, reqBody)
		cancel()
		if err == nil {
			break
		}
		// If not 429, don't retry (assuming llmClient returns "unexpected http status code:429")
		if !strings.Contains(err.Error(), "429") {
			break
		}
		// exponential backoff or simple sleep
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		// Check if it's a 429 error (assuming llmClient returns a specific error format or we can check string)
		// In our current llm.Client, non-2xx status returns "unexpected http status code:%d"
		slog.ErrorContext(ctx, "llm checker api request failed", slog.Any("error", err))
		// Fail-open
		return &CheckResult{IsSensitive: false, Reason: "skipped_api_error"}, nil
	}

	if content == "" {
		return &CheckResult{IsSensitive: false, Reason: "skipped_empty_response"}, nil
	}

	return c.parser.Parse(content), nil
}
