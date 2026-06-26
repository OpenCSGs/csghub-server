package component

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	aigatewaytypes "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// LLMServiceComponent is an interface that defines methods for interacting with LLM configurations.
type LLMServiceComponent interface {
	// IndexLLMConfig retrieves a batch of LLM configurations.
	IndexLLMConfig(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*types.LLMConfig, int, error)
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
	// CreateUpstream adds a new upstream to an existing LLM config.
	CreateUpstream(ctx context.Context, req *types.CreateUpstreamReq) (*types.UpstreamConfig, error)
	// UpdateUpstream updates an existing upstream by ID.
	UpdateUpstream(ctx context.Context, req *types.UpdateUpstreamReq) (*types.UpstreamConfig, error)
	// DeleteUpstream deletes an upstream by ID.
	DeleteUpstream(ctx context.Context, id int64) error
}

type llmServiceComponentImpl struct {
	llmConfigStore    database.LLMConfigStore
	upstreamStore     database.UpstreamStore
	promptPrefixStore database.PromptPrefixStore
	repoStore         database.RepoStore
	healthStateStore  database.AIGatewayUpstreamHealthStateStore
	circuitStateStore database.AIGatewayUpstreamCircuitStateStore
}

var ErrInvalidLLMConfig = errors.New("invalid llm config")

func NewLLMServiceComponent(config *config.Config) (LLMServiceComponent, error) {
	llmConfigStore := database.NewLLMConfigStore(config)
	promptPrefixStore := database.NewPromptPrefixStore(config)
	repoStore := database.NewRepoStore()
	upstreamStore := database.NewUpstreamStore(config)
	healthStateStore := database.NewAIGatewayUpstreamHealthStateStore()
	circuitStateStore := database.NewAIGatewayUpstreamCircuitStateStore()
	llmServiceComp := &llmServiceComponentImpl{
		llmConfigStore:    llmConfigStore,
		upstreamStore:     upstreamStore,
		promptPrefixStore: promptPrefixStore,
		repoStore:         repoStore,
		healthStateStore:  healthStateStore,
		circuitStateStore: circuitStateStore,
	}
	return llmServiceComp, nil
}

