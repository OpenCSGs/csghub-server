//go:build !ee && !saas

package activity

import (
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/component/callback"
)

type stores struct {
	syncClientSetting database.SyncClientSettingStore
}

type Activities struct {
	config        *config.Config
	callback      callback.GitCallbackComponent
	recom         component.RecomComponent
	gitServer     gitserver.GitServer
	multisync     component.MultiSyncComponent
	rftScanner    component.RuntimeArchitectureComponent
	repoComponent component.RepoComponent
	stores        stores
}

func NewActivities(
	cfg *config.Config,
	callback callback.GitCallbackComponent,
	recom component.RecomComponent,
	gitServer gitserver.GitServer,
	multisync component.MultiSyncComponent,
	syncClientSetting database.SyncClientSettingStore,
	rftScanner component.RuntimeArchitectureComponent,
	repoComponent component.RepoComponent,
) *Activities {
	stores := stores{
		syncClientSetting: syncClientSetting,
	}

	return &Activities{
		config:        cfg,
		callback:      callback,
		recom:         recom,
		gitServer:     gitServer,
		multisync:     multisync,
		stores:        stores,
		rftScanner:    rftScanner,
		repoComponent: repoComponent,
	}
}
