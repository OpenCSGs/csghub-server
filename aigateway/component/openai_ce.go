//go:build !ee && !saas

package component

import (
	"context"

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
	return &openaiComponentImpl{
		userStore:      database.NewUserStore(),
		organStore:     database.NewOrgStore(),
		deployStore:    database.NewDeployTaskStore(),
		eventPub:       &event.DefaultEventPublisher,
		extllmStore:    database.NewLLMConfigStore(config),
		modelListCache: cacheClient,
		extendOpenai:   extendOpenai{},
	}, nil
}

func (e *openaiComponentImpl) userPreference(ctx context.Context, req *types.UserPreferenceRequest) ([]types.Model, error) {
	return req.Models, nil
}