func (s *llmServiceComponentImpl) IndexLLMConfig(ctx context.Context, per, page int, search *types.SearchLLMConfig) ([]*types.LLMConfig, int, error) {
	dbLLMConfigs, total, err := s.llmConfigStore.Index(ctx, per, page, search)
	if err != nil {
		return nil, 0, err
	}

	llmConfigs := make([]*types.LLMConfig, 0, len(dbLLMConfigs))
	for _, cfg := range dbLLMConfigs {
		upstreams := buildUpstreamConfigs(cfg.Upstreams)
		isAvailable, reason := computeLLMAvailability(upstreams)
		llmConfigs = append(llmConfigs, &types.LLMConfig{
			ID:                 cfg.ID,
			ModelName:          cfg.ModelName,
			OfficialName:       cfg.PrimaryOfficialName(),
			Upstreams:          upstreams,
			Type:               cfg.Type,
			Enabled:            cfg.Enabled,
			RoutingPolicy:      cfg.RoutingPolicy,
			Metadata:           cfg.Metadata,
			RepoID:             cfg.RepoID,
			NeedSensitiveCheck: cfg.NeedSensitiveCheck,
			ModelSizeB:         cfg.ModelSizeB,
			IsAvailable:        cfg.Enabled && isAvailable,
			AvailabilityReason: reason,
			CreatedAt:          cfg.CreatedAt,
			UpdatedAt:          cfg.UpdatedAt,
		})
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
	// Build upstream configs from relational upstreams with health/circuit state
	upstreams := buildUpstreamConfigs(dbLlmConfig.Upstreams)
	// Derive deprecated API-level fields from the first enabled upstream
	llmConfig := &types.LLMConfig{
		ID:                 dbLlmConfig.ID,
		ModelName:          dbLlmConfig.ModelName,
		Upstreams:          upstreams,
		Type:               dbLlmConfig.Type,
		Enabled:            dbLlmConfig.Enabled,
		RoutingPolicy:      dbLlmConfig.RoutingPolicy,
		Metadata:           dbLlmConfig.Metadata,
		RepoID:             dbLlmConfig.RepoID,
		NeedSensitiveCheck: dbLlmConfig.NeedSensitiveCheck,
		ModelSizeB:         dbLlmConfig.ModelSizeB,
		CreatedAt:          dbLlmConfig.CreatedAt,
		UpdatedAt:          dbLlmConfig.UpdatedAt,
	}
	isAvailable, reason := computeLLMAvailability(upstreams)
	llmConfig.IsAvailable = llmConfig.Enabled && isAvailable
	llmConfig.AvailabilityReason = reason
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
		modelName, err := normalizeRequiredLLMModelName(*req.ModelName)
		if err != nil {
			return nil, err
		}
		llmConfig.ModelName = modelName
	}
	if req.Type != nil {
		llmConfig.Type = *req.Type
	}
	if req.Enabled != nil {
		llmConfig.Enabled = *req.Enabled
	}
	if req.RoutingPolicy != nil {
		llmConfig.RoutingPolicy = *req.RoutingPolicy
	}
	if req.Metadata != nil {
		llmConfig.Metadata = *req.Metadata
	}
	if req.RepoID != nil {
		llmConfig.RepoID = *req.RepoID
	}
	if req.NeedSensitiveCheck != nil {
		llmConfig.NeedSensitiveCheck = *req.NeedSensitiveCheck
	}
	if req.ModelSizeB != nil {
		llmConfig.ModelSizeB = *req.ModelSizeB
	}
	updatedConfig, updateErr := s.llmConfigStore.Update(ctx, *llmConfig)
	if updateErr != nil {
		return nil, updateErr
	}
	// Re-read upstreams from DB to include IDs and any DB-side defaults
	dbUpstreams, err := s.upstreamStore.ListByLLMConfigID(ctx, updatedConfig.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to read upstreams: %w", err)
	}
	upstreams := buildUpstreamConfigs(upstreamPtrsToValues(dbUpstreams))
	resLLMConfig := &types.LLMConfig{
		ID:                 updatedConfig.ID,
		ModelName:          updatedConfig.ModelName,
		Upstreams:          upstreams,
		Type:               updatedConfig.Type,
		Enabled:            updatedConfig.Enabled,
		RoutingPolicy:      updatedConfig.RoutingPolicy,
		Metadata:           updatedConfig.Metadata,
		RepoID:             updatedConfig.RepoID,
		NeedSensitiveCheck: updatedConfig.NeedSensitiveCheck,
		ModelSizeB:         updatedConfig.ModelSizeB,
		CreatedAt:          updatedConfig.CreatedAt,
		UpdatedAt:          updatedConfig.UpdatedAt,
	}
	isAvailable, reason := computeLLMAvailability(upstreams)
	resLLMConfig.IsAvailable = resLLMConfig.Enabled && isAvailable
	resLLMConfig.AvailabilityReason = reason
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
	modelName, err := normalizeRequiredLLMModelName(req.ModelName)
	if err != nil {
		return nil, err
	}
	if err := s.validateLLMEndpointConfig(req.Upstreams); err != nil {
		return nil, err
	}
	dbLLMConfig := database.LLMConfig{
		ModelName:          modelName,
		Type:               req.Type,
		Enabled:            req.Enabled,
		RoutingPolicy:      req.RoutingPolicy,
		Metadata:           req.Metadata,
		RepoID:             repoID,
		NeedSensitiveCheck: req.NeedSensitiveCheck,
		ModelSizeB:         req.ModelSizeB,
	}
	dbRes, err := s.llmConfigStore.Create(ctx, dbLLMConfig)
	if err != nil {
		return nil, err
	}
	// Create upstreams in the relational table (bun auto-fills ID after insert)
	createdUpstreams := make([]database.Upstream, 0, len(req.Upstreams))
	for _, u := range req.Upstreams {
		if strings.TrimSpace(u.URL) == "" {
			continue
		}
		dbUp := &database.Upstream{
			LLMConfigID:           dbRes.ID,
			URL:                   strings.TrimSpace(u.URL),
			Weight:                u.Weight,
			Enabled:               u.Enabled,
			ModelName:             strings.TrimSpace(u.ModelName),
			AuthHeader:            u.AuthHeader,
			Provider:              strings.TrimSpace(u.Provider),
			HealthCheckEnabled:    u.HealthCheckEnabled,
			CircuitBreakerEnabled: u.CircuitBreakerEnabled,
			Tags:                  u.Tags,
			LimitPolicy:           u.LimitPolicy,
		}
		if dbUp.Weight <= 0 {
			dbUp.Weight = 1
		}
		if err := s.upstreamStore.Create(ctx, dbUp); err != nil {
			return nil, fmt.Errorf("create upstream: %w", err)
		} else {
			createdUpstreams = append(createdUpstreams, *dbUp)
		}
	}
	upstreams := buildUpstreamConfigs(createdUpstreams)
	resLLMConfig := &types.LLMConfig{
		ID:                 dbRes.ID,
		ModelName:          dbRes.ModelName,
		Upstreams:          upstreams,
		Type:               dbRes.Type,
		Enabled:            dbRes.Enabled,
		RoutingPolicy:      dbRes.RoutingPolicy,
		Metadata:           dbRes.Metadata,
		RepoID:             dbRes.RepoID,
		NeedSensitiveCheck: dbRes.NeedSensitiveCheck,
		ModelSizeB:         dbRes.ModelSizeB,
		CreatedAt:          dbRes.CreatedAt,
		UpdatedAt:          dbRes.UpdatedAt,
	}
	isAvailable, reason := computeLLMAvailability(upstreams)
	resLLMConfig.IsAvailable = resLLMConfig.Enabled && isAvailable
	resLLMConfig.AvailabilityReason = reason
	return resLLMConfig, nil
}

