package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

type OpenAIComponent interface {
	GetAvailableModels(c context.Context, user string) ([]types.Model, error)
	GetModelByID(c context.Context, username, modelID string) (*types.Model, error)
	RecordUsage(c context.Context, userUUID string, model *types.Model, tokenCounter token.LLMTokenCounter) error
}

type openaiComponentImpl struct {
	userStore   database.UserStore
	deployStore database.DeployTaskStore
	eventPub    *event.EventPublisher
}

func NewOpenAIComponentFromConfig(config *config.Config) (OpenAIComponent, error) {
	return &openaiComponentImpl{
		userStore:   database.NewUserStore(),
		deployStore: database.NewDeployTaskStore(),
		eventPub:    &event.DefaultEventPublisher,
	}, nil
}

// GetAvailableModels returns a list of running models
func (m *openaiComponentImpl) GetAvailableModels(c context.Context, userName string) ([]types.Model, error) {
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
		m := types.Model{
			Object:        "model",
			Created:       deploy.CreatedAt.Unix(),
			Task:          string(deploy.Task),
			CSGHubModelID: deploy.Repository.Path,
			SvcName:       deploy.SvcName,
			SvcType:       deploy.Type,
		}
		modelName := ""
		if deploy.Repository.HFPath != "" {
			modelName = deploy.Repository.HFPath
		} else {
			modelName = deploy.Repository.Path
		}
		m.ID = (ModelIDBuilder{}).To(modelName, deploy.SvcName)
		// change owner of serverless deploys to OpenCSG
		if deploy.Type == 3 {
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

func (m *openaiComponentImpl) GetModelByID(c context.Context, username, modelID string) (*types.Model, error) {
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

func (m *openaiComponentImpl) RecordUsage(c context.Context, userUUID string, model *types.Model, counter token.LLMTokenCounter) error {
	usage, err := counter.Usage()
	if err != nil {
		return fmt.Errorf("failed to get token usage from counter,error:%w", err)
	}
	var tokenUsageExtra = struct {
		PromptTokenNum     int64 `json:"prompt_token_num"`
		CompletionTokenNum int64 `json:"completion_token_num"`
	}{
		PromptTokenNum:     usage.PromptTokens,
		CompletionTokenNum: usage.CompletionTokens,
	}
	extraData, _ := json.Marshal(tokenUsageExtra)
	event := commontypes.METERING_EVENT{
		Uuid:         uuid.New(),
		UserUUID:     userUUID,
		Value:        usage.TotalTokens,
		ValueType:    commontypes.TokenNumberType, // count by token
		Scene:        getSceneFromSvcType(model.SvcType),
		OpUID:        "",
		ResourceID:   model.CSGHubModelID,
		ResourceName: model.CSGHubModelID,
		CustomerID:   model.SvcName,
		CreatedAt:    time.Now(),
		Extra:        string(extraData),
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
