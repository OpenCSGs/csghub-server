package deploy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type Deployer interface {
	Deploy(ctx context.Context, dr types.DeployRepo) (deployID int64, err error)
	Status(ctx context.Context, dr types.DeployRepo) (srvName string, status int, instances []types.Instance, err error)
	Logs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error)
	Stop(ctx context.Context, dr types.DeployRepo) (err error)
	Wakeup(ctx context.Context, dr types.DeployRepo) (err error)
	Exist(ctx context.Context, dr types.DeployRepo) (bool, error)
	GetReplica(ctx context.Context, dr types.DeployRepo) (int, int, []types.Instance, error)
	InstanceLogs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error)
	ListCluster(ctx context.Context) ([]types.ClusterRes, error)
	UpdateCluster(ctx context.Context, data interface{}) (*types.UpdateClusterResponse, error)
	UpdateDeploy(ctx context.Context, mrr types.ModelRunReq, deploy *database.Deploy, frame *database.RuntimeFramework) error
	StartDeploy(ctx context.Context, deploy *database.Deploy) error
}

var _ Deployer = (*deployer)(nil)

type deployer struct {
	s  scheduler.Scheduler
	ib imagebuilder.Builder
	ir imagerunner.Runner

	store              *database.DeployTaskStore
	spaceStore         *database.SpaceStore
	runnerStatuscache  map[string]imagerunner.StatusResponse
	internalRootDomain string
	sfNode             *snowflake.Node
}

func newDeployer(s scheduler.Scheduler, ib imagebuilder.Builder, ir imagerunner.Runner) (*deployer, error) {
	store := database.NewDeployTaskStore()
	node, err := snowflake.NewNode(1)
	if err != nil || node == nil {
		slog.Error("fail to generate uuid for inference service name", slog.Any("error", err))
		return nil, err
	}
	d := &deployer{
		s:                 s,
		ib:                ib,
		ir:                ir,
		store:             store,
		spaceStore:        database.NewSpaceStore(),
		runnerStatuscache: make(map[string]imagerunner.StatusResponse),
		sfNode:            node,
	}

	go d.refreshStatus()
	go d.s.Run()

	return d, nil
}

func (d *deployer) GenerateUniqueSvcName(dr types.DeployRepo) string {
	uniqueSvcName := ""
	if dr.SpaceID > 0 {
		// space
		fields := strings.Split(dr.Path, "/")
		uniqueSvcName = common.UniqueSpaceAppName(fields[0], fields[1], dr.SpaceID)
	} else {
		// model
		// generate unique service name from uuid when create new deploy by snowflake
		uniqueSvcName = d.sfNode.Generate().Base36()
	}
	return uniqueSvcName
}

func (d *deployer) serverlessDeploy(ctx context.Context, dr types.DeployRepo) (*database.Deploy, error) {
	deploy, err := d.store.GetLatestDeployBySpaceID(ctx, dr.SpaceID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail to found the latest deploy for spaceID %v, %w", dr.SpaceID, err)
	}

	deploy.ImageID = ""
	err = d.store.UpdateDeploy(ctx, deploy)
	if err != nil {
		return nil, fmt.Errorf("fail reset deploy image, %w", err)
	}

	return deploy, nil
}

func (d *deployer) dedicatedDeploy(ctx context.Context, dr types.DeployRepo) (*database.Deploy, error) {
	uniqueSvcName := d.GenerateUniqueSvcName(dr)
	if len(uniqueSvcName) < 1 {
		return nil, fmt.Errorf("fail to generate uuid for deploy")
	}
	deploy := &database.Deploy{
		DeployName:       dr.DeployName,
		SpaceID:          dr.SpaceID,
		GitPath:          dr.GitPath,
		GitBranch:        dr.GitBranch,
		Secret:           dr.Secret,
		Template:         dr.Template,
		Env:              dr.Env,
		Hardware:         dr.Hardware,
		ImageID:          dr.ImageID,
		ModelID:          dr.ModelID,
		UserID:           dr.UserID,
		RepoID:           dr.RepoID,
		RuntimeFramework: dr.RuntimeFramework,
		ContainerPort:    dr.ContainerPort,
		Annotation:       dr.Annotation,
		MinReplica:       dr.MinReplica,
		MaxReplica:       dr.MaxReplica,
		CostPerHour:      dr.CostPerHour,
		ClusterID:        dr.ClusterID,
		SecureLevel:      dr.SecureLevel,
		SvcName:          uniqueSvcName,
	}
	err := d.store.CreateDeploy(ctx, deploy)
	return deploy, err
}