func normalizeRequiredLLMModelName(modelName string) (string, error) {
	trimmed := strings.TrimSpace(modelName)
	if trimmed == "" {
		return "", fmt.Errorf("%w: model_name cannot be empty", ErrInvalidLLMConfig)
	}
	return trimmed, nil
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
	// Clean up relational upstreams
	_ = s.upstreamStore.DeleteByLLMConfigID(ctx, id)
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

func (s *llmServiceComponentImpl) validateLLMEndpointConfig(upstreams []types.UpstreamConfig) error {
	if len(upstreams) == 0 {
		return fmt.Errorf("%w: upstreams must be provided", ErrInvalidLLMConfig)
	}
	enabledCount := 0
	for _, upstream := range upstreams {
		if strings.TrimSpace(upstream.URL) == "" {
			return fmt.Errorf("%w: upstream url cannot be empty", ErrInvalidLLMConfig)
		}
		if upstream.Enabled {
			enabledCount++
		}
	}
	if len(upstreams) > 0 && enabledCount == 0 {
		return fmt.Errorf("%w: at least one enabled upstream must be provided", ErrInvalidLLMConfig)
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
			OfficialName: cfg.PrimaryOfficialName(),
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

func (s *llmServiceComponentImpl) CreateUpstream(ctx context.Context, req *types.CreateUpstreamReq) (*types.UpstreamConfig, error) {
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("%w: upstream url cannot be empty", ErrInvalidLLMConfig)
	}
	// Verify the LLM config exists
	_, err := s.llmConfigStore.GetByID(ctx, req.LLMConfigID)
	if err != nil {
		return nil, fmt.Errorf("llm config not found: %w", err)
	}
	dbUp := &database.Upstream{
		LLMConfigID:           req.LLMConfigID,
		URL:                   strings.TrimSpace(req.URL),
		Weight:                req.Weight,
		Enabled:               req.Enabled,
		ModelName:             strings.TrimSpace(req.ModelName),
		AuthHeader:            req.AuthHeader,
		Provider:              strings.TrimSpace(req.Provider),
		HealthCheckEnabled:    req.HealthCheckEnabled,
		CircuitBreakerEnabled: req.CircuitBreakerEnabled,
		Tags:                  req.Tags,
		LimitPolicy:           req.LimitPolicy,
	}
	if dbUp.Weight <= 0 {
		dbUp.Weight = 1
	}
	if err := s.upstreamStore.Create(ctx, dbUp); err != nil {
		return nil, fmt.Errorf("failed to create upstream: %w", err)
	}
	res := buildUpstreamConfigs([]database.Upstream{*dbUp})
	return &res[0], nil
}

func (s *llmServiceComponentImpl) UpdateUpstream(ctx context.Context, req *types.UpdateUpstreamReq) (*types.UpstreamConfig, error) {
	dbUp, err := s.upstreamStore.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("upstream not found: %w", err)
	}
	// Snapshot old state to detect transitions
	wasEnabled := dbUp.Enabled
	wasHealthCheckEnabled := dbUp.HealthCheckEnabled
	wasCircuitBreakerEnabled := dbUp.CircuitBreakerEnabled

	// Apply partial updates
	if req.URL != nil {
		dbUp.URL = strings.TrimSpace(*req.URL)
	}
	if req.Weight != nil {
		dbUp.Weight = *req.Weight
	}
	if req.Enabled != nil {
		dbUp.Enabled = *req.Enabled
	}
	if req.ModelName != nil {
		dbUp.ModelName = strings.TrimSpace(*req.ModelName)
	}
	if req.AuthHeader != nil {
		dbUp.AuthHeader = *req.AuthHeader
	}
	if req.Provider != nil {
		dbUp.Provider = strings.TrimSpace(*req.Provider)
	}
	if req.HealthCheckEnabled != nil {
		dbUp.HealthCheckEnabled = *req.HealthCheckEnabled
	}
	if req.CircuitBreakerEnabled != nil {
		dbUp.CircuitBreakerEnabled = *req.CircuitBreakerEnabled
	}
	if req.LimitPolicy != nil {
		dbUp.LimitPolicy = *req.LimitPolicy
	}
	if req.Tags != nil {
		dbUp.Tags = *req.Tags
	}
	if dbUp.Weight <= 0 {
		dbUp.Weight = 1
	}
	if err := s.upstreamStore.Update(ctx, dbUp); err != nil {
		return nil, fmt.Errorf("failed to update upstream: %w", err)
	}

	// When upstream or check is re-enabled, reset stale state to unknown.
	// Existing DB records from the previous active period are outdated,
	// but we preserve them by updating rather than deleting.
	if dbUp.Enabled {
		if (!wasEnabled || !wasHealthCheckEnabled) && dbUp.HealthCheckEnabled {
			_ = s.resetHealthStateToUnknown(ctx, dbUp.ID)
		}
		if (!wasEnabled || !wasCircuitBreakerEnabled) && dbUp.CircuitBreakerEnabled {
			_ = s.resetCircuitStateToUnknown(ctx, dbUp.ID)
		}
	}

	// Re-read upstream to pick up refreshed health/circuit state
	dbUp, err = s.upstreamStore.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload upstream: %w", err)
	}
	res := buildUpstreamConfigs([]database.Upstream{*dbUp})
	return &res[0], nil
}

