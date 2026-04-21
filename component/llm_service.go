package component

import (
	"context"
	"math"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

// LLMServiceComponent is an interface that defines methods for interacting with LLM configurations.
type LLMServiceComponent interface {
	// IndexLLMConfig retrieves a batch of LLM configurations.
	IndexLLMConfig(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*database.LLMConfig, int, error)
	IndexPromptPrefix(ctx context.Context, per, page int, search *types.SearchPromptPrefix) ([]*database.PromptPrefix, int, error)
	ShowLLMConfig(ctx context.Context, id int64) (*types.LLMConfig, error)
	ShowPromptConfig(ctx context.Context, id int64) (*types.PromptPrefix, error)
	CreateLLMConfig(ctx context.Context, req *types.CreateLLMConfigReq) (*types.LLMConfig, error)
	CreatePromptPrefix(ctx context.Context, req *types.CreatePromptPrefixReq) (*types.PromptPrefix, error)
	// UpdateLLMConfig updates the LLM configuration.
	UpdateLLMConfig(ctx context.Context, req *types.UpdateLLMConfigReq) (*types.LLMConfig, error)
	UpdatePromptPrefix(ctx context.Context, req *types.UpdatePromptPrefixReq) (*types.PromptPrefix, error)
	// DeleteLLMConfig deletes the LLM configuration by ID.
	DeleteLLMConfig(ctx context.Context, id int64) error
	// DeletePromptPrefix deletes the prompt prefix by ID.
	DeletePromptPrefix(ctx context.Context, id int64) error
	ListExternalLLMs(ctx context.Context) ([]*types.LLMConfig, error)
}

type llmServiceComponentImpl struct {
	llmConfigStore    database.LLMConfigStore
	promptPrefixStore database.PromptPrefixStore
	repoStore         database.RepoStore
}

func NewLLMServiceComponent(config *config.Config) (LLMServiceComponent, error) {
	llmConfigStore := database.NewLLMConfigStore(config)
	promptPrefixStore := database.NewPromptPrefixStore(config)
	repoStore := database.NewRepoStore()
	llmServiceComp := &llmServiceComponentImpl{
		llmConfigStore:    llmConfigStore,
		promptPrefixStore: promptPrefixStore,
		repoStore:         repoStore,
	}
	return llmServiceComp, nil
}

func (s *llmServiceComponentImpl) IndexLLMConfig(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*database.LLMConfig, int, error) {
	llmConfigs, total, err := s.llmConfigStore.Index(ctx, per, page, search)
	if err != nil {
		return nil, 0, err
	}

	return llmConfigs, total, nil
}

func (s *llmServiceComponentImpl) IndexPromptPrefix(ctx context.Context, per, page int, search *types.SearchPromptPrefix) ([]*database.PromptPrefix, int, error) {
	promptPrefixes, total, err := s.promptPrefixStore.Index(ctx, per, page, search)
	if err != nil {
		return nil, 0, err
	}
	return promptPrefixes, total, nil
}

func (s *llmServiceComponentImpl) ShowLLMConfig(ctx context.Context, id int64) (*types.LLMConfig, error) {
	dbLlmConfig, err := s.llmConfigStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	llmConfig := &types.LLMConfig{
		ID:           dbLlmConfig.ID,
		ModelName:    dbLlmConfig.ModelName,
		OfficialName: dbLlmConfig.OfficialName,
		ApiEndpoint:  dbLlmConfig.ApiEndpoint,
		AuthHeader:   dbLlmConfig.AuthHeader,
		Type:         dbLlmConfig.Type,
		Enabled:      dbLlmConfig.Enabled,
		Provider:     dbLlmConfig.Provider,
		Metadata:     dbLlmConfig.Metadata,
		RepoID:       dbLlmConfig.RepoID,
		CreatedAt:    dbLlmConfig.CreatedAt,
		UpdatedAt:    dbLlmConfig.UpdatedAt,
	}
	if dbLlmConfig.RepoID > 0 {
		repo, err := s.repoStore.FindById(ctx, dbLlmConfig.RepoID)
		if err == nil && repo != nil {
			llmConfig.Repo = &types.RepositoryLite{
				ID:          repo.ID,
				Path:        repo.Path,
				Name:        repo.Name,
				Nickname:    repo.Nickname,
				Description: repo.Description,
			}
		}
	}
	return llmConfig, nil
}

func (s *llmServiceComponentImpl) ShowPromptConfig(ctx context.Context, id int64) (*types.PromptPrefix, error) {
	dbPromptPrefix, err := s.promptPrefixStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	promptPrefix := &types.PromptPrefix{
		ID:   dbPromptPrefix.ID,
		ZH:   dbPromptPrefix.ZH,
		EN:   dbPromptPrefix.EN,
		Kind: dbPromptPrefix.Kind,
	}
	return promptPrefix, nil
}

func (s *llmServiceComponentImpl) UpdateLLMConfig(ctx context.Context, req *types.UpdateLLMConfigReq) (*types.LLMConfig, error) {
	llmConfig, err := s.llmConfigStore.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.ModelName != nil {
		llmConfig.ModelName = *req.ModelName
	}
	if req.OfficialName != nil {
		llmConfig.OfficialName = *req.OfficialName
	}
	if req.ApiEndpoint != nil {
		llmConfig.ApiEndpoint = *req.ApiEndpoint
	}
	if req.AuthHeader != nil {
		llmConfig.AuthHeader = *req.AuthHeader
	}
	if req.Type != nil {
		llmConfig.Type = *req.Type
	}
	if req.Enabled != nil {
		llmConfig.Enabled = *req.Enabled
	}
	if req.Provider != nil {
		llmConfig.Provider = *req.Provider
	}
	if req.Metadata != nil {
		commonutils.MergeMapWithDeletion(&llmConfig.Metadata, *req.Metadata)
	}
	if req.RepoID != nil {
		llmConfig.RepoID = *req.RepoID
	}
	updatedConfig, err := s.llmConfigStore.Update(ctx, *llmConfig)
	if err != nil {
		return nil, err
	}
	resLLMConfig := &types.LLMConfig{
		ID:           updatedConfig.ID,
		ModelName:    updatedConfig.ModelName,
		OfficialName: updatedConfig.OfficialName,
		ApiEndpoint:  updatedConfig.ApiEndpoint,
		AuthHeader:   updatedConfig.AuthHeader,
		Type:         updatedConfig.Type,
		Provider:     updatedConfig.Provider,
		Enabled:      updatedConfig.Enabled,
		Metadata:     updatedConfig.Metadata,
		RepoID:       updatedConfig.RepoID,
		CreatedAt:    updatedConfig.CreatedAt,
		UpdatedAt:    updatedConfig.UpdatedAt,
	}
	return resLLMConfig, nil
}

func (s *llmServiceComponentImpl) UpdatePromptPrefix(ctx context.Context, req *types.UpdatePromptPrefixReq) (*types.PromptPrefix, error) {
	promptPrefix, err := s.promptPrefixStore.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.ZH != nil {
		promptPrefix.ZH = *req.ZH
	}
	if req.EN != nil {
		promptPrefix.EN = *req.EN
	}
	if req.Kind != nil {
		promptPrefix.Kind = *req.Kind
	}
	_, err = s.promptPrefixStore.Update(ctx, *promptPrefix)
	if err != nil {
		return nil, err
	}
	resPromptPrefix := &types.PromptPrefix{
		ID:   promptPrefix.ID,
		ZH:   promptPrefix.ZH,
		EN:   promptPrefix.EN,
		Kind: promptPrefix.Kind,
	}
	return resPromptPrefix, nil
}

func (s *llmServiceComponentImpl) CreateLLMConfig(ctx context.Context, req *types.CreateLLMConfigReq) (*types.LLMConfig, error) {
	var repoID int64
	if req.RepoID != nil {
		repoID = *req.RepoID
	}
	dbLLMConfig := database.LLMConfig{
		ModelName:    req.ModelName,
		OfficialName: req.OfficialName,
		ApiEndpoint:  req.ApiEndpoint,
		AuthHeader:   req.AuthHeader,
		Type:         req.Type,
		Provider:     req.Provider,
		Enabled:      req.Enabled,
		Metadata:     req.Metadata,
		RepoID:       repoID,
	}
	dbRes, err := s.llmConfigStore.Create(ctx, dbLLMConfig)
	if err != nil {
		return nil, err
	}
	resLLMConfig := &types.LLMConfig{
		ID:           dbRes.ID,
		ModelName:    dbRes.ModelName,
		OfficialName: dbRes.OfficialName,
		ApiEndpoint:  dbRes.ApiEndpoint,
		AuthHeader:   dbRes.AuthHeader,
		Type:         dbRes.Type,
		Provider:     dbRes.Provider,
		Enabled:      dbRes.Enabled,
		Metadata:     dbRes.Metadata,
		RepoID:       dbRes.RepoID,
		CreatedAt:    dbRes.CreatedAt,
		UpdatedAt:    dbRes.UpdatedAt,
	}
	return resLLMConfig, nil
}

func (s *llmServiceComponentImpl) CreatePromptPrefix(ctx context.Context, req *types.CreatePromptPrefixReq) (*types.PromptPrefix, error) {
	dbPromptPrefix := database.PromptPrefix{
		ZH:   req.ZH,
		EN:   req.EN,
		Kind: req.Kind,
	}
	dbRes, err := s.promptPrefixStore.Create(ctx, dbPromptPrefix)
	if err != nil {
		return nil, err
	}
	resPromptPrefix := &types.PromptPrefix{
		ID:   dbRes.ID,
		ZH:   dbPromptPrefix.ZH,
		EN:   dbPromptPrefix.EN,
		Kind: dbPromptPrefix.Kind,
	}
	return resPromptPrefix, nil
}

func (s *llmServiceComponentImpl) DeleteLLMConfig(ctx context.Context, id int64) error {
	err := s.llmConfigStore.Delete(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
func (s *llmServiceComponentImpl) DeletePromptPrefix(ctx context.Context, id int64) error {
	err := s.promptPrefixStore.Delete(ctx, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *llmServiceComponentImpl) ListExternalLLMs(ctx context.Context) ([]*types.LLMConfig, error) {
	typeVal := database.LLMTypeAigatewayExternal
	enabled := true
	search := &types.SearchLLMConfig{
		Type:    &typeVal,
		Enabled: &enabled,
	}
	configs, _, err := s.llmConfigStore.IndexWithRepo(ctx, math.MaxInt, 1, search)
	if err != nil {
		return nil, err
	}
	var result []*types.LLMConfig
	for _, cfg := range configs {
		item := &types.LLMConfig{
			ID:           cfg.ID,
			ModelName:    cfg.ModelName,
			OfficialName: cfg.OfficialName,
			Type:         cfg.Type,
			Enabled:      cfg.Enabled,
			Provider:     cfg.Provider,
			RepoID:       cfg.RepoID,
			CreatedAt:    cfg.CreatedAt,
			UpdatedAt:    cfg.UpdatedAt,
		}
		if cfg.Repo != nil {
			tags, _ := s.repoStore.Tags(ctx, cfg.Repo.ID)
			var liteTags []types.RepoTag
			for _, t := range tags {
				liteTags = append(liteTags, types.RepoTag{
					Name:     t.Name,
					Category: t.Category,
					Group:    t.Group,
				})
			}
			item.Repo = &types.RepositoryLite{
				ID:          cfg.Repo.ID,
				Path:        cfg.Repo.Path,
				Name:        cfg.Repo.Name,
				Nickname:    cfg.Repo.Nickname,
				Description: cfg.Repo.Description,
				Tags:        liteTags,
			}
		}
		result = append(result, item)
	}
	return result, nil
}
