package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	modelCacheKey = "aigateway:models"
	modelCacheTTL = 1 * time.Minute
)

type OpenAIComponent interface {
	GetAvailableModels(c context.Context, user string) ([]types.Model, error)
	ListModels(c context.Context, user string, req types.ListModelsReq) (types.ModelList, error)
	GetModelByID(c context.Context, username, modelID string) (*types.Model, error)
	RecordUsage(c context.Context, userUUID string, model *types.Model, tokenCounter token.Counter, sceneValue string) error
	CheckBalance(ctx context.Context, username string, model *types.Model, sceneValue string) error
}

type openaiComponentImpl struct {
	userStore   database.UserStore
	organStore  database.OrgStore
	deployStore database.DeployTaskStore
	eventPub    *event.EventPublisher
	extllmStore database.LLMConfigStore

	modelListCache cache.RedisClient
	extendOpenai
}

// GetAvailableModels returns a list of running models
func (m *openaiComponentImpl) GetAvailableModels(c context.Context, userName string) ([]types.Model, error) {
	var models []types.Model
	user, err := m.userStore.FindByUsername(c, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username in db,error:%w", err)
	}
	csghubModels, err := m.getCSGHubModels(c, user.ID)
	if err != nil {
		return nil, err
	}
	models = csghubModels
	externalModels := m.getExternalModels(c)
	models = append(models, externalModels...)

	models = m.enrichModelsWithPrice(c, models)

	// Save models to cache asynchronously
	go func(modelList []types.Model) {
		if len(modelList) == 0 {
			return
		}
		if err := m.saveModelsToCache(modelList); err != nil {
			// Log error but don't affect the main request
			slog.Error("failed to save models to cache", "error", err)
		}
	}(models)

	req := &types.UserPreferenceRequest{
		UserUUID: user.UUID,
		Models:   models,
		Scenario: types.AgenticHubApp,
	}
	models, err = m.userPreference(c, req)
	if err != nil {
		slog.Warn("failed to apply user preference", "error", err)
		// Continue with original models if user preference fails
	}

	return models, nil
}

func (m *openaiComponentImpl) ListModels(c context.Context, userName string, req types.ListModelsReq) (types.ModelList, error) {
	models, err := m.GetAvailableModels(c, userName)
	if err != nil {
		return types.ModelList{}, err
	}
	return filterAndPaginateModels(models, req), nil
}

type modelFilter func(m *types.Model) bool

func filterByModelID(query string) modelFilter {
	return func(m *types.Model) bool {
		return strings.Contains(strings.ToLower(m.ID), query)
	}
}

func filterBySource(source string) modelFilter {
	return func(m *types.Model) bool {
		switch source {
		case string(types.ModelSourceCSGHub):
			return m.CSGHubModelID != ""
		case string(types.ModelSourceExternal):
			return m.Provider != ""
		default:
			return true
		}
	}
}