func (d *deployer) buildDeploy(ctx context.Context, dr types.DeployRepo) (*database.Deploy, error) {
	var deploy *database.Deploy = nil
	var err error = nil
	if dr.SpaceID > 0 {
		// space case: SpaceID>0 and ModelID=0, reuse latest deploy of spaces
		deploy, err = d.serverlessDeploy(ctx, dr)
		if err != nil {
			return nil, fmt.Errorf("fail to check serverless deploy for spaceID %v, %w", dr.SpaceID, err)
		}
	}

	if deploy == nil {
		// create new deploy for model inference and no latest deploy of space
		deploy, err = d.dedicatedDeploy(ctx, dr)
	}

	if err != nil {
		return nil, err
	}
	return deploy, nil
}

func (d *deployer) Deploy(ctx context.Context, dr types.DeployRepo) (int64, error) {
	deploy, err := d.buildDeploy(ctx, dr)
	if err != nil || deploy == nil {
		return -1, fmt.Errorf("failed to create deploy in db, %w", err)
	}
	// skip build step for model as inference
	bldTaskStatus := 0
	bldTaskMsg := ""
	if len(strings.Trim(deploy.ImageID, " ")) > 0 {
		bldTaskStatus = scheduler.BuildSkip
		bldTaskMsg = "Skip"
	}
	buildTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 0,
		Status:   bldTaskStatus,
		Message:  bldTaskMsg,
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

func (d *deployer) Status(ctx context.Context, dr types.DeployRepo) (string, int, []types.Instance, error) {
	deploy, err := d.store.GetDeployByID(ctx, dr.DeployID)
	if err != nil || deploy == nil {
		slog.Error("fail to get deploy by deploy id", slog.Any("DeployID", deploy.ID), slog.Any("error", err))
		return "", common.Stopped, nil, fmt.Errorf("can't get deploy, %w", err)
	}
	svcName := deploy.SvcName
	// srvName := common.UniqueSpaceAppName(dr.Namespace, dr.Name, dr.SpaceID)
	rstatus, found := d.runnerStatuscache[svcName]
	if !found {
		slog.Debug("status cache miss", slog.String("svc_name", svcName))
		if deploy.Status == common.Running {
			// service was Stopped or delete, so no running instance
			return svcName, common.Stopped, nil, nil
		}
		return svcName, deploy.Status, nil, nil
	}

	if dr.ModelID > 0 {
		targetID := dr.DeployID // support model deploy with multi-instance
		status, err := d.ir.Status(ctx, &imagerunner.StatusRequest{
			ClusterID: dr.ClusterID,
			OrgName:   dr.Namespace,
			RepoName:  dr.Name,
			SvcName:   deploy.SvcName,
			ID:        targetID,
		})
		if err != nil {
			slog.Error("fail to get status by deploy id", slog.Any("DeployID", deploy.ID), slog.Any("error", err))
			return "", common.RunTimeError, nil, fmt.Errorf("can't get deploy status, %w", err)
		}
		rstatus.Instances = status.Instances

	}
	if rstatus.DeployID == 0 || rstatus.DeployID >= deploy.ID {
		return svcName, rstatus.Code, rstatus.Instances, nil
	}
	return svcName, deploy.Status, rstatus.Instances, nil
}

func (d *deployer) Logs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error) {
	// get latest Deploy
	deploy, err := d.store.GetLatestDeployBySpaceID(ctx, dr.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("can't get space delopyment,%w", err)
	}

	slog.Debug("get logs for space", slog.Any("deploy", deploy), slog.Int64("space_id", dr.SpaceID))
	buildLog, err := d.ib.Logs(ctx, &imagebuilder.LogsRequest{
		OrgName:   dr.Namespace,
		SpaceName: dr.Name,
		BuildID:   strconv.FormatInt(deploy.ID, 10),
	})
	if err != nil {
		// return nil, fmt.Errorf("connect to imagebuilder failed: %w", err)
		slog.Error("failed to read log from image builder", slog.Any("error", err))
	}

	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	runLog, err := d.ir.Logs(ctx, &imagerunner.LogsRequest{
		ID:       targetID,
		OrgName:  dr.Namespace,
		RepoName: dr.Name,
		SvcName:  deploy.SvcName,
	})
	if err != nil {
		slog.Error("failed to read log from image runner", slog.Any("error", err))
		// return nil, fmt.Errorf("connect to imagerunner failed: %w", err)
	}

	return NewMultiLogReader(buildLog, runLog), nil
}

