package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/types"
)

type SyncClientSettingComponent struct {
	mtStore *database.SyncClientSettingStore
}

func NewSyncClientSettingComponent(config *config.Config) (*SyncClientSettingComponent, error) {
	return &SyncClientSettingComponent{
		mtStore: database.NewSyncClientSettingStore(),
	}, nil
}

func (c *SyncClientSettingComponent) Create(ctx context.Context, req types.CreateSyncClientSettingReq) (*database.SyncClientSetting, error) {
	exists, err := c.mtStore.SyncClientSettingExists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check mirror token if exists, error: %w", err)
	}
	if exists {
		err := c.mtStore.DeleteAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing mirror token, error: %w", err)
		}
	}
	var mt database.SyncClientSetting
	mt.Token = req.Token
	mt.ConcurrentCount = req.ConcurrentCount
	mt.MaxBandwidth = req.MaxBandwidth
	res, err := c.mtStore.Create(ctx, &mt)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror token, error: %w", err)
	}
	return res, nil
}
