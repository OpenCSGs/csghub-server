package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/llm"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const IndustryRepoPromptPrefixKind types.PromptPrefixKind = "identify_industry_repo"

type IndustryTagComponent interface {
	RefreshRepoAutoIndustryTags(ctx context.Context, req types.IdentifyIndustryTagsReq) error
	ClearRepoAutoIndustryTags(ctx context.Context, req types.ClearRepoIndustryTagsReq) error
	IdentifyIndustryTags(ctx context.Context, req types.IdentifyIndustryTagsReq) (*types.IdentifyIndustryTagsResult, error)
}

type industryTagComponentImpl struct {
	repoStore         database.RepoStore
	tagStore          database.TagStore
	promptPrefixStore database.PromptPrefixStore
	llmConfigStore    database.LLMConfigStore
	gitServer         gitserver.GitServer
	llmClient         llm.LLMSvcClient
}

func NewIndustryTagComponent(cfg *config.Config) (IndustryTagComponent, error) {
	gs, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, err
	}

	return &industryTagComponentImpl{
		repoStore:         database.NewRepoStore(),
		tagStore:          database.NewTagStore(),
		promptPrefixStore: database.NewPromptPrefixStore(cfg),
		llmConfigStore:    database.NewLLMConfigStore(cfg),
		gitServer:         gs,
		llmClient:         llm.NewClient(),
	}, nil
}

func (c *industryTagComponentImpl) RefreshRepoAutoIndustryTags(ctx context.Context, req types.IdentifyIndustryTagsReq) error {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("find repo: %w", err)
	}

	readme := req.Readme
	if readme == "" {
		branch := req.Branch
		if branch == "" {
			branch = repo.DefaultBranch
		}
		if branch == "" {
			branch = types.MainBranch
		}
		readme, err = c.gitServer.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       branch,
			Path:      types.ReadmeFileName,
			RepoType:  req.RepoType,
		})
		if err != nil {
			return fmt.Errorf("get repo readme: %w", err)
		}
	}

	description := req.Description
	if description == "" {
		description = repo.Description
	}

	result, err := c.IdentifyIndustryTags(ctx, types.IdentifyIndustryTagsReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		RepoType:    req.RepoType,
		Branch:      req.Branch,
		Description: description,
		Readme:      readme,
	})
	if err != nil {
		return err
	}
	if len(result.TagIDs) == 0 {
		slog.Warn("no industry tags detected", slog.Any("reason", result.Reason))
	}

	return c.tagStore.ReplaceRepoTagsByCategoryAndSource(ctx, repo.ID, "industry", types.TagSourceAuto, result.TagIDs)
}

func (c *industryTagComponentImpl) ClearRepoAutoIndustryTags(ctx context.Context, req types.ClearRepoIndustryTagsReq) error {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("find repo: %w", err)
	}
	return c.tagStore.RemoveRepoTagsByCategoryAndSource(ctx, repo.ID, []string{"industry"}, types.TagSourceAuto)
}

func (c *industryTagComponentImpl) IdentifyIndustryTags(ctx context.Context, req types.IdentifyIndustryTagsReq) (*types.IdentifyIndustryTagsResult, error) {
	tagScope, err := industryTagScopeForRepoType(req.RepoType)
	if err != nil {
		return nil, err
	}

	builtIn := true
	allTags, err := c.tagStore.AllTags(ctx, &types.TagFilter{
		Scopes:     []types.TagScope{tagScope},
		Categories: []string{"industry"},
		BuiltIn:    &builtIn,
	})
	if err != nil {
		return nil, fmt.Errorf("load repo industry tags: %w", err)
	}
	if len(allTags) == 0 {
		return &types.IdentifyIndustryTagsResult{Reason: "no industry candidates configured"}, nil
	}

	description := strings.TrimSpace(req.Description)
	readme := strings.TrimSpace(req.Readme)
	if description == "" && isMetaOnlyReadme(readme) {
		return &types.IdentifyIndustryTagsResult{Reason: "no useful repo text found"}, nil
	}

	prompt, err := c.promptPrefixStore.Get(ctx, IndustryRepoPromptPrefixKind)
	if err != nil {
		return nil, fmt.Errorf("load industry prompt prefix: %w", err)
	}
	llmConfig, err := c.llmConfigStore.GetModelForSummaryReadme(ctx)
	if err != nil {
		return &types.IdentifyIndustryTagsResult{Reason: fmt.Sprintf("failed to load LLM config: %v", err)}, nil
	}

	var headers map[string]string
	if err := json.Unmarshal([]byte(llmConfig.AuthHeader), &headers); err != nil {
		return nil, fmt.Errorf("parse llm auth header: %w", err)
	}

	candidates := make([]string, 0, len(allTags))
	for _, tag := range allTags {
		candidates = append(candidates, tag.Name)
	}

	input, err := json.Marshal(map[string]any{
		"description": description,
		"readme":      readme,
		"candidates":  candidates,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal industry identify input: %w", err)
	}

	resp, err := c.llmClient.Chat(ctx, llmConfig.ApiEndpoint, "", headers, types.LLMReqBody{
		Model: llmConfig.ModelName,
		Messages: []types.LLMMessage{
			{Role: SystemRole, Content: prompt.ZH},
			{Role: UserRole, Content: string(input)},
		},
		Stream:      false,
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("identify repo industry tags: %w", err)
	}

	tagNames, reason, err := parseIndustryResponse(resp)
	if err != nil {
		return nil, err
	}

	allowed := make(map[string]int64, len(allTags))
	for _, candidate := range allTags {
		allowed[candidate.Name] = candidate.ID
	}

	filtered := make([]string, 0, len(tagNames))
	filteredIDs := make([]int64, 0, len(tagNames))
	seen := make(map[string]struct{}, len(tagNames))
	for _, tagName := range tagNames {
		tagID, ok := allowed[tagName]
		if !ok {
			continue
		}
		if _, ok := seen[tagName]; ok {
			continue
		}
		seen[tagName] = struct{}{}
		filtered = append(filtered, tagName)
		filteredIDs = append(filteredIDs, tagID)
	}

	return &types.IdentifyIndustryTagsResult{
		TagIDs:    filteredIDs,
		TagNames:  filtered,
		MatchedBy: "llm_candidates",
		Reason:    reason,
	}, nil
}

func industryTagScopeForRepoType(repoType types.RepositoryType) (types.TagScope, error) {
	switch repoType {
	case types.ModelRepo,
		types.DatasetRepo,
		types.CodeRepo,
		types.SpaceRepo,
		types.PromptRepo,
		types.MCPServerRepo,
		types.SkillRepo:
		return getTagScopeByRepoType(repoType), nil
	default:
		return "", fmt.Errorf("unsupported repo type for industry tags: %s", repoType)
	}
}

func parseIndustryResponse(resp string) ([]string, string, error) {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var tagNames []string
	if err := json.Unmarshal([]byte(resp), &tagNames); err == nil {
		return tagNames, "", nil
	}

	var payload struct {
		TagNames []string `json:"tag_names"`
		Reason   string   `json:"reason"`
	}
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		return nil, "", fmt.Errorf("parse industry llm response: %w", err)
	}
	return payload.TagNames, payload.Reason, nil
}

func isMetaOnlyReadme(readme string) bool {
	readme = strings.TrimSpace(readme)
	if readme == "" {
		return true
	}
	if !strings.HasPrefix(readme, "---") {
		return false
	}
	parts := strings.SplitN(readme, "---", 3)
	if len(parts) < 3 {
		return false
	}
	return strings.TrimSpace(parts[2]) == ""
}
