//go:build !ee && !saas

package component

import (
	"context"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type extendOpenai struct{}

func NewOpenAIComponentFromConfig(config *config.Config) (OpenAIComponent, error) {
	cacheClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(config.AIGateway.ModelIDFmt)) == 0 {
		return nil, fmt.Errorf("modelIDFmt is empty")
	}
	return &openaiComponentImpl{
		userStore:      database.NewUserStore(),
		organStore:     database.NewOrgStore(),
		deployStore:    database.NewDeployTaskStore(),
		eventPub:       &event.DefaultEventPublisher,
		extllmStore:    database.NewLLMConfigStore(config),
		modelListCache: cacheClient,
		extendOpenai:   extendOpenai{},
		modelIDFmt:     config.AIGateway.ModelIDFmt,
		modelIDBuilder: NewModelIDBuilder(),
	}, nil
}

func (e *openaiComponentImpl) userPreference(ctx context.Context, req *types.UserPreferenceRequest) ([]types.Model, error) {
	return req.Models, nil
}

func (e *extendOpenai) CheckBalance(ctx context.Context, username, userUUID string) error {
	return nil
}

func (e *extendOpenai) enrichModelsWithPrice(_ context.Context, models []types.Model) []types.Model {
	return models
}
