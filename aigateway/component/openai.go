package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strconv"
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
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

var apiKeyJSONFieldRegex = regexp.MustCompile(`"api_key"\s*:\s*"[^"]*"`)

type OpenAIComponent interface {
	GetAvailableModels(c context.Context, nsUUID string) ([]types.Model, error)
	ListModels(c context.Context, nsUUID string, req types.ListModelsReq) (types.ModelList, error)
	GetModelByID(c context.Context, nsUUID, modelID string) (*types.Model, error)
	// AutoModelID returns the virtual model ID that selects automatic
	// routing, or "" when automatic routing is not configured.
	AutoModelID() string
	// ResolveAutoModel picks the concrete model that should serve one
	// turn requested against the virtual model.  The returned decision
	// always names a model: it may be one a real model of the caller's
	// already owns the virtual ID for, or the one a pinned conversation
	// is bound to, and the decision says which.
	ResolveAutoModel(c context.Context, req types.AutoRouteRequest) (*types.AutoRouteDecision, error)
	RecordUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, tokenCounter token.Counter, apikey string, tokenID int64) error
	RecordUsageFromTokenUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, tokenID int64) error
	BuildUsageMeteringEvent(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) (*commontypes.MeteringEvent, error)
	CheckBalance(ctx context.Context, nsUUID string) error
	CheckUsageLimit(ctx context.Context, userUUID string, model *types.Model, endpoint string) error
	CommitUsageLimit(ctx context.Context, userUUID string, model *types.Model, tokenCounter token.Counter) error
	CommitUsageLimitFromUsage(ctx context.Context, userUUID string, model *types.Model, usage *token.Usage) error
	// CheckCapacityAdmission evaluates per-upstream CapacityPolicy admission
	// for the router-owned candidate set carried in the request (initial
	// admission: selection + occupation, or waiting in the upstream's
	// admission reservation queue when capacity is exhausted and queueing is
	// configured). Returns nil when admission does not apply (no candidate
	// upstream has an enabled CapacityPolicy).
	CheckCapacityAdmission(ctx context.Context, req types.CapacityAdmissionRequest) *types.AdmissionDecision
	// AcquireCapacityAdmission pins a specific upstream and atomically
	// check+acquires its lease (fallback operation: no re-selection).
	AcquireCapacityAdmission(ctx context.Context, model *types.Model, upstreamID int64, estimatedTokens int64) *types.AdmissionDecision
	// FinalizeCapacityAdmission closes the lease lifecycle: releases the
	// lease and, when usage is non-nil, commits actual token usage to the
	// TPM window. Idempotent; safe to call from multiple paths.
	FinalizeCapacityAdmission(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage)
	// FinalizeCanceledCapacityAdmission finalizes a lease whose CLIENT
	// aborted the attempt (HTTP 499 / context canceled): releases the lease
	// and also removes its RPM window entry, so the aborted attempt does not
	// rate-lock a legitimate client retry for the rest of the sliding
	// window.
	FinalizeCanceledCapacityAdmission(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage)
	// EstimateAdmissionTokens returns the TPM pre-reservation estimate for
	// a prompt text.
	EstimateAdmissionTokens(promptText string) int64
	// SetCapacityAdmissionAvailabilitySource injects the live circuit-state
	// source used by the admission reservation queue to detect an upstream
	// that turned unavailable while a request waits (reroute signal).
	SetCapacityAdmissionAvailabilitySource(src types.UpstreamCircuitStateSource)
	// ShutdownCapacityAdmission stops controller-owned background
	// goroutines (queue wake subscription). Call once at service shutdown.
	ShutdownCapacityAdmission()
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
	modelIDBuilder         upstream.ModelIDBuilder
	usageLimiter           UsageLimiter
	capacityPolicyDefaults commontypes.CapacityPolicy
	quotaRateComponent     QuotaRateComponent
	autoRouter             AutoModelRouter
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
	// Whether a real model owns the virtual ID must not depend on how
	// healthy it is, or the listing and the request path would disagree
	// about what that ID means.  The check therefore runs against the
	// caller's full visible list, before unhealthy models are dropped.
	auto, publishAuto := m.autoModel(c)
	if publishAuto {
		if shadow, found := findModelByID(models, auto.ID); found {
			// Two entries with one ID would make the listing ambiguous, and
			// the real model is the one the caller can actually name.
			slog.WarnContext(c, "a real model owns the automatic routing model ID, not publishing the virtual one",
				slog.String("model_id", shadow.ID))
			publishAuto = false
		}
	}

	models = computeModelListAvailability(models)
	// The virtual model is added after availability is computed because it
	// has no upstreams of its own and would otherwise be filtered out.  It
	// is published only while the ranking service is answering, so a
	// client never sees a model ID it cannot actually use.
	if publishAuto {
		models = append([]types.Model{auto}, models...)
	}
	modelList := filterAndPaginateModels(models, req)
	return modelList, nil
}

