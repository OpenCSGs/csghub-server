package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type syncClientSettingComponentImpl struct {
	settingStore database.SyncClientSettingStore
	userStore    database.UserStore
}

type SyncClientSettingComponent interface {
	Create(ctx context.Context, req types.CreateSyncClientSettingReq) (*database.SyncClientSetting, error)
	Show(ctx context.Context, currentUser string) (*database.SyncClientSetting, error)
}

func NewSyncClientSettingComponent(config *config.Config) (SyncClientSettingComponent, error) {
	return &syncClientSettingComponentImpl{
		settingStore: database.NewSyncClientSettingStore(),
		userStore:    database.NewUserStore(),
	}, nil
}

func (c *syncClientSettingComponentImpl) Create(ctx context.Context, req types.CreateSyncClientSettingReq) (*database.SyncClientSetting, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, errorx.ErrUnauthorized
	}
	if !user.CanAdmin() {
		return nil, fmt.Errorf("only admin was allowed create sync client setting")
	}
	exists, err := c.settingStore.SyncClientSettingExists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check sync client setting if exists, error: %w", err)
	}
	if exists {
		err := c.settingStore.DeleteAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing sync client setting, error: %w", err)
		}
	}
	var mt database.SyncClientSetting
	mt.Token = req.Token
	mt.ConcurrentCount = req.ConcurrentCount
	mt.MaxBandwidth = req.MaxBandwidth
	res, err := c.settingStore.Create(ctx, &mt)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync client setting, error: %w", err)
	}
	return res, nil
}

func (c *syncClientSettingComponentImpl) Show(ctx context.Context, currentUser string) (*database.SyncClientSetting, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errorx.ErrUnauthorized
	}
	if !user.CanAdmin() {
		return nil, fmt.Errorf("only admin was allowed get sync client setting")
	}
	res, err := c.settingStore.First(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to create sync client setting, error: %w", err)
	}
	return res, nil
}