// resetHealthStateToUnknown resets the health state of an upstream to unknown
// if a DB record exists. No-op if no record exists (buildUpstreamConfigs
// will treat nil as unknown).
func (s *llmServiceComponentImpl) resetHealthStateToUnknown(ctx context.Context, upstreamID int64) error {
	state, err := s.healthStateStore.GetByUpstreamID(ctx, upstreamID)
	if err != nil {
		// No record exists, nothing to reset
		return nil
	}
	state.HealthState = string(aigatewaytypes.HealthStateUnknown)
	return s.healthStateStore.Update(ctx, state)
}

// resetCircuitStateToUnknown resets the circuit state of an upstream to unknown
// if a DB record exists. No-op if no record exists (buildUpstreamConfigs
// will treat nil as unknown).
func (s *llmServiceComponentImpl) resetCircuitStateToUnknown(ctx context.Context, upstreamID int64) error {
	state, err := s.circuitStateStore.GetByUpstreamID(ctx, upstreamID)
	if err != nil {
		// No record exists, nothing to reset
		return nil
	}
	state.CircuitState = string(aigatewaytypes.CircuitStateUnknown)
	return s.circuitStateStore.Update(ctx, state)
}

func (s *llmServiceComponentImpl) DeleteUpstream(ctx context.Context, id int64) error {
	return s.upstreamStore.Delete(ctx, id)
}

// upstreamPtrsToValues converts a slice of upstream pointers to a slice of values.
func upstreamPtrsToValues(ptrs []*database.Upstream) []database.Upstream {
	result := make([]database.Upstream, 0, len(ptrs))
	for _, p := range ptrs {
		result = append(result, *p)
	}
	return result
}