// autoModel returns the virtual model entry when automatic routing is
// configured and the ranking service is currently reachable.
func (m *openaiComponentImpl) autoModel(c context.Context) (types.Model, bool) {
	if m.autoRouter == nil || !m.autoRouter.Available(c) {
		return types.Model{}, false
	}
	return autoModelEntry(m.autoRouter.ModelID()), true
}

// AutoModelID reports the virtual model ID, or "" when automatic routing
// is not configured.
func (m *openaiComponentImpl) AutoModelID() string {
	if m.autoRouter == nil {
		return ""
	}
	return m.autoRouter.ModelID()
}

// ResolveAutoModel picks the model that serves one turn requested against
// the virtual model.  Candidates are the caller's own visible, available
// models narrowed by the same edition filters the model list applies, so
// automatic routing can never land on a model the caller could not have
// named directly.
func (m *openaiComponentImpl) ResolveAutoModel(c context.Context, req types.AutoRouteRequest) (*types.AutoRouteDecision, error) {
	if m.autoRouter == nil {
		return nil, fmt.Errorf("automatic model routing is not configured")
	}
	models, err := m.GetAvailableModels(c, req.TenantID)
	if err != nil {
		return nil, err
	}
	visible := applyFilters(models, modelListDefaultFilters())

	// A real model owning this ID wins, healthy or not: the virtual model
	// shares a public namespace with real ones, and whether an ID belongs
	// to a real model cannot depend on how that model happens to be
	// feeling.  The listing applies the same rule, so both agree.
	if real, found := findModelByID(visible, m.autoRouter.ModelID()); found {
		slog.WarnContext(c, "a real model owns the automatic routing model ID, serving it directly",
			slog.String("model_id", real.ID))
		return &types.AutoRouteDecision{ModelID: real.ID, Shadowed: true}, nil
	}

	// A conversation already bound to one upstream must stay on the model
	// that owns it.  Ranking a fresh model here would pick one that does
	// not have that upstream, and resolution would then fail outright.
	if req.RequiredUpstreamID != 0 {
		return pinnedAutoModel(c, visible, req.RequiredUpstreamID)
	}

	// Availability is annotated rather than filtered out, so that a model
	// which is only unhealthy right now can be told apart from one this
	// caller cannot use at all.  The two produce different HTTP statuses.
	for i := range visible {
		visible[i].Availability = &types.ModelAvailability{
			IsAvailable: modelUpstreamsAvailable(visible[i].Upstreams),
		}
	}
	return m.autoRouter.Select(c, req.Input, visible)
}