func (d *deployer) Stop(ctx context.Context, dr types.DeployRepo) error {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	resp, err := d.ir.Stop(ctx, &imagerunner.StopRequest{
		ID:       targetID,
		OrgName:  dr.Namespace,
		RepoName: dr.Name,
		SvcName:  dr.SvcName,
	})
	if err != nil {
		slog.Error("deployer stop deploy", slog.Any("runner_resp", resp), slog.Int64("space_id", dr.SpaceID), slog.Any("deploy_id", dr.DeployID), slog.Any("error", err))
	}
	return err
}

func (d *deployer) Wakeup(ctx context.Context, dr types.DeployRepo) error {
	// srvName := common.UniqueSpaceAppName(dr.Namespace, dr.Name, dr.SpaceID)
	svcName := dr.SvcName
	srvURL := fmt.Sprintf("http://%s.%s", svcName, d.internalRootDomain)
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

func (d *deployer) Exist(ctx context.Context, dr types.DeployRepo) (bool, error) {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	req := &imagerunner.CheckRequest{
		ID:       targetID,
		OrgName:  dr.Namespace,
		RepoName: dr.Name,
		SvcName:  dr.SvcName,
	}
	resp, err := d.ir.Exist(ctx, req)
	if err != nil {
		slog.Error("fail to check deploy", slog.Any("req", req), slog.Any("error", err))
		return true, err
	}

	if resp.Code == -1 {
		// service check with error
		slog.Error("deploy check result", slog.Any("resp", resp))
		return true, errors.New("fail to check deploy instance")
	} else if resp.Code == 1 {
		// service exist
		return true, nil
	}
	// service not exist
	return false, nil
}

func (d *deployer) GetReplica(ctx context.Context, dr types.DeployRepo) (int, int, []types.Instance, error) {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	req := &imagerunner.StatusRequest{
		ID:        targetID,
		OrgName:   dr.Namespace,
		RepoName:  dr.Name,
		ClusterID: dr.ClusterID,
		SvcName:   dr.SvcName,
	}
	resp, err := d.ir.GetReplica(ctx, req)
	if err != nil {
		slog.Warn("fail to get deploy replica with error", slog.Any("req", req), slog.Any("error", err))
		return 0, 0, []types.Instance{}, err
	}
	return resp.ActualReplica, resp.DesiredReplica, resp.Instances, nil
}

func (d *deployer) InstanceLogs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error) {
	slog.Debug("get logs for deploy", slog.Any("deploy", dr))

	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	runLog, err := d.ir.InstanceLogs(ctx, &imagerunner.InstanceLogsRequest{
		ID:           targetID,
		OrgName:      dr.Namespace,
		RepoName:     dr.Name,
		ClusterID:    dr.ClusterID,
		SvcName:      dr.SvcName,
		InstanceName: dr.InstanceName,
	})
	if err != nil {
		slog.Error("failed to read log from deploy runner", slog.Any("error", err))
		// return nil, fmt.Errorf("connect to imagerunner failed: %w", err)
	}

	return NewMultiLogReader(nil, runLog), nil
}

