package deploy

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type Deployer interface {
	Deploy(ctx context.Context, s types.Space) (deployID int64, err error)
	Status(ctx context.Context, spaceID int64) (status int, err error)
	Logs(ctx context.Context, spaceID int64) (*MultiLogReader, error)
	Stop(ctx context.Context, spaceID int64) (err error)
}

var _ Deployer = (*deployer)(nil)

type deployer struct {
	s  scheduler.Scheduler
	ib imagebuilder.Builder
	ir imagerunner.Runner

	store *database.DeployTaskStore
}

func newDeployer(s scheduler.Scheduler, ib imagebuilder.Builder, ir imagerunner.Runner) (Deployer, error) {
	store := database.NewDeployTaskStore()
	d := &deployer{
		s:     s,
		ib:    ib,
		ir:    ir,
		store: store,
	}

	go d.s.Run()

	return d, nil
}

func (d *deployer) Deploy(ctx context.Context, s types.Space) (int64, error) {
	deploy := &database.Deploy{
		SpaceID:   s.ID,
		GitPath:   s.Path,
		GitBranch: s.DefaultBranch,
		Env:       s.Env,
		Secret:    s.Secrets,
		Template:  s.Template,
		Hardware:  s.Hardware,
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

	go d.s.Queue(buildTask.ID)

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

func (d *deployer) Logs(ctx context.Context, spaceID int64) (*MultiLogReader, error) {
	// get latest Deploy
	deploy, err := d.store.GetSpaceLatestDeploy(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	slog.Debug("get logs for space", slog.Any("deploy", deploy), slog.Int64("space_id", spaceID))
	buildLog, err := d.ib.Logs(ctx, &imagebuilder.LogsRequest{
		BuildID: deploy.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to imagebuilder failed: %w", err)
	}
	runLog, err := d.ir.Logs(ctx, &imagerunner.LogsRequest{ImageID: deploy.ImageID})
	if err != nil {
		return nil, fmt.Errorf("connect to imagerunner failed: %w", err)
	}

	return &MultiLogReader{
		buildReader:  buildLog.SSEReadCloser,
		runnerReader: runLog.SSEReadCloser,
	}, nil
}

func (d *deployer) Stop(ctx context.Context, spaceID int64) error {
	// get latest Deploy
	deploy, err := d.store.GetSpaceLatestDeploy(ctx, spaceID)
	if err != nil {
		return err
	}
	resp, err := d.ir.Stop(ctx, &imagerunner.StopRequest{
		OrgName:   "",
		SpaceName: "",
		BuildID:   0,
		ImageID:   deploy.ImageID,
	})

	slog.Info("stop space", slog.Any("runner_resp", resp), slog.Int64("space_id", spaceID), slog.Any("error", err))
	return err
}