// pinnedAutoModel returns the model owning the upstream a conversation is
// bound to.  The upstream must still be usable, judged exactly as the
// ordinary resolution path judges it: a disabled, circuit-broken or
// unhealthy upstream is dropped there too, and picking it here would only
// move the failure one step later under a different error.
func pinnedAutoModel(c context.Context, visible []types.Model, upstreamID int64) (*types.AutoRouteDecision, error) {
	for _, model := range visible {
		for _, upstream := range model.Upstreams {
			if upstream.ID != upstreamID {
				continue
			}
			if unavailable, reason := types.IsUpstreamUnavailable(upstream); unavailable {
				slog.WarnContext(c, "the upstream this conversation is bound to can no longer serve it",
					slog.String("model", model.ID),
					slog.Int64("required_upstream_id", upstreamID),
					slog.String("reason", reason))
				return nil, newAutoRouteError(autoRouteCodeRequiredUpstream,
					"the upstream this conversation is bound to (%d) is %s", upstreamID, reason)
			}
			slog.InfoContext(c, "automatic routing reused the model this conversation is bound to",
				slog.String("model", model.ID),
				slog.Int64("required_upstream_id", upstreamID))
			return &types.AutoRouteDecision{ModelID: model.ID, Pinned: true}, nil
		}
	}
	return nil, newAutoRouteError(autoRouteCodeRequiredUpstream,
		"the upstream this conversation is bound to (%d) no longer exists", upstreamID)
}

// findModelByID looks a model up by its exact ID.  Model identity is
// exact everywhere else in the gateway — the catalogue is queried with a
// plain equality on model_name — so the virtual ID is owned only by a
// model spelled exactly the same way, and every path agrees on that.
func findModelByID(models []types.Model, modelID string) (types.Model, bool) {
	wanted := strings.TrimSpace(modelID)
	if wanted == "" {
		return types.Model{}, false
	}
	for _, model := range models {
		if strings.TrimSpace(model.ID) == wanted {
			return model, true
		}
	}
	return types.Model{}, false
}