func (d *deployer) ListCluster(ctx context.Context) ([]types.ClusterRes, error) {
	resp, err := d.ir.ListCluster(ctx)
	if err != nil {
		return nil, err
	}
	var result []types.ClusterRes
	for _, c := range resp {
		availableGPUs := make(map[string]types.Resources)

		for _, node := range c.Nodes {
			if len(node.GPUVendor) == 0 {
				continue
			}
			gpuModel := node.GPUModel
			usedGPUs := node.UsedGPU
			totalGPUs := node.TotalGPU

			if gpuModel != "" && totalGPUs >= usedGPUs {
				availableGPUs[gpuModel] = types.Resources{
					GPUVendor:    node.GPUVendor,
					AvailableGPU: totalGPUs - usedGPUs,
					GPUModel:     gpuModel,
				}
			}
		}
		resources := make([]types.Resources, 0)
		for k, v := range availableGPUs {
			resources = append(resources, types.Resources{
				GPUModel:     k,
				AvailableGPU: v.AvailableGPU,
				GPUVendor:    v.GPUVendor,
			})
		}
		result = append(result, types.ClusterRes{
			ClusterID: c.ClusterID,
			Region:    c.Region,
			Zone:      c.Zone,
			Provider:  c.Provider,
			Resources: resources,
		})
	}
	return result, err
}

func (d *deployer) UpdateCluster(ctx context.Context, data interface{}) (*types.UpdateClusterResponse, error) {
	resp, err := d.ir.UpdateCluster(ctx, data)
	return (*types.UpdateClusterResponse)(resp), err
}

// UpdateDeploy implements Deployer.
func (d *deployer) UpdateDeploy(ctx context.Context, mrr types.ModelRunReq, deploy *database.Deploy, frame *database.RuntimeFramework) error {
	var hardware types.HardWare
	err := json.Unmarshal([]byte(mrr.Hardware), &hardware)
	if err != nil {
		return fmt.Errorf("invalid hardware setting: %v, %w", mrr.Hardware, err)
	}

	// choose image
	containerImg := frame.FrameCpuImage
	if hardware.Gpu.Num != "" {
		gpuNum, err := strconv.Atoi(hardware.Gpu.Num)
		if err != nil {
			return fmt.Errorf("invalid hardware gpu setting: %v, %w", mrr.Hardware, err)
		}
		if gpuNum > 0 {
			// use gpu image
			containerImg = frame.FrameImage
		}
	}

	deploy.DeployName = mrr.DeployName
	deploy.Env = mrr.Env
	deploy.RuntimeFramework = frame.FrameName
	deploy.ImageID = containerImg
	deploy.ContainerPort = frame.ContainerPort
	deploy.Hardware = mrr.Hardware
	deploy.MinReplica = mrr.MinReplica
	deploy.MaxReplica = mrr.MaxReplica
	deploy.GitBranch = mrr.Revision
	deploy.SecureLevel = mrr.SecureLevel
	deploy.CostPerHour = mrr.CostPerHour
	deploy.ClusterID = mrr.ClusterID

	// deploy.Status = common.Pending
	// update deploy table
	err = d.store.UpdateDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("failed to update deploy, %w", err)
	}

	return nil
}

func (d *deployer) StartDeploy(ctx context.Context, deploy *database.Deploy) error {
	deploy.Status = common.Pending
	// update deploy table
	err := d.store.UpdateDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("failed to update deploy, %w", err)
	}

	// create run model as inference task
	runTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 1,
	}
	d.store.CreateDeployTask(ctx, runTask)

	go d.s.Queue(runTask.ID)

	return nil
}
