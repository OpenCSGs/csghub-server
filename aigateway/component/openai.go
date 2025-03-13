package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type OpenAIComponent interface {
	GetAvailableModels(c context.Context, user string) ([]types.Model, error)
	GetModelByID(c context.Context, username, modelID string) (*types.Model, error)
}

type openaiComponentImpl struct {
	userStore   database.UserStore
	deployStore database.DeployTaskStore
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
			Object:  "model",
			Created: deploy.CreatedAt.Unix(),
			Task:    string(deploy.Task),
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

// GetModelByID implements ModelHandler.
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

func NewOpenAIComponentFromConfig(config *config.Config) (OpenAIComponent, error) {
	return &openaiComponentImpl{
		userStore:   database.NewUserStore(),
		deployStore: database.NewDeployTaskStore(),
	}, nil
}
