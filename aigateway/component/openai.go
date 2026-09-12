package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/component/router"
	"opencsg.com/csghub-server/aigateway/component/upstream"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

var apiKeyJSONFieldRegex = regexp.MustCompile(`"api_key"\s*:\s*"[^"]*"`)

type OpenAIComponent interface {
	GetAvailableModels(c context.Context, nsUUID string) ([]types.Model, error)
	ListModels(c context.Context, nsUUID string, req types.ListModelsReq) (types.ModelList, error)
	GetModelByID(c context.Context, nsUUID, modelID string) (*types.Model, error)
	RecordUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, tokenCounter token.Counter, apikey string) error
	RecordUsageFromTokenUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error
	BuildUsageMeteringEvent(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) (*commontypes.MeteringEvent, error)
	CheckBalance(ctx context.Context, nsUUID string) error
	CheckUsageLimit(ctx context.Context, userUUID string, model *types.Model, endpoint string) error
	CommitUsageLimit(ctx context.Context, userUUID string, model *types.Model, tokenCounter token.Counter) error
	CommitUsageLimitFromUsage(ctx context.Context, userUUID string, model *types.Model, usage *token.Usage) error
	// CanManageModel reports whether the user can manage the given model
	// (e.g. upload or delete voices of a TTS deployment): only the deploy
	// owner and platform admins are allowed.
	CanManageModel(ctx context.Context, username, nsUUID string, model *types.Model) (bool, error)
}

type openaiComponentImpl struct {
	userStore      database.UserStore
	organStore     database.OrgStore
	deployStore    database.DeployTaskStore
	eventPub       *event.EventPublisher
	extllmStore    database.LLMConfigStore
	modelListCache cache.RedisClient
	extendOpenai
	modelIDBuilder upstream.ModelIDBuilder
	usageLimiter   UsageLimiter
}

func (m *openaiComponentImpl) getModelIDBuilder() upstream.ModelIDBuilder {
	if m.modelIDBuilder == nil {
		return upstream.NewModelIDBuilder()
	}
	return m.modelIDBuilder
}

func (m *openaiComponentImpl) getUsageLimiter() UsageLimiter {
	if m.usageLimiter == nil {
		return NewUsageLimiter(m.modelListCache)
	}
	return m.usageLimiter
}

// GetAvailableModels returns all enabled models from the llm_config table.
// Both internal (csghub deploy) and external models are served uniformly from
// llm_config, which loads its upstreams as a relation. Visibility filtering
// is applied per-model: internal non-serverless models are only visible to the
// deploy owner; serverless and external models are visible to all users.
func (m *openaiComponentImpl) GetAvailableModels(c context.Context, nsUUID string) ([]types.Model, error) {
	models, err := m.getModelsFromLLMConfig(c, nsUUID)
	if err != nil {
		return nil, err
	}

	models = m.enrichModelsWithPrice(c, models)

	if strings.TrimSpace(nsUUID) != "" {
		req := &types.UserPreferenceRequest{
			UserUUID: nsUUID,
			Models:   models,
			Scenario: types.AgenticHubApp,
		}
		var prefErr error
		models, prefErr = m.userPreference(c, req)
		if prefErr != nil {
			slog.Warn("failed to apply user preference", "error", prefErr)
		}
	}

	return models, nil
}

func (m *openaiComponentImpl) ListModels(c context.Context, nsUUID string, req types.ListModelsReq) (types.ModelList, error) {
	models, err := m.GetAvailableModels(c, nsUUID)
	if err != nil {
		return types.ModelList{}, err
	}
	modelList := filterAndPaginateModels(models, req)
	computeModelListAvailability(&modelList)
	return modelList, nil
}

// computeModelListAvailability sets ModelAvailability.IsAvailable for each model
// based on the already-loaded upstream health/circuit state (no extra DB call).
func computeModelListAvailability(modelList *types.ModelList) {
	if modelList == nil || len(modelList.Data) == 0 {
		return
	}
	for idx := range modelList.Data {
		model := &modelList.Data[idx]
		available := modelUpstreamsAvailable(model.Upstreams)
		model.Availability = &types.ModelAvailability{
			IsAvailable: available,
		}
	}
}