func filterByTask(task string) modelFilter {
	return func(m *types.Model) bool {
		modelTasks := strings.FieldsFunc(strings.ToLower(m.Task), func(r rune) bool {
			return r == ','
		})
		return slices.Contains(modelTasks, task)
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
	var filters []modelFilter

	if searchQuery := strings.ToLower(req.ModelID); searchQuery != "" {
		filters = append(filters, filterByModelID(searchQuery))
	}
	if source := strings.ToLower(req.Source); source != "" {
		filters = append(filters, filterBySource(source))
	}
	if task := strings.ToLower(req.Task); task != "" {
		filters = append(filters, filterByTask(task))
	}

	models = applyFilters(models, filters)

	per := 20
	page := 1
	if req.Per != "" {
		if parsedPer, err := strconv.Atoi(req.Per); err == nil && parsedPer > 0 {
			per = parsedPer
			if per > 100 {
				per = 100
			}
		}
	}
	if req.Page != "" {
		if parsedPage, err := strconv.Atoi(req.Page); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	totalCount := len(models)
	startIndex := (page - 1) * per
	if startIndex > totalCount {
		startIndex = totalCount
	}
	endIndex := startIndex + per
	if endIndex > totalCount {
		endIndex = totalCount
	}

	paginated := models[startIndex:endIndex]

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
		HasMore:    endIndex < totalCount,
		TotalCount: totalCount,
	}
}

// providerTypeFromDeployType maps a deploy type integer to the LLM type string (MetaKeyLLMType).
func providerTypeFromDeployType(t int) string {
	switch t {
	case commontypes.ServerlessType:
		return types.ProviderTypeServerless
	case commontypes.InferenceType:
		return types.ProviderTypeInference
	default:
		return types.ProviderTypeInference
	}
}

func (m *openaiComponentImpl) getCSGHubModels(c context.Context, userID int64) ([]types.Model, error) {
	runningDeploys, err := m.deployStore.RunningVisibleToUser(c, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get running models visible to user,error:%w", err)
	}
	var models []types.Model
	for _, deploy := range runningDeploys {
		if deploy.Repository == nil {
			slog.WarnContext(c, "skip deploy with nil repository", "deploy_id", deploy.ID, "svc_name", deploy.SvcName)
			continue
		}
		// Check if engine_args contains tool-call-parser parameter
		supportFunctionCall := strings.Contains(deploy.EngineArgs, "tool-call-parser")
		repoName := deploy.Repository.Name
		m := types.Model{
			BaseModel: types.BaseModel{
				Object:              "model",
				Created:             deploy.CreatedAt.Unix(),
				SupportFunctionCall: supportFunctionCall,
				Task:                string(deploy.Task),
				DisplayName:         repoName,
				Metadata: map[string]any{
					types.MetaKeyLLMType: providerTypeFromDeployType(deploy.Type),
				},
			},
			InternalModelInfo: types.InternalModelInfo{
				CSGHubModelID:    deploy.Repository.Path,
				OwnerUUID:        deploy.User.UUID,
				ClusterID:        deploy.ClusterID,
				SvcName:          deploy.SvcName,
				SvcType:          deploy.Type,
				ImageID:          deploy.ImageID,
				RuntimeFramework: deploy.RuntimeFramework,
			},
		}
		if deploy.Type == commontypes.ServerlessType {
			m.BaseModel.OwnedBy = "OpenCSG"
		} else {
			m.BaseModel.OwnedBy = deploy.User.Username
		}
		modelName := ""
		if deploy.Repository.HFPath != "" {
			modelName = deploy.Repository.HFPath
		} else {
			modelName = deploy.Repository.Path
		}
		m.ID = (ModelIDBuilder{}).To(modelName, deploy.SvcName)
		// change owner of serverless deploys to OpenCSG
		if deploy.Type == commontypes.ServerlessType {
			m.OwnedBy = "OpenCSG"
		} else {
			m.OwnedBy = deploy.User.Username
		}
		m.Endpoint = deploy.Endpoint
		slog.Debug("running model", slog.Any("model", m), slog.Any("deploy", deploy))
		models = append(models, m)
	}
	return models, nil
}

func (m *openaiComponentImpl) getExternalModels(c context.Context) []types.Model {
	search := &commontypes.SearchLLMConfig{}
	searchType := 16
	search.Type = &searchType
	enabled := true
	search.Enabled = &enabled

	per := 50
	page := 1
	var models []types.Model
	for {
		extModels, _, err := m.extllmStore.Index(c, per, page, search)
		if err != nil {
			slog.Error("failed to get external models", "error", err)
			break
		}

		for _, extModel := range extModels {
			// Extract tasks from metadata if present
			task := ""
			if extModel.Metadata != nil {
				if tasks, ok := extModel.Metadata[types.MetaKeyTasks].([]any); ok && len(tasks) > 0 {
					tasksStrings := make([]string, 0, len(tasks))
					for _, t := range tasks {
						if s, ok := t.(string); ok {
							tasksStrings = append(tasksStrings, s)
						}
					}
					task = strings.Join(tasksStrings, ",")
				}
			}
			if extModel.Metadata == nil {
				extModel.Metadata = map[string]any{}
			}
			extModel.Metadata[types.MetaKeyLLMType] = types.ProviderTypeExternalLLM
			m := types.Model{
				BaseModel: types.BaseModel{
					Object:      "model",
					ID:          extModel.ModelName,
					OwnedBy:     extModel.Provider,
					DisplayName: extModel.DisplayName,
					Metadata:    extModel.Metadata,
					Task:        task,
				},
				Endpoint: extModel.ApiEndpoint,
				ExternalModelInfo: types.ExternalModelInfo{
					Provider: extModel.Provider,
					AuthHead: extModel.AuthHeader,
				},
			}
			models = append(models, m)
		}
		if len(extModels) < per {
			break
		} else {
			page++
		}
	}
	return models
}

func (m *openaiComponentImpl) saveModelsToCache(models []types.Model) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Use HSET to store each model as a field in a hash
	for _, model := range models {
		model := model.ForInternalUse()
		jsonBytes, err := json.Marshal(model)
		if err != nil {
			return fmt.Errorf("failed to marshal model %s to JSON: %w", model.ID, err)
		}
		// Use model ID as the field name
		err = m.modelListCache.HSet(ctx, modelCacheKey, model.ID, string(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to set model %s in cache hash for key %s: %w", model.ID, modelCacheKey, err)
		}
	}
	// Set TTL for the entire hash
	err := m.modelListCache.Expire(ctx, modelCacheKey, modelCacheTTL)
	if err != nil {
		return fmt.Errorf("failed to set TTL for cache hash key %s: %w", modelCacheKey, err)
	}
	slog.Debug("models saved to cache hash", "key", modelCacheKey, "modelCount", len(models))
	return nil
}

// loadModelFromCache loads a model from cache by model ID
//
// if cache expire, return nil, nil
// if not hint, return nil, redis.NIL
func (m *openaiComponentImpl) loadModelFromCache(ctx context.Context, modelID string) (*types.Model, error) {
	// First check if the hash key exists to distinguish between key not found and field not found
	exists, err := m.modelListCache.Exists(ctx, modelCacheKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check cache hash key existence: %w", err)
	}

	if exists == 0 {
		// Hash key does not exist
		return nil, nil
	}

	// Get model from Redis hash using HGET
	modelJSON, err := m.modelListCache.HGet(ctx, modelCacheKey, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get model %s from cache hash for key %s: %w", modelID, modelCacheKey, err)
	}

	// Unmarshal JSON to model
	var model types.Model
	err = json.Unmarshal([]byte(modelJSON), &model)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal model %s from JSON: %w", modelID, err)
	}
	slog.Debug("model loaded from cache", "modelID", modelID)

	return &model, nil
}