// computeModelListAvailability sets ModelAvailability.IsAvailable for each model
// based on the already-loaded upstream health/circuit state (no extra DB call),
// and returns only models whose IsAvailable is true.
func computeModelListAvailability(models []types.Model) []types.Model {
	if len(models) == 0 {
		return models
	}
	filtered := make([]types.Model, 0, len(models))
	for _, model := range models {
		available := modelUpstreamsAvailable(model.Upstreams)
		model.Availability = &types.ModelAvailability{
			IsAvailable: available,
		}
		if available {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

// modelUpstreamsAvailable returns true if at least one upstream is not unavailable
// based on the inline health/circuit state carried on UpstreamConfig.
// After unification, all llm_config records must have upstreams configured;
// a model with no upstreams at all is not routable and should be filtered out.
func modelUpstreamsAvailable(upstreams []commontypes.UpstreamConfig) bool {
	if len(upstreams) == 0 {
		return false
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
//   - When applyVisibility is true, internal non-serverless models that are
//     not public (SecureLevel != EndpointPublic) and whose OwnerUUID does
//     not match callerUUID are skipped.
func (m *openaiComponentImpl) llmConfigToModel(ctx context.Context, cfg *database.LLMConfig, callerUUID string) (types.Model, bool) {
	upstreams := dbUpstreamsToConfigs(cfg.Upstreams, m.capacityPolicyDefaults)

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

		// Visibility: public (SecureLevel == EndpointPublic) internal models
		// are visible to any caller; private (and legacy zero-value) models
		// are owner-only. Serverless models are always public.
		if info.SvcType != commontypes.ServerlessType &&
			info.SecureLevel != commontypes.EndpointPublic && info.OwnerUUID != callerUUID {
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
//   - Non-serverless deploys with a public secure level
//     (SecureLevel == EndpointPublic) are visible to all users.
//   - Other non-serverless deploys (private or legacy unset secure level) are
//     only visible to the deploy owner (OwnerUUID == callerUUID).
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
			model, ok := m.llmConfigToModel(ctx, cfg, callerUUID)
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
// and public (SecureLevel == EndpointPublic) models are visible to all users;
// other non-serverless (private) models are only visible to the deploy owner.
func (m *openaiComponentImpl) GetModelByID(c context.Context, nsUUID, modelID string) (*types.Model, error) {
	cfg, err := m.extllmStore.GetByModelName(c, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get llm config by model name %q: %w", modelID, err)
	}
	if cfg != nil && cfg.Enabled {
		if model, ok := m.llmConfigToModel(c, cfg, nsUUID); ok {
			models := m.enrichModelsWithPrice(c, []types.Model{model})
			return &models[0], nil
		}
	}

	// No real model owns this ID, so it may be the virtual one.  The
	// lookup is in this order, not the reverse, so that a real model can
	// never be shadowed by automatic routing.
	if autoID := m.AutoModelID(); autoID != "" && strings.TrimSpace(modelID) == autoID {
		if auto, ok := m.autoModel(c); ok {
			return &auto, nil
		}
	}
	return nil, nil
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
	ReasoningTokenNum   string                     `json:"reasoning_token_num"`
	OwnerType           commontypes.TokenUsageType `json:"owner_type"`
	APIKey              string                     `json:"api_key"`
	Provider            string                     `json:"provider"`
	ModelName           string                     `json:"model_name"`
	// UpstreamID is the ai_gateway_upstreams.id that served the request, as a
	// string like the other extra fields; empty/0 means unattributed (e.g.
	// events before this field existed) and is excluded from cost lookup.
	UpstreamID string `json:"upstream_id,omitempty"`
	// multiModalMeteringExtra is serialized into MeteringEvent.Extra for multi-modal billing.
	CompletionDataType   string `json:"completion_data_type"`
	CompletionResolution string `json:"completion_resolution"`
	CompletionDuration   string `json:"completion_duration"`
	CompletionDesc       string `json:"completion_desc"`
	// AutoRoute records that the caller asked for the virtual model and
	// which model automatic routing chose, so the spend a routing decision
	// produced can be reconstructed from the billing record alone.  The
	// whole object is absent for a request that named a model directly.
	AutoRoute *autoRouteMeteringExtra `json:"auto_route,omitempty"`
}

// autoRouteMeteringExtra is the automatic-routing part of the billing
// record.  RequestedModel is the virtual ID the caller sent, which is
// what makes these rows separable from ordinary ones; Rank and
// IndexVersion are what the ranking was, so a decision can be reviewed
// against the index it came from.
type autoRouteMeteringExtra struct {
	RequestedModel string `json:"requested_model"`
	Candidate      string `json:"candidate,omitempty"`
	Rank           int    `json:"rank,omitempty"`
	IndexVersion   string `json:"index_version,omitempty"`
	PolicyVersion  string `json:"policy_version,omitempty"`
	// Pinned marks a turn that reused the model its conversation was
	// already bound to rather than being ranked.
	Pinned bool `json:"pinned,omitempty"`
}

// autoRouteExtra converts the Planner's decision into its billing form.
// A shadowed decision is not automatic routing at all — a real model owned
// the virtual ID — so it is recorded as an ordinary request.
func autoRouteExtra(requestedModel string, decision *types.AutoRouteDecision) *autoRouteMeteringExtra {
	if decision == nil || decision.Shadowed {
		return nil
	}
	return &autoRouteMeteringExtra{
		RequestedModel: requestedModel,
		Candidate:      decision.BenchmarkID,
		Rank:           decision.Rank,
		IndexVersion:   decision.IndexVersion,
		PolicyVersion:  decision.PolicyVersion,
		Pinned:         decision.Pinned,
	}
}

func sanitizeMeteringEventForLog(event commontypes.MeteringEvent) commontypes.MeteringEvent {
	sanitized := event
	if strings.TrimSpace(event.Extra) == "" {
		return sanitized
	}
	sanitized.Extra = apiKeyJSONFieldRegex.ReplaceAllString(event.Extra, `"api_key":""`)
	return sanitized
}

// upstreamIDExtraValue formats the serving upstream id for the metering extra;
// 0/unattributed stays empty so the field is omitted from the JSON.
func upstreamIDExtraValue(upstreamID int64) string {
	if upstreamID <= 0 {
		return ""
	}
	return strconv.FormatInt(upstreamID, 10)
}

func buildUsageExtraData(usageModel *types.Model, upstreamModelName string, usage *token.Usage, apikey string, meteringInfo usageMeteringInfo, autoRoute *autoRouteMeteringExtra) (string, error) {
	extra := usageMeteringExtra{
		PromptTokenNum:       fmt.Sprintf("%d", usage.PromptTokens),
		PromptTokenCacheNum:  fmt.Sprintf("%d", usage.CachedPromptTokens),
		CompletionTokenNum:   fmt.Sprintf("%d", usage.CompletionTokens),
		ReasoningTokenNum:    fmt.Sprintf("%d", usage.ReasoningTokens),
		OwnerType:            meteringInfo.OwnerType,
		APIKey:               apikey,
		Provider:             usageModel.Provider,
		ModelName:            upstreamModelName,
		UpstreamID:           upstreamIDExtraValue(usageModel.UpstreamID),
		CompletionDataType:   usage.DataType,
		CompletionResolution: usage.Resolution,
		CompletionDuration:   fmt.Sprintf("%.2f", usage.Duration),
		CompletionDesc:       usage.CompletionDesc,
		AutoRoute:            autoRoute,
	}
	extraData, err := json.Marshal(extra)
	if err != nil {
		return "", fmt.Errorf("failed to marshal usage extra: %w", err)
	}
	return string(extraData), nil
}

func (m *openaiComponentImpl) RecordUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, counter token.Counter, apikey string, tokenID int64) error {
	usage, err := counter.Usage(c)
	if err != nil {
		return fmt.Errorf("failed to get token usage from counter: %w", err)
	}
	return m.RecordUsageFromTokenUsage(c, nsUUID, model, targetModelName, usage, apikey, tokenID)
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
	extraData, err := buildUsageExtraData(model, targetModelName, usage, apikey, meteringInfo,
		autoRouteExtra(m.AutoModelID(), types.AutoRouteDecisionFromContext(c)))
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

func (m *openaiComponentImpl) RecordUsageFromTokenUsage(c context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, tokenID int64) error {
	event, err := m.BuildUsageMeteringEvent(c, nsUUID, model, targetModelName, usage, apikey)
	if err != nil {
		return err
	}
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal metering event: %w", err)
	}

	if m.quotaRateComponent != nil {
		err = m.quotaRateComponent.RecordUsage(c, tokenID, int64(usage.TotalTokens))
		if err != nil {
			slog.ErrorContext(c, "failed to record token rate usage", slog.Any("tokenID", tokenID), slog.Any("error", err))
		}
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

// capacityPolicyDefaultsFromConfig maps the AIGateway CapacityPolicyDefaults
// config into a CapacityPolicy usable by ApplyDefaults. Enabled stays false:
// defaults only supply limit values, they never turn a policy on.
func capacityPolicyDefaultsFromConfig(cfg *config.Config) commontypes.CapacityPolicy {
	d := cfg.AIGateway.CapacityPolicyDefaults
	return commontypes.CapacityPolicy{
		MaxConcurrency:   d.MaxConcurrency,
		MaxQueueDepth:    d.MaxQueueDepth,
		MaxTPM:           d.MaxTPM,
		MaxRPM:           d.MaxRPM,
		QueueWaitSeconds: d.QueueWaitSeconds,
	}
}

// dbUpstreamsToConfigs converts database.Upstream slice to types.UpstreamConfig slice for routing.
// capacityDefaults fills non-positive limits of enabled CapacityPolicies
// (see CapacityPolicy.ApplyDefaults).
func dbUpstreamsToConfigs(dbUpstreams []database.Upstream, capacityDefaults commontypes.CapacityPolicy) []commontypes.UpstreamConfig {
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
		// Apply defaults on a copy so the shared database.Upstream row
		// (also read by health checks and admin APIs) is not mutated.
		if u.CapacityPolicy != nil {
			capacity := *u.CapacityPolicy
			capacity.ApplyDefaults(capacityDefaults)
			uc.CapacityPolicy = &capacity
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