// modelUpstreamsAvailable returns true if at least one upstream is not unavailable
// based on the inline health/circuit state carried on UpstreamConfig.
func modelUpstreamsAvailable(upstreams []commontypes.UpstreamConfig) bool {
	if len(upstreams) == 0 {
		// No upstreams configured: model is available (legacy behavior, uses model.Endpoint directly).
		return true
	}
	for _, u := range upstreams {
		if unavailable, _ := types.IsUpstreamUnavailable(u); !unavailable {
			return true
		}
	}
	return false
}

type modelFilter func(m *types.Model) bool

func filterByModelID(query string) modelFilter {
	return func(m *types.Model) bool {
		return strings.Contains(strings.ToLower(m.ID), query)
	}
}

func filterByLLMTypes(llmTypes []string) modelFilter {
	allowedTypes := parseLLMTypes(llmTypes)
	return func(m *types.Model) bool {
		if len(allowedTypes) == 0 {
			return true
		}
		if m == nil || m.Metadata == nil {
			return false
		}
		llmType, _ := m.Metadata[types.MetaKeyLLMType].(string)
		return allowedTypes[strings.ToLower(strings.TrimSpace(llmType))]
	}
}

func parseLLMTypes(llmTypes []string) map[string]bool {
	allowedTypes := make(map[string]bool)
	for _, rawType := range llmTypes {
		llmType := strings.ToLower(strings.TrimSpace(rawType))
		if llmType == "" {
			continue
		}
		allowedTypes[llmType] = true
	}
	return allowedTypes
}

func filterByTask(task string) modelFilter {
	return func(m *types.Model) bool {
		modelTasks := strings.FieldsFunc(strings.ToLower(m.Task), func(r rune) bool {
			return r == ','
		})
		return slices.Contains(modelTasks, task)
	}
}

func filterByHasAssociatedModel(hasAssociatedModel bool) modelFilter {
	return func(m *types.Model) bool {
		hasRepoPath := false
		if m != nil && m.Metadata != nil {
			repoPath, _ := m.Metadata[types.MetaKeyRepoPath].(string)
			hasRepoPath = strings.TrimSpace(repoPath) != ""
		}
		return hasRepoPath == hasAssociatedModel
	}
}

func applyFilters(models []types.Model, filters []modelFilter) []types.Model {
	if len(filters) == 0 {
		return models
	}
	filtered := make([]types.Model, 0, len(models))
	for i := range models {
		m := &models[i]
		keep := true
		for _, f := range filters {
			if !f(m) {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, *m)
		}
	}
	return filtered
}

func filterAndPaginateModels(models []types.Model, req types.ListModelsReq) types.ModelList {
	filters := modelListDefaultFilters()

	if searchQuery := strings.ToLower(req.ModelID); searchQuery != "" {
		filters = append(filters, filterByModelID(searchQuery))
	}
	if len(req.LLMTypes) > 0 {
		filters = append(filters, filterByLLMTypes(req.LLMTypes))
	}
	if task := strings.ToLower(req.Task); task != "" {
		filters = append(filters, filterByTask(task))
	}
	if req.HasAssociatedModel != nil {
		filters = append(filters, filterByHasAssociatedModel(*req.HasAssociatedModel))
	}

	models = applyFilters(models, filters)

	totalCount := len(models)
	paginated := models
	hasMore := false

	if req.Per > 0 && req.Page > 0 {
		per := req.Per
		if per > 100 {
			per = 100
		}

		startIndex := (req.Page - 1) * per
		if startIndex > totalCount {
			startIndex = totalCount
		}
		endIndex := startIndex + per
		if endIndex > totalCount {
			endIndex = totalCount
		}

		paginated = models[startIndex:endIndex]
		hasMore = endIndex < totalCount
	}

	var firstID, lastID *string
	if len(paginated) > 0 {
		firstID = &paginated[0].ID
		lastID = &paginated[len(paginated)-1].ID
	}

	return types.ModelList{
		Object:     "list",
		Data:       paginated,
		FirstID:    firstID,
		LastID:     lastID,
		HasMore:    hasMore,
		TotalCount: totalCount,
	}
}

