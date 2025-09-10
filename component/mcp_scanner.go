package component

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"slices"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type MCPScannerComponent interface {
	Scan(ctx context.Context, namespace, name string) ([]types.ScannerIssue, error)
}

type mcpScannerComponentImpl struct {
	llmConfigStore database.LLMConfigStore
	promptStore    database.PromptPrefixStore
	gitServer      gitserver.GitServer
	plugins        []MCPScannerPlugin
	llmTemperature float64
}

func NewMCPScannerComponent(config *config.Config) MCPScannerComponent {
	if !config.MCPScan.Enable {
		return nil
	}
	gitServer, err := git.NewGitServer(config)
	if err != nil {
		return nil
	}
	plugins := []MCPScannerPlugin{}
	llmConfigStore := database.NewLLMConfigStore(config)
	promptStore := database.NewPromptPrefixStore(config)
	ctx := context.Background()

	scanner := &mcpScannerComponentImpl{
		llmConfigStore: llmConfigStore,
		promptStore:    promptStore,
		gitServer:      gitServer,
		plugins:        plugins,
		llmTemperature: config.MCPScan.Temperature,
	}
	// Initialize plugins
	pluginTypes := config.MCPScan.Plugins
	slog.Debug("mcp scan plugins", slog.Any("plugins", pluginTypes))
	scanner.parsePlugins(ctx, pluginTypes)
	return scanner
}

func (scanner *mcpScannerComponentImpl) parsePlugins(ctx context.Context, pluginNames []string) {
	summaryPrompt, err := scanner.promptStore.Get(ctx, types.PromptPrefixKind("mcp_scan_summary"))
	if err != nil {
		return
	}
	llmConfig, err := scanner.llmConfigStore.GetByType(ctx, 8)
	if err != nil {
		return
	}
	llmClinet := llm.NewClient()
	for _, pluginType := range pluginNames {
		var plugin MCPScannerPlugin
		switch pluginType {
		case "tool_poison":
			prompt, err := scanner.promptStore.Get(ctx, types.PromptPrefixKind("tool_poison"))
			if err != nil {
				return
			}
			plugin = newToolPoisoningPlugin(llmClinet, prompt, summaryPrompt, llmConfig, scanner.llmTemperature)
		default:
			return
		}
		scanner.plugins = append(scanner.plugins, plugin)
	}
}

func (scanner *mcpScannerComponentImpl) Scan(ctx context.Context, namespace, name string) ([]types.ScannerIssue, error) {
	req := gitserver.GetRepoAllFilesReq{
		Namespace: namespace,
		Name:      name,
		Ref:       "main",
		RepoType:  types.MCPServerRepo,
	}
	files, err := scanner.gitServer.GetRepoAllFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		req := gitserver.GetRepoInfoByPathReq{
			Namespace: namespace,
			Name:      name,
			Ref:       "main",
			Path:      file.Path,
			RepoType:  types.MCPServerRepo,
		}
		rawFile, err := scanner.gitServer.GetRepoFileRaw(ctx, req)
		if err != nil {
			return nil, err
		}
		file.Content = rawFile
	}
	var result []types.ScannerIssue
	for _, plugin := range scanner.plugins {
		issues, err := plugin.Check(ctx, files)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			if issue.Level == types.LevelCritical {
				result = append(result, issue)
			}
		}
	}
	return result, nil
}

func ignoreFile(file *types.File) bool {
	if strings.Contains(file.Path, ".git/") || file.Name == ".gitignore" {
		return true
	}
	if !isTextFileByExtension(file.Name) {
		return true
	}
	return false
}

func isTextFileByExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	// Check if extension is in the list of known extensions
	return slices.Contains(types.TextFileExtensions, ext)
}