func (m *openaiComponentImpl) GetModelByID(c context.Context, username, modelID string) (*types.Model, error) {
	model, err := m.loadModelFromCache(c, modelID)
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return nil, err
		}
		// redis.Nil: no cache yet; fall through to fetch from GetAvailableModels (and allow cache to be populated)
	} else if model != nil {
		return model, nil
	}
	// Cache miss or cache expired: fetch full list (which also triggers saveModelsToCache)
	models, err := m.GetAvailableModels(c, username)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if model.ID == modelID {
			return &model, nil
		}
	}
	return nil, nil
}

func getSceneFromSvcType(svcType int) int {
	switch svcType {
	case commontypes.InferenceType:
		return int(commontypes.SceneModelInference)
	case commontypes.ServerlessType:
		return int(commontypes.SceneModelServerless)
	default:
		return int(commontypes.SceneUnknow)
	}
}

func (m *openaiComponentImpl) RecordUsage(c context.Context, userUUID string, model *types.Model, counter token.Counter, sceneValue string) error {
	usage, err := counter.Usage(c)
	if err != nil {
		return fmt.Errorf("failed to get token usage from counter,error:%w", err)
	}

	scene := parseScene(sceneValue)
	slog.DebugContext(c, "token usage", slog.Any("usage", usage), slog.Any("scene", scene))
	var tokenUsageExtra = struct {
		PromptTokenNum     string `json:"prompt_token_num"`
		CompletionTokenNum string `json:"completion_token_num"`
		// 0: external, 1: owner is user, 2: other user is inference, 3: serverless
		OwnerType commontypes.TokenUsageType `json:"owner_type"`
	}{
		PromptTokenNum:     fmt.Sprintf("%d", usage.PromptTokens),
		CompletionTokenNum: fmt.Sprintf("%d", usage.CompletionTokens),
	}
	if model.CSGHubModelID != "" && model.Provider != "" {
		slog.WarnContext(c, "bad model info, both csghub model id and external model provider is set",
			slog.Any("model info", model))
	}
	if model.CSGHubModelID == "" && model.Provider == "" {
		slog.WarnContext(c, "bad model info, both csghub model id and external model provider is not set",
			slog.Any("model info", model))
	}
	if model.CSGHubModelID != "" {
		switch model.SvcType {
		case commontypes.ServerlessType:
			tokenUsageExtra.OwnerType = commontypes.CSGHubServerlessInference
		case commontypes.InferenceType:
			if model.OwnerUUID == userUUID {
				tokenUsageExtra.OwnerType = commontypes.CSGHubUserDeployedInference
			} else {
				belong, err := m.checkOrganization(c, userUUID, model.OwnerUUID)
				if err != nil {
					return fmt.Errorf("failed to check organization,error:%w", err)
				}
				if belong {
					tokenUsageExtra.OwnerType = commontypes.CSGHubOrganFellowDeployedInference
				} else {
					tokenUsageExtra.OwnerType = commontypes.CSGHubOtherDeployedInference
				}
			}
		default:
			slog.WarnContext(c, "bad model info, csghub model missing service type",
				slog.Any("model info", model))
		}
	}
	if model.Provider != "" {
		tokenUsageExtra.OwnerType = commontypes.ExternalInference
	}

	extraData, _ := json.Marshal(tokenUsageExtra)
	event := commontypes.MeteringEvent{
		Uuid:      uuid.New(),
		UserUUID:  userUUID,
		Value:     usage.TotalTokens,
		ValueType: commontypes.TokenNumberType, // count by token
		Scene:     int(scene),
		OpUID:     "aigateway",
		CreatedAt: time.Now(),
		Extra:     string(extraData),
	}
	if model.CSGHubModelID != "" {
		event.ResourceID = model.CSGHubModelID
		event.ResourceName = model.CSGHubModelID
		event.CustomerID = model.SvcName
	}
	if model.Provider != "" {
		extendModelKey := fmt.Sprintf("%s:%s", model.Provider, model.ID)
		event.ResourceID = extendModelKey
		event.ResourceName = extendModelKey
		event.CustomerID = extendModelKey
	}
	eventData, _ := json.Marshal(event)
	err = m.eventPub.PublishMeteringEvent(eventData)
	if err != nil {
		slog.ErrorContext(c, "failed to publish token usage event", slog.Any("event", event), slog.Any("error", err))
		return fmt.Errorf("failed to publish token usage event,error:%w", err)
	}

	slog.InfoContext(c, "public token usage event success", slog.Any("event", event))
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
