package activity

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/multisync"
)

func (a *Activities) SyncAsClient(ctx context.Context) error {
	if a.config.Saas {
		return nil
	}
	setting, err := a.stores.syncClientSetting.First(ctx)
	if err != nil {
		slog.Error("failed to find sync client setting", "error", err)
		return err
	}
	apiDomain := a.config.MultiSync.SaasAPIDomain
	sc := multisync.FromOpenCSG(apiDomain, setting.Token)
	return a.multisync.SyncAsClient(ctx, sc)
}
