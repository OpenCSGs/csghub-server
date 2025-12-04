package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
}

type llmServiceComponentImpl struct {
	llmConfigStore    database.LLMConfigStore
	promptPrefixStore database.PromptPrefixStore
}

func NewLLMServiceComponent(config *config.Config) (LLMServiceComponent, error) {
	llmConfigStore := database.NewLLMConfigStore(config)
	promptPrefixStore := database.NewPromptPrefixStore(config)
	llmServiceComp := &llmServiceComponentImpl{
		llmConfigStore:    llmConfigStore,
		promptPrefixStore: promptPrefixStore,
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
		ID:          dbLlmConfig.ID,
		ModelName:   dbLlmConfig.ModelName,
		ApiEndpoint: dbLlmConfig.ApiEndpoint,
		AuthHeader:  dbLlmConfig.AuthHeader,
		Type:        dbLlmConfig.Type,
		Enabled:     dbLlmConfig.Enabled,
		Provider:    dbLlmConfig.Provider,
		CreatedAt:   dbLlmConfig.CreatedAt,
		UpdatedAt:   dbLlmConfig.UpdatedAt,
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
	_, err = s.llmConfigStore.Update(ctx, *llmConfig)
	if err != nil {
		return nil, err
	}
	resLLMConfig := &types.LLMConfig{
		ID:          llmConfig.ID,
		ModelName:   llmConfig.ModelName,
		ApiEndpoint: llmConfig.ApiEndpoint,
		AuthHeader:  llmConfig.AuthHeader,
		Type:        llmConfig.Type,
		Enabled:     llmConfig.Enabled,
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
	dbLLMConfig := database.LLMConfig{
		ModelName:   req.ModelName,
		ApiEndpoint: req.ApiEndpoint,
		AuthHeader:  req.AuthHeader,
		Type:        req.Type,
		Enabled:     req.Enabled,
	}
	dbRes, err := s.llmConfigStore.Create(ctx, dbLLMConfig)
	if err != nil {
		return nil, err
	}
	resLLMConfig := &types.LLMConfig{
		ID:          dbRes.ID,
		ModelName:   dbLLMConfig.ModelName,
		ApiEndpoint: dbLLMConfig.ApiEndpoint,
		AuthHeader:  dbLLMConfig.AuthHeader,
		Type:        dbLLMConfig.Type,
		Enabled:     dbLLMConfig.Enabled,
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