// buildInternalModel converts a csghub-sourced llm_config into a types.Model.
func (m *openaiComponentImpl) buildInternalModel(cfg *database.LLMConfig, info *commontypes.InternalModelInfo, upstreams []commontypes.UpstreamConfig) types.Model {
	modelID := info.LegacyModelID

	supportFunctionCall := commontypes.EngineArgToolCallingEnabled(info.EngineArgs, info.RuntimeFramework)
	model := types.Model{
		BaseModel: types.BaseModel{
			Object:              "model",
			ID:                  modelID,
			Created:             info.CreatedAt,
			SupportFunctionCall: supportFunctionCall,
			Task:                info.Task,
			Metadata: map[string]any{
				types.MetaKeyLLMType:  commontypes.ProviderTypeFromDeployType(info.SvcType),
				types.MetaKeyRepoPath: info.CSGHubModelID,
			},
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID:    info.CSGHubModelID,
			LegacyModelID:    info.LegacyModelID,
			OwnerUUID:        info.OwnerUUID,
			OwnerUsername:    info.OwnerUsername,
			OwnerNamespace:   info.OwnerNamespace,
			OwnerType:        info.OwnerType,
			ClusterID:        info.ClusterID,
			SvcName:          info.SvcName,
			SvcType:          info.SvcType,
			ImageID:          info.ImageID,
			RuntimeFramework: info.RuntimeFramework,
			EngineArgs:       info.EngineArgs,
			SourceDeployID:   info.SourceDeployID,
			CreatedAt:        info.CreatedAt,
			Host:             info.Host,
		},
		ExternalModelInfo: types.ExternalModelInfo{
			NeedSensitiveCheck: cfg.NeedSensitiveCheck,
		},
		Endpoint:      router.FirstEnabledUpstream(upstreams),
		Upstreams:     upstreams,
		RoutingPolicy: cfg.RoutingPolicy,
	}
	model.OwnedBy = m.getModelIDBuilder().GetModelOwner(info.SvcType, info.OwnerUsername)
	return model
}

// buildExternalModel converts an external llm_config into a types.Model.
func (m *openaiComponentImpl) buildExternalModel(ctx context.Context, cfg *database.LLMConfig, upstreams []commontypes.UpstreamConfig) types.Model {
	metadata := maps.Clone(cfg.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}

	task := ""
	if tasks, ok := metadata[types.MetaKeyTasks].([]any); ok && len(tasks) > 0 {
		tasksStrings := make([]string, 0, len(tasks))
		for _, t := range tasks {
			if s, ok := t.(string); ok {
				tasksStrings = append(tasksStrings, s)
			}
		}
		task = strings.Join(tasksStrings, ",")
	}

	if cfg.RepoID != 0 {
		if cfg.Repo != nil && cfg.Repo.Path != "" {
			metadata[types.MetaKeyRepoPath] = cfg.Repo.Path
		} else {
			slog.WarnContext(ctx, "llm config repo relation unavailable", "llm_config_id", cfg.ID, "repo_id", cfg.RepoID)
		}
	}
	metadata[types.MetaKeyLLMType] = commontypes.ProviderTypeExternalLLM

	provider := cfg.PrimaryProvider()
	model := types.Model{
		BaseModel: types.BaseModel{
			Object:   "model",
			ID:       cfg.ModelName,
			OwnedBy:  provider,
			Metadata: metadata,
			Task:     task,
		},
		Endpoint:      router.FirstEnabledUpstream(upstreams),
		Upstreams:     upstreams,
		RoutingPolicy: cfg.RoutingPolicy,
		ExternalModelInfo: types.ExternalModelInfo{
			Provider:           provider,
			AuthHead:           cfg.PrimaryAuthHeader(),
			NeedSensitiveCheck: cfg.NeedSensitiveCheck,
		},
	}
	return model
}

