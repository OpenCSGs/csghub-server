package component

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// ToolPoissoningPlugin used to check tool poison attack
type toolPoisoningPlugin struct {
	llmClient     *llm.Client
	prompt        *database.PromptPrefix
	summaryPrompt *database.PromptPrefix
	llmConfig     *database.LLMConfig
	temperature   float64
}

func newToolPoisoningPlugin(llmClient *llm.Client, prompt *database.PromptPrefix, summaryPrompt *database.PromptPrefix, llmConfig *database.LLMConfig, temperature float64) MCPScannerPlugin {
	return &toolPoisoningPlugin{
		llmClient:     llmClient,
		prompt:        prompt,
		summaryPrompt: summaryPrompt,
		llmConfig:     llmConfig,
		temperature:   temperature,
	}
}

func (plugin *toolPoisoningPlugin) Check(ctx context.Context, files []*types.File) ([]types.ScannerIssue, error) {
	var resultBuf strings.Builder
	scannerIssueResults := make([]types.ScannerIssue, 0)
	// 1. Get the code path
	for _, file := range files {
		if ignoreFile(file) {
			continue
		}
		// 2. Check the code content, file by file
		reqData := types.LLMReqBody{
			Model: plugin.llmConfig.ModelName,
			Messages: []types.LLMMessage{
				{Role: SystemRole, Content: plugin.prompt.ZH},
				{Role: UserRole, Content: file.Content},
			},
			Stream:      false,
			Temperature: plugin.temperature,
		}
		var headers map[string]string
		err := json.Unmarshal([]byte(plugin.llmConfig.AuthHeader), &headers)
		if err != nil {
			return nil, fmt.Errorf("parse llm config header error: %w", err)
		}
		resp, err := plugin.llmClient.Chat(ctx, plugin.llmConfig.ApiEndpoint, headers, reqData)
		if err != nil {
			return nil, err
		}
		resultBuf.WriteString(resp)
		singleFileIssue, err := summaryResult(ctx, plugin.llmClient, plugin.llmConfig, plugin.summaryPrompt.EN, resp, plugin.temperature)
		if err != nil {
			return nil, err
		}
		for i := range singleFileIssue {
			singleFileIssue[i].FilePath = file.Path
		}
		scannerIssueResults = append(scannerIssueResults, singleFileIssue...)
	}
	// 3. Summarize the results
	// summaryResult, err := SummaryResult(ctx, plugin.llmClient, plugin.llmConfig, plugin.summaryPrompt.EN, resultBuf.String())
	// if err != nil {
	//	return nil, err
	// }
	// scannerIssueResults = append(scannerIssueResults, summaryResult...)
	return scannerIssueResults, nil
}

func (plugin *toolPoisoningPlugin) GetName() types.PluginName {
	return types.ToolPoisoningPluginName
}
