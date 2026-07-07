//go:build !ee && !saas

package activity

import (
	"log/slog"

	aigatewaytask "opencsg.com/csghub-server/aigateway/task"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/component/callback"
)

type stores struct {
	syncClientSetting database.SyncClientSettingStore
	deployTask        database.DeployTaskStore
	argoWorkFlow      database.ArgoWorkFlowStore
}

type Activities struct {
	config                 *config.Config
	callback               callback.GitCallbackComponent
	recom                  component.RecomComponent
	gitServer              gitserver.GitServer
	multisync              component.MultiSyncComponent
	rftScanner             component.RuntimeArchitectureComponent
	repoComponent          component.RepoComponent
	industryTag            component.IndustryTagComponent
	asyncGenerationService aigatewaytask.AsyncGenerationService
	stores                 stores

	// Deploy reconcile
	deployer     deploy.Deployer
	deployConfig common.DeployConfig
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
	industryTag component.IndustryTagComponent,
	asyncGenerationService aigatewaytask.AsyncGenerationService,
) *Activities {
	stores := stores{
		syncClientSetting: syncClientSetting,
		deployTask:        database.NewDeployTaskStore(),
		argoWorkFlow:      database.NewArgoWorkFlowStore(),
	}

	return &Activities{
		config:                 cfg,
		callback:               callback,
		recom:                  recom,
		gitServer:              gitServer,
		multisync:              multisync,
		stores:                 stores,
		rftScanner:             rftScanner,
		repoComponent:          repoComponent,
		industryTag:            industryTag,
		asyncGenerationService: asyncGenerationService,
		deployer:               newDeployerForReconcile(cfg),
		deployConfig:           common.BuildDeployConfig(cfg),
	}
}

func newDeployerForReconcile(cfg *config.Config) deploy.Deployer {
	dc := common.BuildDeployConfig(cfg)
	d, err := deploy.NewDeployerForReconcile(cfg, dc)
	if err != nil {
		slog.Error("failed to create deployer for reconcile", "error", err)
		return nil
	}
	return d
}