// llmConfigToModel converts a single LLMConfig into a types.Model.
// The model type (internal vs external) is determined by the upstream's Source
// field, not by the presence of InternalModelInfo.
//
// Returns (model, true) for a valid model, or (zero, false) to skip:
//   - A csghub upstream without InternalModelInfo is skipped (with a warning).
//   - When applyVisibility is true, internal non-serverless models whose
//     OwnerUUID does not match callerUUID are skipped.
func (m *openaiComponentImpl) llmConfigToModel(ctx context.Context, cfg *database.LLMConfig, callerUUID string, applyVisibility bool) (types.Model, bool) {
	upstreams := dbUpstreamsToConfigs(cfg.Upstreams)

	for _, u := range cfg.Upstreams {
		if u.Source != commontypes.UpstreamSourceCSGHubDeploy {
			continue
		}
		// Internal (csghub deploy) model.
		if u.Metadata == nil || u.Metadata.InternalModelInfo == nil {
			slog.WarnContext(ctx, "skip csghub upstream without internal model info", "upstream_id", u.ID)
			return types.Model{}, false
		}
		info := u.Metadata.InternalModelInfo

		if applyVisibility && info.SvcType != commontypes.ServerlessType && info.OwnerUUID != callerUUID {
			return types.Model{}, false
		}

		return m.buildInternalModel(cfg, info, upstreams), true
	}

	// No csghub upstream found — external model.
	return m.buildExternalModel(ctx, cfg, upstreams), true
}

// getModelsFromLLMConfig reads all enabled llm_configs from the database
// and converts them to types.Model. This is the unified read path — both
// internal (csghub deploy) and external models are stored in llm_config
// with their upstreams as a relation.
//
// Visibility rules for internal (csghub-sourced) models:
//   - Serverless deploys (SvcType == ServerlessType) are visible to all users.
//   - Non-serverless deploys are only visible to the deploy owner (OwnerUUID == callerUUID).
//
// External models are always visible.
func (m *openaiComponentImpl) getModelsFromLLMConfig(ctx context.Context, callerUUID string) ([]types.Model, error) {
	enabled := true
	search := &commontypes.SearchLLMConfig{
		Enabled:   &enabled,
		SortBy:    "model_size_b",
		SortOrder: "desc",
	}

	per := 50
	page := 1
	var models []types.Model
	for {
		configs, _, err := m.extllmStore.IndexWithRepo(ctx, per, page, search)
		if err != nil {
			return nil, fmt.Errorf("failed to list llm configs: %w", err)
		}

		for _, cfg := range configs {
			model, ok := m.llmConfigToModel(ctx, cfg, callerUUID, true)
			if !ok {
				continue
			}
			models = append(models, model)
		}

		if len(configs) < per {
			break
		}
		page++
	}
	return models, nil
}

// GetModelByID resolves a single model by its model ID from the llm_config table.
// It queries extllmStore.GetByModelName which loads the llm_config with its
// upstreams relation, then applies the same conversion logic as
// getModelsFromLLMConfig. Visibility filtering is applied: serverless models
// are visible to all users, non-serverless (private) models are only visible
// to the deploy owner.
func (m *openaiComponentImpl) GetModelByID(c context.Context, nsUUID, modelID string) (*types.Model, error) {
	cfg, err := m.extllmStore.GetByModelName(c, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get llm config by model name %q: %w", modelID, err)
	}
	if cfg == nil {
		return nil, nil
	}
	if !cfg.Enabled {
		return nil, nil
	}

	model, ok := m.llmConfigToModel(c, cfg, nsUUID, true)
	if !ok {
		return nil, nil
	}

	models := []types.Model{model}
	models = m.enrichModelsWithPrice(c, models)
	return &models[0], nil
}

