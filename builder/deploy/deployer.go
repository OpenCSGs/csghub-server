package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type Deployer interface {
	Deploy(ctx context.Context, s types.Space) (deployID int64, err error)
	Status(ctx context.Context, s types.Space) (srvName string, status int, err error)
	Logs(ctx context.Context, s types.Space) (*MultiLogReader, error)
	Stop(ctx context.Context, s types.Space) (err error)
	Wakeup(ctx context.Context, s types.Space) (err error)
}

var _ Deployer = (*deployer)(nil)

type deployer struct {
	s  scheduler.Scheduler
	ib imagebuilder.Builder
	ir imagerunner.Runner

	store             *database.DeployTaskStore
	spaceStore        *database.SpaceStore
	runnerStatuscache map[string]imagerunner.StatusResponse

	internalRootDomain string
}

func newDeployer(s scheduler.Scheduler, ib imagebuilder.Builder, ir imagerunner.Runner) (*deployer, error) {
	store := database.NewDeployTaskStore()
	d := &deployer{
		s:                 s,
		ib:                ib,
		ir:                ir,
		store:             store,
		spaceStore:        database.NewSpaceStore(),
		runnerStatuscache: make(map[string]imagerunner.StatusResponse),
	}

	go d.refreshStatus()
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

func (d *deployer) refreshStatus() {
	for {
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		status, err := d.ir.StatusAll(ctxTimeout)
		cancel()
		if err != nil {
			slog.Error("refresh status all failed", slog.Any("error", err))
		} else {
			slog.Debug("status all cached", slog.Any("status", d.runnerStatuscache))
			d.runnerStatuscache = status
		}

		time.Sleep(5 * time.Second)
	}
}

func (d *deployer) Status(ctx context.Context, space types.Space) (string, int, error) {
	// get latest Deploy
	deploy, err := d.store.GetSpaceLatestDeploy(ctx, space.ID)
	if err != nil {
		return "", 0, fmt.Errorf("can't get space delopyment,%w", err)
	}

	srvName := common.UniqueSpaceAppName(space.Namespace, space.Name, space.ID)
	rstatus, found := d.runnerStatuscache[srvName]
	if !found {
		slog.Debug("status cache miss", slog.String("srv_name", srvName))
		if deploy.Status == common.Running {
			// service was Stopped or delete, so no running instance
			return srvName, common.Stopped, nil
		}
		return srvName, deploy.Status, nil
	}

	if rstatus.DeployID == 0 || rstatus.DeployID >= deploy.ID {
		return srvName, rstatus.Code, nil
	}
	return srvName, deploy.Status, nil
}

func (d *deployer) Logs(ctx context.Context, space types.Space) (*MultiLogReader, error) {
	// get latest Deploy
	deploy, err := d.store.GetSpaceLatestDeploy(ctx, space.ID)
	if err != nil {
		return nil, fmt.Errorf("can't get space delopyment,%w", err)
	}

	slog.Debug("get logs for space", slog.Any("deploy", deploy), slog.Int64("space_id", space.ID))
	buildLog, err := d.ib.Logs(ctx, &imagebuilder.LogsRequest{
		OrgName:   space.Namespace,
		SpaceName: space.Name,
		BuildID:   strconv.FormatInt(deploy.ID, 10),
	})
	if err != nil {
		// return nil, fmt.Errorf("connect to imagebuilder failed: %w", err)
		slog.Error("failed to read log from image builder", slog.Any("error", err))
	}

	runLog, err := d.ir.Logs(ctx, &imagerunner.LogsRequest{
		SpaceID:   space.ID,
		OrgName:   space.Namespace,
		SpaceName: space.Name,
	})
	if err != nil {
		slog.Error("failed to read log from image runner", slog.Any("error", err))
		// return nil, fmt.Errorf("connect to imagerunner failed: %w", err)
	}

	return NewMultiLogReader(buildLog, runLog), nil
}

func (d *deployer) Stop(ctx context.Context, space types.Space) error {
	resp, err := d.ir.Stop(ctx, &imagerunner.StopRequest{
		SpaceID:   space.ID,
		OrgName:   space.Namespace,
		SpaceName: space.Name,
	})

	slog.Info("deployer stop space", slog.Any("runner_resp", resp), slog.Int64("space_id", space.ID), slog.Any("error", err))
	return err
}

func (d *deployer) Wakeup(ctx context.Context, space types.Space) error {
	srvName := common.UniqueSpaceAppName(space.Namespace, space.Name, space.ID)
	srvURL := fmt.Sprintf("http://%s.%s", srvName, d.internalRootDomain)
	// Create a new HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Send a GET request to wake up the service
	resp, err := client.Get(srvURL)
	if err != nil {
		fmt.Printf("Error sending request to Knative service: %s\n", err)
		return fmt.Errorf("failed call service endpoint, %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode == http.StatusOK {
		return nil
	} else {
		return fmt.Errorf("space endpoint status not ok, status:%d", resp.StatusCode)
	}
}
