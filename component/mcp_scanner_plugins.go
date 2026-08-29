package component

import (
	"context"
	"encoding/json"

	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type MCPScannerPlugin interface {
	Check(ctx context.Context, files []*types.File) ([]types.ScannerIssue, error)
	GetName() types.PluginName
}

func summaryResult(ctx context.Context, llmClient *llm.Client, llmConfig *database.LLMConfig, summaryPrompt, result string, temperature float64) ([]types.ScannerIssue, error) {
	req := types.LLMReqBody{
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: summaryPrompt},
			{Role: UserRole, Content: result},
		},
		Stream:      false,
		Temperature: temperature,
	}
	summaryResult, err := llmClient.WithLLMConfig(llmConfig).Chat(ctx, "", "", nil, req)
	if err != nil {
		return nil, err
	}
	var scannerIssues []types.ScannerIssue
	err = json.Unmarshal([]byte(summaryResult), &scannerIssues)
	return scannerIssues, err
}