// llmTypeFromModel returns metadata llm_type used to classify usage metering records.
func llmTypeFromModel(m *types.Model) (string, error) {
	if m == nil {
		return "", fmt.Errorf("model is nil")
	}
	if m.Metadata == nil {
		return "", fmt.Errorf("model metadata is nil: cannot resolve %s", types.MetaKeyLLMType)
	}
	llmType, ok := m.Metadata[types.MetaKeyLLMType].(string)
	if !ok || strings.TrimSpace(llmType) == "" {
		return "", fmt.Errorf("model metadata %s missing or not a string", types.MetaKeyLLMType)
	}
	return llmType, nil
}

type usageMeteringInfo struct {
	Resource  types.MeteringResource
	Scene     commontypes.SceneType
	OwnerType commontypes.TokenUsageType
}

func (m *openaiComponentImpl) resolveUsageMeteringInfo(c context.Context, nsUUID string, model *types.Model) (usageMeteringInfo, error) {
	if model == nil {
		return usageMeteringInfo{}, fmt.Errorf("model is nil")
	}
	llmType, err := llmTypeFromModel(model)
	if err != nil {
		return usageMeteringInfo{}, err
	}
	switch llmType {
	case commontypes.ProviderTypeServerless, commontypes.ProviderTypeInference:
		if model.CSGHubModelID == "" {
			return usageMeteringInfo{}, fmt.Errorf("model metadata %s=%s requires csghub model id", types.MetaKeyLLMType, llmType)
		}
		id := fmt.Sprintf(commontypes.CSGHubResourceFmt, llmType, model.CSGHubModelID)
		meteringInfo := usageMeteringInfo{
			Resource: types.MeteringResource{
				ResourceID:   id,
				ResourceName: id,
				CustomerID:   model.SvcName,
			},
			Scene: commontypes.SceneModelServerless,
		}
		if llmType == commontypes.ProviderTypeInference {
			meteringInfo.Scene = commontypes.SceneModelServerless
			ownerType, err := m.resolveUsageOwnerType(c, nsUUID, model)
			if err != nil {
				return usageMeteringInfo{}, err
			}
			meteringInfo.OwnerType = ownerType
		} else {
			meteringInfo.OwnerType = commontypes.CSGHubServerlessInference
		}
		return meteringInfo, nil
	case commontypes.ProviderTypeExternalLLM:
		if model.ID == "" {
			return usageMeteringInfo{}, fmt.Errorf("model metadata %s=%s requires model id", types.MetaKeyLLMType, llmType)
		}
		id := fmt.Sprintf(commontypes.ExternalLLMResourceFmt, model.ID)
		return usageMeteringInfo{
			Resource: types.MeteringResource{
				ResourceID:   id,
				ResourceName: id,
				CustomerID:   model.ID,
			},
			Scene:     commontypes.SceneModelServerless,
			OwnerType: commontypes.ExternalInference,
		}, nil
	default:
		return usageMeteringInfo{}, fmt.Errorf("model metadata %s has unsupported value %s", types.MetaKeyLLMType, llmType)
	}
}

func (m *openaiComponentImpl) CanManageModel(ctx context.Context, username, nsUUID string, model *types.Model) (bool, error) {
	// A nil model indicates a caller bug (target resolution failed upstream),
	// not a permission decision; surface it as an error instead of a 403.
	if model == nil {
		return false, fmt.Errorf("model is nil")
	}
	user, err := m.userStore.FindByUsername(ctx, username)
	if err != nil {
		return false, fmt.Errorf("failed to find user by username in db, error: %w", err)
	}
	if user.CanAdmin() {
		return true, nil
	}
	if model.OwnerUUID == "" {
		// External models have no deploy owner; only admins can manage them.
		return false, nil
	}
	return model.OwnerUUID == nsUUID || model.OwnerUUID == user.UUID, nil
}

func (m *openaiComponentImpl) resolveUsageOwnerType(c context.Context, nsUUID string, model *types.Model) (commontypes.TokenUsageType, error) {
	if model.OwnerUUID == nsUUID {
		return commontypes.CSGHubUserDeployedInference, nil
	}
	belong, err := m.checkOrganization(c, nsUUID, model.OwnerUUID)
	if err != nil {
		return "", fmt.Errorf("failed to check organization: %w", err)
	}
	if belong {
		return commontypes.CSGHubOrganFellowDeployedInference, nil
	}
	return commontypes.CSGHubOtherDeployedInference, nil
}

