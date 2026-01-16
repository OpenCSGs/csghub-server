package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	modelCacheKey = "aigateway:models"
	modelCacheTTL = 1 * time.Minute
)

type OpenAIComponent interface {
	GetAvailableModels(c context.Context, user string) ([]types.Model, error)
	GetModelByID(c context.Context, username, modelID string) (*types.Model, error)
	RecordUsage(c context.Context, userUUID string, model *types.Model, tokenCounter token.Counter) error
}

type openaiComponentImpl struct {
	userStore   database.UserStore
	organStore  database.OrgStore
	deployStore database.DeployTaskStore
	eventPub    *event.EventPublisher
	extllmStore database.LLMConfigStore

	modelListCache cache.RedisClient
}

func NewOpenAIComponentFromConfig(config *config.Config) (OpenAIComponent, error) {
	cacheClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, err
	}
	return &openaiComponentImpl{
		userStore:   database.NewUserStore(),
		organStore:  database.NewOrgStore(),
		deployStore: database.NewDeployTaskStore(),
		eventPub:    &event.DefaultEventPublisher,
		extllmStore: database.NewLLMConfigStore(config),

		modelListCache: cacheClient,
	}, nil
}

// GetAvailableModels returns a list of running models
func (m *openaiComponentImpl) GetAvailableModels(c context.Context, userName string) ([]types.Model, error) {
	var models []types.Model
	csghubModels, err := m.getCSGHubModels(c, userName)
	if err != nil {
		return nil, err
	}
	models = csghubModels
	externalModels := m.getExternalModels(c)
	models = append(models, externalModels...)

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

	return models, nil
}

func (m *openaiComponentImpl) getCSGHubModels(c context.Context, userName string) ([]types.Model, error) {
	user, err := m.userStore.FindByUsername(c, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username in db,error:%w", err)
	}

	runningDeploys, err := m.deployStore.RunningVisibleToUser(c, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get running models visible to user,error:%w", err)
	}
	var models []types.Model
	for _, deploy := range runningDeploys {
		// Check if engine_args contains tool-call-parser parameter
		supportFunctionCall := strings.Contains(deploy.EngineArgs, "tool-call-parser")
		m := types.Model{
			BaseModel: types.BaseModel{
				Object:              "model",
				Created:             deploy.CreatedAt.Unix(),
				SupportFunctionCall: supportFunctionCall,
				Task:                string(deploy.Task),
			},
			InternalModelInfo: types.InternalModelInfo{
				CSGHubModelID: deploy.Repository.Path,
				OwnerUUID:     deploy.User.UUID,
				ClusterID:     deploy.ClusterID,
				SvcName:       deploy.SvcName,
				SvcType:       deploy.Type,
				ImageID:       deploy.ImageID,
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
			m := types.Model{
				BaseModel: types.BaseModel{
					Object:  "model",
					ID:      extModel.ModelName,
					OwnedBy: extModel.Provider,
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
	// First try to load model from cache
	model, err := m.loadModelFromCache(c, modelID)
	// Handle different error cases
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			// Other errors should be returned
			return nil, err
		} else {
			// no cache hint
			return nil, nil
		}
	} else {
		if model != nil {
			// Cache hit, return the model
			return model, nil
		} else {
			slog.Debug("cache expire, refresh cache")
		}
	}

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

func (m *openaiComponentImpl) RecordUsage(c context.Context, userUUID string, model *types.Model, counter token.Counter) error {
	usage, err := counter.Usage(c)
	if err != nil {
		return fmt.Errorf("failed to get token usage from counter,error:%w", err)
	}
	slog.Debug("token", slog.Any("usage", usage))
	var tokenUsageExtra = struct {
		PromptTokenNum     string `json:"prompt_token_num"`
		CompletionTokenNum string `json:"completion_token_num"`
		// 0: external, 1: owner is user, 2: other user is inference, 3: serverless
		OwnerType commontypes.TokenUsageType
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
		Scene:     int(commontypes.SceneModelServerless),
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
		event.ResourceID = model.ID
		event.ResourceName = model.ID
		event.CustomerID = model.Provider
	}
	eventData, _ := json.Marshal(event)
	err = m.eventPub.PublishMeteringEvent(eventData)
	if err != nil {
		slog.Error("failed to publish token usage event", "event", event, "error", err)
		return fmt.Errorf("failed to publish token usage event,error:%w", err)
	}

	slog.Info("public token usage event success", "event", event)
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
