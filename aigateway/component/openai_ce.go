//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/aigateway/component/upstream"
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
		userStore:              database.NewUserStore(),
		organStore:             database.NewOrgStore(config.IsHierarchicalOrganization(), nil),
		deployStore:            database.NewDeployTaskStore(),
		eventPub:               &event.DefaultEventPublisher,
		extllmStore:            database.NewLLMConfigStore(config),
		modelListCache:         cacheClient,
		extendOpenai:           extendOpenai{},
		modelIDBuilder:         upstream.NewModelIDBuilder(),
		capacityPolicyDefaults: capacityPolicyDefaultsFromConfig(config),
	}, nil
}

func (e *openaiComponentImpl) userPreference(ctx context.Context, req *types.UserPreferenceRequest) ([]types.Model, error) {
	return req.Models, nil
}

// pinnedUserUUID reports whose pinned preferences drive the gateway listing.
// CE has no pinned-model support: ordering and flags stay untouched.
func (e *extendOpenai) pinnedUserUUID(_ string) string {
	return ""
}

// pinnedOrderingAvailable probes whether the pinned preferences can be read.
// CE never enables pinned ordering.
func (e *extendOpenai) pinnedOrderingAvailable(_ context.Context, _ string) bool {
	return false
}

// priceConfiguredFilterEnabled reports that CE listings do not hide unpriced
// models — CE carries no accounting prices at all.
func (e *extendOpenai) priceConfiguredFilterEnabled() bool {
	return false
}

func (e *extendOpenai) CheckBalance(ctx context.Context, userUUID string) error {
	return nil
}

func (e *extendOpenai) enrichModelsWithPrice(_ context.Context, models []types.Model) ([]types.Model, error) {
	return models, nil
}