// usageMeteringExtra is serialized into MeteringEvent.Extra for usage billing breakdown.
type usageMeteringExtra struct {
	PromptTokenNum      string                     `json:"prompt_token_num"`
	PromptTokenCacheNum string                     `json:"prompt_token_cache_num"`
	CompletionTokenNum  string                     `json:"completion_token_num"`
	OwnerType           commontypes.TokenUsageType `json:"owner_type"`
	APIKey              string                     `json:"api_key"`
	Provider            string                     `json:"provider"`
	ModelName           string                     `json:"model_name"`
	// multiModalMeteringExtra is serialized into MeteringEvent.Extra for multi-modal billing.
	CompletionDataType   string `json:"completion_data_type"`
	CompletionResolution string `json:"completion_resolution"`
	CompletionDuration   string `json:"completion_duration"`
	CompletionDesc       string `json:"completion_desc"`
}

func sanitizeMeteringEventForLog(event commontypes.MeteringEvent) commontypes.MeteringEvent {
	sanitized := event
	if strings.TrimSpace(event.Extra) == "" {
		return sanitized
	}
	sanitized.Extra = apiKeyJSONFieldRegex.ReplaceAllString(event.Extra, `"api_key":""`)
	return sanitized
}

func buildUsageExtraData(usageModel *types.Model, upstreamModelName string, usage *token.Usage, apikey string, meteringInfo usageMeteringInfo) (string, error) {
	extra := usageMeteringExtra{
		PromptTokenNum:       fmt.Sprintf("%d", usage.PromptTokens),
		PromptTokenCacheNum:  fmt.Sprintf("%d", usage.CachedPromptTokens),
		CompletionTokenNum:   fmt.Sprintf("%d", usage.CompletionTokens),
		OwnerType:            meteringInfo.OwnerType,
		APIKey:               apikey,
		Provider:             usageModel.Provider,
		ModelName:            upstreamModelName,
		CompletionDataType:   usage.DataType,
		CompletionResolution: usage.Resolution,
		CompletionDuration:   fmt.Sprintf("%.2f", usage.Duration),
		CompletionDesc:       usage.CompletionDesc,
	}
	extraData, err := json.Marshal(extra)
	if err != nil {
		return "", fmt.Errorf("failed to marshal usage extra: %w", err)
	}
	return string(extraData), nil
}

func (m *openaiComponentImpl) RecordUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, counter token.Counter, apikey string) error {
	usage, err := counter.Usage(c)
	if err != nil {
		return fmt.Errorf("failed to get token usage from counter: %w", err)
	}
	return m.RecordUsageFromTokenUsage(c, nsUUID, model, targetModelName, usage, apikey)
}

func (m *openaiComponentImpl) BuildUsageMeteringEvent(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) (*commontypes.MeteringEvent, error) {
	if usage == nil {
		return nil, fmt.Errorf("cannot record usage: nil token usage")
	}
	meteringInfo, err := m.resolveUsageMeteringInfo(c, nsUUID, model)
	if err != nil {
		slog.ErrorContext(c, "cannot record usage: invalid model for resource id", slog.Any("error", err), slog.Any("model", model))
		return nil, fmt.Errorf("cannot record usage: %w", err)
	}
	if meteringInfo.Resource.ResourceID == "" {
		slog.ErrorContext(c, "cannot record usage: empty resource id for model", slog.Any("model", model))
		return nil, fmt.Errorf("cannot record usage: empty resource id")
	}
	valueType := commontypes.TokenNumberType
	value := usage.TotalTokens
	isMultiModal := commontypes.DataType(usage.DataType).IsMultiModal()
	if isMultiModal {
		meteringInfo.Scene = commontypes.SceneMultiModalServerless
		valueType = commontypes.CountNumberType
		value = usage.CompletionRC
	}
	extraData, err := buildUsageExtraData(model, targetModelName, usage, apikey, meteringInfo)
	if err != nil {
		return nil, err
	}
	event := commontypes.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     nsUUID,
		Value:        value,
		ValueType:    valueType,
		Scene:        meteringInfo.Scene,
		OpUID:        string(commontypes.AccessTokenAppAIGateway),
		ResourceID:   meteringInfo.Resource.ResourceID,
		ResourceName: meteringInfo.Resource.ResourceName,
		CustomerID:   meteringInfo.Resource.CustomerID,
		CreatedAt:    time.Now(),
		Extra:        extraData,
	}
	return &event, nil
}