// buildUpstreamConfigs converts database.Upstream slice to types.UpstreamConfig slice.
func buildUpstreamConfigs(dbUpstreams []database.Upstream) []types.UpstreamConfig {
	result := make([]types.UpstreamConfig, 0, len(dbUpstreams))
	for _, u := range dbUpstreams {
		uc := types.UpstreamConfig{
			ID:                    u.ID,
			URL:                   u.URL,
			Weight:                u.Weight,
			Enabled:               u.Enabled,
			ModelName:             u.ModelName,
			AuthHeader:            u.AuthHeader,
			Provider:              u.Provider,
			HealthCheckEnabled:    u.HealthCheckEnabled,
			CircuitBreakerEnabled: u.CircuitBreakerEnabled,
			Tags:                  u.Tags,
			LimitPolicy:           u.LimitPolicy,
		}
		if u.HealthState != nil {
			uc.HealthState = u.HealthState.HealthState
		} else if u.HealthCheckEnabled {
			// No health check record yet, treat as unknown until first probe
			uc.HealthState = string(aigatewaytypes.HealthStateUnknown)
		}
		if u.CircuitState != nil {
			uc.CircuitState = u.CircuitState.CircuitState
		} else if u.CircuitBreakerEnabled {
			// No circuit state record yet, treat as unknown until first probe
			uc.CircuitState = string(aigatewaytypes.CircuitStateUnknown)
		}
		uc.IsAvailable, _ = computeUpstreamAvailability(uc)
		uc.AvailabilityStatus = computeUpstreamAvailabilityStatus(uc)
		result = append(result, uc)
	}
	return result
}

func computeUpstreamAvailability(u types.UpstreamConfig) (bool, string) {
	if !u.Enabled {
		return false, aigatewaytypes.ReasonUpstreamDisabled
	}
	// When health check or circuit breaker is enabled but state is unknown
	// (not yet probed), treat upstream as unavailable to avoid misleading users.
	if u.CircuitBreakerEnabled && u.CircuitState == string(aigatewaytypes.CircuitStateUnknown) {
		return false, aigatewaytypes.ReasonCircuitStateUnknown
	}
	if u.HealthCheckEnabled && u.HealthState == string(aigatewaytypes.HealthStateUnknown) {
		return false, aigatewaytypes.ReasonHealthStateUnknown
	}
	if unavailable, reason := aigatewaytypes.IsUpstreamUnavailable(u); unavailable {
		return false, reason
	}
	return true, ""
}

func computeUpstreamAvailabilityStatus(u types.UpstreamConfig) string {
	if !u.Enabled {
		return string(aigatewaytypes.UpstreamStatusDisabled)
	}
	// When health check or circuit breaker is enabled but state is unknown
	// (not yet probed), return unknown status to reflect real state.
	if u.CircuitBreakerEnabled && u.CircuitState == string(aigatewaytypes.CircuitStateUnknown) {
		return string(aigatewaytypes.UpstreamStatusUnknown)
	}
	if u.HealthCheckEnabled && u.HealthState == string(aigatewaytypes.HealthStateUnknown) {
		return string(aigatewaytypes.UpstreamStatusUnknown)
	}
	if u.CircuitBreakerEnabled && u.CircuitState == string(aigatewaytypes.CircuitStateOpen) {
		return string(aigatewaytypes.UpstreamStatusUnavailable)
	}
	if u.HealthCheckEnabled && u.HealthState == string(aigatewaytypes.HealthStateUnhealthy) {
		return string(aigatewaytypes.UpstreamStatusUnavailable)
	}
	if u.HealthCheckEnabled && u.HealthState == string(aigatewaytypes.HealthStateDegraded) {
		return string(aigatewaytypes.UpstreamStatusDegraded)
	}
	return string(aigatewaytypes.UpstreamStatusAvailable)
}

func computeLLMAvailability(upstreams []types.UpstreamConfig) (bool, string) {
	if len(upstreams) == 0 {
		return true, ""
	}
	for _, upstream := range upstreams {
		if available, _ := computeUpstreamAvailability(upstream); available {
			return true, ""
		}
	}
	return false, aigatewaytypes.ReasonAllUpstreamsUnavailable
}
