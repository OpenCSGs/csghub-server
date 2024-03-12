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
	Deploy(ctx context.Context, s types.Space) (deployID int64, err error)
	Status(ctx context.Context, spaceID int64) (status int, err error)
	Logs(ctx context.Context, spaceID int64) (log string, err error)
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

func (d *deployer) Deploy(ctx context.Context, s types.Space) (int64, error) {
	deploy := &database.Deploy{
		GitPath: s.Path,
		// TODO:fix fields
		// Env: s.Env,
		// Secret: s.Secret,
	}
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

func (d *deployer) Status(ctx context.Context, spaceID int64) (int, error) {
	// get latest Deploy
	deploy, err := d.store.GetSpaceLatestDeploy(ctx, spaceID)
	if err != nil {
		return -1, err
	}
	return deploy.Status, nil
}

func (d *deployer) Logs(ctx context.Context, spaceID int64) (string, error) {
	// get latest Deploy
	deploy, err := d.store.GetSpaceLatestDeploy(ctx, spaceID)
	if err != nil {
		return "", err
	}
	// get logs of the deploy
	return d.m.Logs(ctx, deploy.ID)
}
