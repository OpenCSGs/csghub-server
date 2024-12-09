package activity

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

func SyncAsClient(ctx context.Context, config *config.Config) error {
	c, err := component.NewMultiSyncComponent(config)
	if err != nil {
		slog.Error("failed to create multi sync component", "err", err)
		return err
	}
	syncClientSettingStore := database.NewSyncClientSettingStore()
	setting, err := syncClientSettingStore.First(ctx)
	if err != nil {
		slog.Error("failed to find sync client setting", "error", err)
		return err
	}
	apiDomain := config.MultiSync.SaasAPIDomain
	sc := multisync.FromOpenCSG(apiDomain, setting.Token)
	return c.SyncAsClient(ctx, sc)
}