func (m *openaiComponentImpl) RecordUsageFromTokenUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
	event, err := m.BuildUsageMeteringEvent(c, nsUUID, model, targetModelName, usage, apikey)
	if err != nil {
		return err
	}
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal metering event: %w", err)
	}
	err = m.eventPub.PublishMeteringEvent(eventData)
	if err != nil {
		slog.ErrorContext(c, "failed to publish token usage event", slog.Any("event", sanitizeMeteringEventForLog(*event)), slog.Any("error", err))
		return fmt.Errorf("failed to publish token usage event: %w", err)
	}

	slog.InfoContext(c, "published token usage event success", slog.Any("event", sanitizeMeteringEventForLog(*event)))
	return nil
}

func (m *openaiComponentImpl) checkOrganization(c context.Context, userUUID string, ownerUUID string) (bool, error) {
	user, err := m.userStore.FindByUUID(c, userUUID)
	if err != nil {
		slog.ErrorContext(c, "Failed to find user in db")
		return false, err
	}
	owner, err := m.userStore.FindByUUID(c, ownerUUID)
	if err != nil {
		slog.ErrorContext(c, "Failed to find owner in db")
		return false, err
	}
	userOrgs, err := m.organStore.GetUserBelongOrgs(c, user.ID)
	if err != nil {
		slog.ErrorContext(c, "Failed to find user organizations")
		return false, err
	}
	if len(userOrgs) == 0 {
		return false, nil
	}
	ownerOrgs, err := m.organStore.GetUserBelongOrgs(c, owner.ID)
	if err != nil {
		slog.ErrorContext(c, "Failed to find owner organizations")
		return false, err
	}
	if len(ownerOrgs) == 0 {
		return false, nil
	}
	userOrgansMap := make(map[int64]struct{}, len(userOrgs))
	for _, org := range userOrgs {
		userOrgansMap[org.ID] = struct{}{}
	}
	for _, org := range ownerOrgs {
		if _, ok := userOrgansMap[org.ID]; ok {
			return true, nil
		}
	}
	return false, nil
}

// dbUpstreamsToConfigs converts database.Upstream slice to types.UpstreamConfig slice for routing.
func dbUpstreamsToConfigs(dbUpstreams []database.Upstream) []commontypes.UpstreamConfig {
	result := make([]commontypes.UpstreamConfig, 0, len(dbUpstreams))
	for _, u := range dbUpstreams {
		uc := commontypes.UpstreamConfig{
			ID:                    u.ID,
			Source:                u.Source,
			URL:                   u.URL,
			Weight:                u.Weight,
			Enabled:               u.Enabled,
			ModelName:             u.ModelName,
			AuthHeader:            u.AuthHeader,
			Provider:              u.Provider,
			HealthCheckEnabled:    u.HealthCheckEnabled,
			CircuitBreakerEnabled: u.CircuitBreakerEnabled,
			Tags:                  u.Tags,
			Metadata:              u.Metadata,
			LimitPolicy:           u.LimitPolicy,
		}
		// Carry health/circuit state from DB so the proxy path can use
		// inline availability checks without a separate cache call.
		if u.HealthState != nil {
			uc.HealthState = u.HealthState.HealthState
		}
		if u.CircuitState != nil {
			uc.CircuitState = u.CircuitState.CircuitState
		}
		if uc.Weight <= 0 {
			uc.Weight = 1
		}
		result = append(result, uc)
	}
	return result
}
