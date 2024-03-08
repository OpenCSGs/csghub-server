package deploy

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy/monitor"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type Deployer interface {
	Deploy(s types.Space) (deployID int64, err error)
}

var _ Deployer = (*deployer)(nil)

type deployer struct {
	s scheduler.Scheduler
	m monitor.Monitor

	store *database.DeployTaskStore
}

func NewDeployer() (Deployer, error) {
	s := scheduler.NewFIFOScheduler()
	store := &database.DeployTaskStore{}
	m := monitor.NewMonitor()
	d := &deployer{s: s, m: m, store: store}

	go d.s.Run()
	go d.m.Run()

	return d, nil
}

func (d *deployer) Deploy(s types.Space) (int64, error) {
	deploy := &database.Deploy{
		GitPath: s.Path,
		// Env: s.Env,
		// Secret: s.Secret,
	}
	ctx := context.Background()
	// TODO:save deploy tasks in sql tx
	err := d.store.CreateDeploy(ctx, deploy)
	if err != nil {
		slog.Error("failed to create deploy in db", slog.Any("error", err))
		return -1, err
	}
	buildTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 0,
	}
	d.store.CreateDeployTask(ctx, buildTask)
	runTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 1,
	}
	d.store.CreateDeployTask(ctx, runTask)
	return deploy.ID, nil
}
