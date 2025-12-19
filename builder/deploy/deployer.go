package deploy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/redis"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	hubcom "opencsg.com/csghub-server/common/utils/common"
)

type DeployWorkflowFunc func(buildTask, runTask *database.DeployTask)

var DeployWorkflow DeployWorkflowFunc

type Deployer interface {
	Deploy(ctx context.Context, dr types.DeployRepo) (deployID int64, err error)
	Status(ctx context.Context, dr types.DeployRepo, needDetails bool) (srvName string, status int, instances []types.Instance, err error)
	Logs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error)
	Stop(ctx context.Context, dr types.DeployRepo) (err error)
	StopBuild(ctx context.Context, req types.ImageBuildStopReq) (err error)
	Purge(ctx context.Context, dr types.DeployRepo) (err error)
	Wakeup(ctx context.Context, dr types.DeployRepo) (err error)
	Exist(ctx context.Context, dr types.DeployRepo) (bool, error)
	GetReplica(ctx context.Context, dr types.DeployRepo) (int, int, []types.Instance, error)
	InstanceLogs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error)
	ListCluster(ctx context.Context) ([]types.ClusterRes, error)
	GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	GetClusterUsageById(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	UpdateCluster(ctx context.Context, data types.ClusterRequest) (*types.UpdateClusterResponse, error)
	UpdateDeploy(ctx context.Context, dur *types.DeployUpdateReq, deploy *database.Deploy) error
	StartDeploy(ctx context.Context, deploy *database.Deploy) error
	CheckResourceAvailable(ctx context.Context, clusterId string, orderDetailID int64, hardWare *types.HardWare) (bool, error)
	SubmitEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error)
	DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
	GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.ArgoWorkFlowRes, error)
	CheckHeartbeatTimeout(ctx context.Context, clusterId string) (bool, error)
	SubmitFinetuneJob(ctx context.Context, req types.FinetuneReq) (*types.ArgoWorkFlowRes, error)
	DeleteFinetuneJob(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
	GetWorkflowLogsInStream(ctx context.Context, req types.FinetuneLogReq) (*MultiLogReader, error)
	GetWorkflowLogsNonStream(ctx context.Context, req types.FinetuneLogReq) (*loki.LokiQueryResponse, error)
}

func (d *deployer) GenerateUniqueSvcName(dr types.DeployRepo) string {
	uniqueSvcName := ""
	if dr.Type == types.SpaceType {
		// space
		fields := strings.Split(dr.Path, "/")
		uniqueSvcName = common.UniqueSpaceAppName("u", fields[0], fields[1], dr.SpaceID)
	} else if dr.Type == types.ServerlessType {
		// model serverless
		fields := strings.Split(dr.Path, "/")
		uniqueSvcName = common.UniqueSpaceAppName("s", fields[0], fields[1], dr.RepoID)
	} else {
		// model inference
		// generate unique service name from uuid when create new deploy by snowflake
		uniqueSvcName = d.snowflakeNode.Generate().Base36()
	}
	return uniqueSvcName
}

func (d *deployer) serverlessDeploy(ctx context.Context, dr types.DeployRepo) (*database.Deploy, error) {
	var (
		deploy *database.Deploy
		err    error
	)
	slog.Debug("do deployer.serverlessDeploy check type", slog.Any("dr.Type", dr.Type))
	if dr.Type == types.SpaceType {
		deploy, err = d.deployTaskStore.GetLatestDeployBySpaceID(ctx, dr.SpaceID)
	} else {
		deploy, err = d.deployTaskStore.GetServerlessDeployByRepID(ctx, dr.RepoID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail to find space or serverless deploy, spaceid:%v, repoid:%v, %w", dr.SpaceID, dr.RepoID, err)
	}
	if deploy == nil {
		return nil, nil
	}
	deploy.UserUUID = dr.UserUUID
	deploy.SKU = dr.SKU
	// dr.ImageID is not null for nginx space, otherwise it's ""
	deploy.ImageID = dr.ImageID
	deploy.Annotation = dr.Annotation
	deploy.Env = dr.Env
	deploy.Hardware = dr.Hardware
	deploy.RuntimeFramework = dr.RuntimeFramework
	deploy.Secret = dr.Secret
	deploy.SecureLevel = dr.SecureLevel
	deploy.ContainerPort = dr.ContainerPort
	deploy.Template = dr.Template
	deploy.MinReplica = dr.MinReplica
	deploy.MaxReplica = dr.MaxReplica
	deploy.EngineArgs = dr.EngineArgs
	deploy.Variables = dr.Variables
	deploy.ClusterID = dr.ClusterID
	deploy.Task = types.PipelineTask(dr.Task)
	// deploy
	slog.Debug("do deployer.serverlessDeploy", slog.Any("dr", dr), slog.Any("deploy", deploy))
	err = d.deployTaskStore.UpdateDeploy(ctx, deploy)
	if err != nil {
		return nil, fmt.Errorf("fail reset deploy image, %w", err)
	}
	slog.Debug("return deployer.serverlessDeploy", slog.Any("dr", dr), slog.Any("deploy", deploy))
	return deploy, nil
}

func (d *deployer) dedicatedDeploy(ctx context.Context, dr types.DeployRepo) (*database.Deploy, error) {
	uniqueSvcName := d.GenerateUniqueSvcName(dr)
	if len(uniqueSvcName) < 1 {
		err := fmt.Errorf("fail to generate uuid for deploy")
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("deploy_type", dr.Type).
				Set("path", dr.Path).
				Set("repo_id", dr.RepoID).
				Set("model_id", dr.ModelID).
				Set("space_id", dr.SpaceID),
		)
	}
	deploy := &database.Deploy{
		DeployName:       dr.DeployName,
		SpaceID:          dr.SpaceID,
		GitPath:          dr.Path,
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
		ClusterID:        dr.ClusterID,
		SecureLevel:      dr.SecureLevel,
		SvcName:          uniqueSvcName,
		Type:             dr.Type,
		UserUUID:         dr.UserUUID,
		SKU:              dr.SKU,
		Task:             types.PipelineTask(dr.Task),
		EngineArgs:       dr.EngineArgs,
		Variables:        dr.Variables,
	}
	updateDatabaseDeploy(deploy, dr)
	err := d.deployTaskStore.CreateDeploy(ctx, deploy)
	return deploy, err
}

func (d *deployer) buildDeploy(ctx context.Context, dr types.DeployRepo) (*database.Deploy, error) {
	var deploy *database.Deploy = nil
	var err error = nil
	slog.Debug("do deployer.buildDeploy check type", slog.Any("dr.Type", dr.Type))
	if dr.Type == types.SpaceType || dr.Type == types.ServerlessType {
		// space case: SpaceID>0 and ModelID=0, reuse latest deploy of spaces
		deploy, err = d.serverlessDeploy(ctx, dr)
		if err != nil {
			return nil, fmt.Errorf("fail to check serverless deploy for spaceID %v, %w", dr.SpaceID, err)
		}
	}
	slog.Debug("do deployer.buildDeploy", slog.Any("dr", dr), slog.Any("deploy", deploy))
	if deploy == nil {
		// create new deploy for model inference and no latest deploy of space
		// create new deploy for note book
		deploy, err = d.dedicatedDeploy(ctx, dr)
	}

	if err != nil {
		return nil, err
	}
	slog.Debug("return deployer.buildDeploy", slog.Any("dr", dr), slog.Any("deploy", deploy))
	return deploy, nil
}

func (d *deployer) Deploy(ctx context.Context, dr types.DeployRepo) (int64, error) {
	//check reserved resource
	err := d.checkOrderDetail(ctx, dr)
	if err != nil {
		return -1, err
	}

	deploy, err := d.buildDeploy(ctx, dr)
	slog.Debug("do deployer.Deploy", slog.Any("dr", dr), slog.Any("deploy", deploy))
	if err != nil || deploy == nil {
		return -1, fmt.Errorf("failed to create deploy in db, %w", err)
	}
	// skip build step for model as inference
	bldTaskStatus := 0
	bldTaskMsg := ""

	imgStrLen := len(strings.Trim(deploy.ImageID, " "))
	slog.Debug("do deployer.Deploy check image", slog.Any("deploy.ImageID", deploy.ImageID), slog.Any("imgStrLen", imgStrLen))
	if imgStrLen > 0 {
		bldTaskStatus = scheduler.BuildSkip
		bldTaskMsg = "Skip"
	}
	slog.Debug("create build task", slog.Any("bldTaskStatus", bldTaskStatus), slog.Any("bldTaskMsg", bldTaskMsg))
	buildTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 0,
		Status:   bldTaskStatus,
		Message:  bldTaskMsg,
	}
	err = d.deployTaskStore.CreateDeployTask(ctx, buildTask)
	if err != nil {
		return -1, fmt.Errorf("create deploy task failed: %w", err)
	}
	runTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 1,
	}
	err = d.deployTaskStore.CreateDeployTask(ctx, runTask)
	if err != nil {
		return -1, fmt.Errorf("create deploy task failed: %w", err)
	}

	buildTask.Deploy = deploy
	runTask.Deploy = deploy
	go DeployWorkflow(buildTask, runTask)

	d.logReporter.Report(types.LogEntry{
		Message:  types.PreBuildSubmit.String(),
		DeployID: strconv.FormatInt(deploy.ID, 10),
		Stage:    types.StagePreBuild,
		Step:     types.StepWaitingForResource,
		Labels: map[string]string{
			types.LogLabelTypeKey:       types.LogLabelImageBuilder,
			types.StreamKeyDeployType:   strconv.Itoa(deploy.Type),
			types.StreamKeyDeployTaskID: strconv.FormatInt(buildTask.ID, 10),
		},
	})
	return deploy.ID, nil
}

func (d *deployer) Status(ctx context.Context, dr types.DeployRepo, needDetails bool) (string, int, []types.Instance, error) {
	deploy, err := d.deployTaskStore.GetDeployByID(ctx, dr.DeployID)
	if err != nil || deploy == nil {
		slog.Error("fail to get deploy by deploy id", slog.Any("DeployID", dr.DeployID), slog.Any("error", err))
		return "", common.Stopped, nil, fmt.Errorf("can't get deploy, %w", err)
	}
	svcName := deploy.SvcName
	if deploy.Status == common.Pending {
		//if deploy is pending, no need to check ksvc status
		return svcName, common.Pending, nil, nil
	}
	svc, err := d.imageRunner.Exist(ctx, &types.CheckRequest{
		SvcName:   svcName,
		ClusterID: deploy.ClusterID,
	})
	if err != nil {
		slog.Error("fail to get deploy by service name", slog.Any("Service NamE", svcName), slog.Any("error", err))
		return "", common.Stopped, nil, fmt.Errorf("can't get svc, %w", err)
	}
	if svc.Code == common.Stopped || svc.Code == -1 {
		// like queuing, or stopped, use status from deploy
		return svcName, deploy.Status, nil, nil
	}
	return svcName, svc.Code, svc.Instances, nil
}

func (d *deployer) Logs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error) {
	deploy, err := d.deployTaskStore.GetLatestDeployBySpaceID(ctx, dr.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("can't get space latest deploy,%w", err)
	}
	deployTasks, err := d.deployTaskStore.GetDeployTasksOfDeploy(ctx, deploy.ID)
	if err != nil {
		return nil, fmt.Errorf("can't get space delopyment task%w", err)
	}

	sort.Slice(deployTasks, func(i, j int) bool {
		return deployTasks[i].ID > deployTasks[j].ID
	})

	runTask := deployTasks[0]
	buildTask := deployTasks[1]

	deployId := fmt.Sprintf("%d", buildTask.DeployID)

	// read build log from loki
	labels := map[string]string{
		types.LogLabelTypeKey:       types.LogLabelImageBuilder,
		types.StreamKeyDeployID:     deployId,
		types.StreamKeyDeployTaskID: fmt.Sprintf("%d", buildTask.ID),
	}

	if dr.InstanceName != "" {
		labels[types.StreamKeyInstanceName] = dr.InstanceName
	}

	buildLog, err := d.readLogsFromLoki(ctx, types.ReadLogRequest{
		DeployID:  deployId,
		StartTime: buildTask.CreatedAt,
		Labels:    labels,
	})
	if err != nil {
		return nil, err
	}

	// read deploy log from loki
	labels = map[string]string{
		types.LogLabelTypeKey:       types.LogLabelDeploy,
		types.StreamKeyDeployID:     deployId,
		types.StreamKeyDeployTaskID: fmt.Sprintf("%d", runTask.ID),
	}
	if dr.InstanceName != "" {
		labels[types.StreamKeyInstanceName] = dr.InstanceName
	}

	var startTime = deploy.CreatedAt
	if dr.Since != "" {
		startTime = parseSinceTime(dr.Since)
	}

	runLog, err := d.readLogsFromLoki(ctx, types.ReadLogRequest{
		DeployID:  deployId,
		StartTime: startTime,
		Labels:    labels,
	})
	if err != nil {
		return nil, err
	}

	return NewMultiLogReader(buildLog, runLog), nil
}

func (d *deployer) readLogsFromLoki(ctx context.Context, params types.ReadLogRequest) (<-chan string, error) {
	log, err := d.lokiClient.StreamAllLogs(ctx, params.DeployID, params.StartTime, params.Labels, params.TimeLoc)
	if err != nil {
		return nil, err
	}

	return log, nil
}

func (d *deployer) Stop(ctx context.Context, dr types.DeployRepo) error {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	resp, err := d.imageRunner.Stop(ctx, &types.StopRequest{
		ID:        targetID,
		OrgName:   dr.Namespace,
		RepoName:  dr.Name,
		SvcName:   dr.SvcName,
		ClusterID: dr.ClusterID,
	})
	if err != nil {
		slog.Error("deployer stop deploy", slog.Any("runner_resp", resp), slog.Int64("space_id", dr.SpaceID), slog.Any("deploy_id", dr.DeployID), slog.Any("error", err))
	}
	// release resource if it's a order case
	err = d.releaseUserResourceByOrder(ctx, dr)
	if err != nil {
		return err
	}
	return err
}

func (d *deployer) StopBuild(ctx context.Context, req types.ImageBuildStopReq) error {
	slog.Debug("deployer stop build", slog.Any("req", req))
	err := d.imageBuilder.Stop(ctx, req)
	if err != nil {
		slog.Error("deployer stop build failed", slog.Any("req", req), slog.Any("error", err))
		return fmt.Errorf("deployer stop build failed, %w", err)
	}
	slog.Info("deployer stop build success", slog.Any("req", req))
	return nil
}

func (d *deployer) Purge(ctx context.Context, dr types.DeployRepo) error {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	resp, err := d.imageRunner.Purge(ctx, &types.PurgeRequest{
		ID:         targetID,
		OrgName:    dr.Namespace,
		RepoName:   dr.Name,
		SvcName:    dr.SvcName,
		ClusterID:  dr.ClusterID,
		DeployType: dr.Type,
		UserID:     dr.UserUUID,
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
		Timeout: 3 * time.Second,
	}

	// Send a GET request to wake up the service
	resp, err := client.Get(srvURL)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		slog.Error("Error sending request to Knative service",
			slog.String("path", fmt.Sprintf("%s/%s", dr.Namespace, dr.Name)),
			slog.String("svc_name", svcName),
			slog.String("svc_url", srvURL),
			slog.Any("error", err))
		return fmt.Errorf("failed call service endpoint, %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode == http.StatusOK {
		return nil
	} else {
		var err error
		switch resp.StatusCode {
		case http.StatusNotFound:
			err = fmt.Errorf("service endpoint not found, status: %d", resp.StatusCode)
		case http.StatusUnauthorized:
			err = fmt.Errorf("unauthorized access to service endpoint, status: %d", resp.StatusCode)
		case http.StatusForbidden:
			err = fmt.Errorf("forbidden access to service endpoint, status: %d", resp.StatusCode)
		case http.StatusBadRequest:
			err = fmt.Errorf("bad request to service endpoint, status: %d", resp.StatusCode)
		case http.StatusInternalServerError:
			err = fmt.Errorf("internal server error from service endpoint, status: %d", resp.StatusCode)
		case http.StatusBadGateway:
			err = fmt.Errorf("bad gateway error from service endpoint, status: %d", resp.StatusCode)
		case http.StatusServiceUnavailable:
			err = fmt.Errorf("service unavailable, status: %d", resp.StatusCode)
		case http.StatusGatewayTimeout:
			err = fmt.Errorf("gateway timeout when accessing service endpoint, status: %d", resp.StatusCode)
		default:
			err = fmt.Errorf("unexpected response from service endpoint, status: %d", resp.StatusCode)
		}
		return errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("path", fmt.Sprintf("%s/%s", dr.Namespace, dr.Name)).
				Set("svc_name", svcName))
	}
}

func (d *deployer) Exist(ctx context.Context, dr types.DeployRepo) (bool, error) {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	req := &types.CheckRequest{
		ID:        targetID,
		OrgName:   dr.Namespace,
		RepoName:  dr.Name,
		SvcName:   dr.SvcName,
		ClusterID: dr.ClusterID,
	}
	resp, err := d.imageRunner.Exist(ctx, req)
	if err != nil {
		slog.Error("fail to check deploy", slog.Any("req", req), slog.Any("error", err))
		return true, err
	}

	if resp.Code == -1 {
		// service check with error
		slog.Warn("deploy exist check result", slog.Any("resp", resp))
		return true, errors.New("fail to check deploy instance")
	} else if resp.Code == common.Stopped {
		// service not exist
		return false, nil
	}
	return true, nil
}

func (d *deployer) GetReplica(ctx context.Context, dr types.DeployRepo) (int, int, []types.Instance, error) {
	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	req := &types.StatusRequest{
		ID:        targetID,
		OrgName:   dr.Namespace,
		RepoName:  dr.Name,
		ClusterID: dr.ClusterID,
		SvcName:   dr.SvcName,
	}
	resp, err := d.imageRunner.GetReplica(ctx, req)
	if err != nil {
		slog.Warn("fail to get deploy replica with error", slog.Any("req", req), slog.Any("error", err))
		return 0, 0, []types.Instance{}, err
	}
	return resp.ActualReplica, resp.DesiredReplica, resp.Instances, nil
}

func parseSinceTime(since string) time.Time {
	switch since {
	case "10mins":
		return time.Now().Add(-10 * time.Minute)
	case "30mins":
		return time.Now().Add(-30 * time.Minute)
	case "1hour":
		return time.Now().Add(-1 * time.Hour)
	case "6hours":
		return time.Now().Add(-6 * time.Hour)
	case "1day":
		return time.Now().Add(-24 * time.Hour)
	case "2days":
		return time.Now().Add(-48 * time.Hour)
	case "1week":
		return time.Now().Add(-7 * 24 * time.Hour)
	default:
		return time.Now().Add(-10 * time.Minute)
	}
}

func (d *deployer) InstanceLogs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error) {
	slog.Debug("get logs for deploy", slog.Any("deploy", dr))

	deploy, err := d.deployTaskStore.GetDeployByID(ctx, dr.DeployID)
	if err != nil {
		return nil, fmt.Errorf("can't get space delopyment,%w", err)
	}

	labels := map[string]string{
		types.LogLabelTypeKey:   types.LogLabelDeploy,
		types.StreamKeyDeployID: fmt.Sprintf("%d", deploy.ID),
	}
	if dr.InstanceName != "" {
		labels[types.StreamKeyInstanceName] = dr.InstanceName
	}

	deployId := fmt.Sprintf("%d", deploy.ID)
	var startTime = deploy.CreatedAt
	if dr.Since != "" {
		startTime = parseSinceTime(dr.Since)
	}
	runLog, err := d.readLogsFromLoki(ctx, types.ReadLogRequest{
		DeployID:  deployId,
		StartTime: startTime,
		Labels:    labels,
	})
	if err != nil {
		slog.Error("fail to get deploy logs", slog.Any("deploy", deploy), slog.Any("error", err))
		return nil, err
	}

	return NewMultiLogReader(nil, runLog), nil
}

func (d *deployer) ListCluster(ctx context.Context) ([]types.ClusterRes, error) {
	resp, err := d.imageRunner.ListCluster(ctx)
	if err != nil {
		return nil, err
	}
	var result []types.ClusterRes
	for _, c := range resp {
		resources := make([]types.NodeResourceInfo, 0)
		for _, node := range c.Nodes {
			resources = append(resources, node)
		}
		result = append(result, types.ClusterRes{
			ClusterID:      c.ClusterID,
			Region:         c.Region,
			Zone:           c.Zone,
			Provider:       c.Provider,
			Resources:      resources,
			LastUpdateTime: c.UpdatedAt.Unix(),
		})
	}
	return result, err
}

func (d *deployer) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	resp, err := d.imageRunner.GetClusterById(ctx, clusterId)
	if err != nil {
		slog.Warn("failed to get cluster by id in deployer", slog.Any("cluster_id", clusterId), slog.Any("error", err))
		return &types.ClusterRes{
			ClusterID: clusterId,
			Status:    types.ClusterStatusUnavailable,
		}, nil
	}

	// get reserved resources
	resources, err := d.getResources(ctx, clusterId, resp)
	if err != nil {
		return nil, err
	}
	result := types.ClusterRes{
		ClusterID:      resp.ClusterID,
		Region:         resp.Region,
		Zone:           resp.Zone,
		Provider:       resp.Provider,
		Resources:      resources,
		ResourceStatus: resp.ResourceStatus,
		Status:         types.ClusterStatusRunning,
		NodeNumber:     len(resources),
	}
	for _, node := range resources {
		result.AvailableCPU += node.AvailableCPU
		result.AvailableGPU += node.AvailableXPU
		result.AvailableMem += float64(node.AvailableMem)
		result.TotalCPU += node.TotalCPU
		result.TotalMem += float64(node.TotalMem)
		result.TotalGPU += node.TotalXPU
	}
	result.CPUUsage = (result.TotalCPU - result.AvailableCPU) / result.TotalCPU
	result.MemUsage = (result.TotalMem - result.AvailableMem) / result.TotalMem
	result.GPUUsage = float64(result.TotalGPU-result.AvailableGPU) / float64(result.TotalGPU)
	return &result, err
}

func (d *deployer) GetClusterUsageById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	resp, err := d.imageRunner.GetClusterById(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	res := types.ClusterRes{
		ClusterID: resp.ClusterID,
		Region:    resp.Region,
		Zone:      resp.Zone,
		Provider:  resp.Provider,
		Status:    types.ClusterStatusRunning,
	}
	var vendorSet = make(map[string]struct{}, 0)
	var modelsSet = make(map[string]struct{}, 0)
	for _, node := range resp.Nodes {
		res.TotalCPU += node.TotalCPU
		res.AvailableCPU += node.AvailableCPU
		res.TotalMem += float64(node.TotalMem)
		res.AvailableMem += float64(node.AvailableMem)
		res.TotalGPU += node.TotalXPU
		res.AvailableGPU += node.AvailableXPU
		if node.GPUVendor != "" {
			vendorSet[node.GPUVendor] = struct{}{}
			modelsSet[fmt.Sprintf("%s(%s)", node.XPUModel, node.XPUMem)] = struct{}{}
		}
	}

	var vendor string
	for k := range vendorSet {
		vendor += k + ", "
	}
	if vendor != "" {
		vendor = vendor[:len(vendor)-2]
	}

	var models string
	for k := range modelsSet {
		models += k + ", "
	}
	if models != "" {
		models = models[:len(models)-2]
	}

	res.XPUVendors = vendor
	res.XPUModels = models
	res.AvailableCPU = math.Floor(res.AvailableCPU)
	res.TotalMem = math.Floor(res.TotalMem)
	res.AvailableMem = math.Floor(res.AvailableMem)
	res.NodeNumber = len(resp.Nodes)
	res.CPUUsage = math.Round((res.TotalCPU-res.AvailableCPU)/res.TotalCPU*100) / 100
	res.MemUsage = math.Round((res.TotalMem-res.AvailableMem)/res.TotalMem*100) / 100
	res.GPUUsage = math.Round(float64(res.TotalGPU-res.AvailableGPU)/float64(res.TotalGPU)*100) / 100

	return &res, err
}

func (d *deployer) UpdateCluster(ctx context.Context, data types.ClusterRequest) (*types.UpdateClusterResponse, error) {
	return d.imageRunner.UpdateCluster(ctx, &data)
}

// UpdateDeploy implements Deployer.
func (d *deployer) UpdateDeploy(ctx context.Context, dur *types.DeployUpdateReq, deploy *database.Deploy) error {
	var (
		frame    *database.RuntimeFramework
		resource *database.SpaceResource
		hardware *types.HardWare
		err      error
	)

	if dur.RuntimeFrameworkID != nil {
		frame, err = d.runtimeFrameworkStore.FindEnabledByID(ctx, *dur.RuntimeFrameworkID)
		if err != nil || frame == nil {
			return fmt.Errorf("can't find available runtime framework %v, %w", *dur.RuntimeFrameworkID, err)
		}
	}

	if dur.ResourceID != nil {
		resource, err = d.spaceResourceStore.FindByID(ctx, *dur.ResourceID)
		if err != nil {
			return fmt.Errorf("error finding space resource %d, %w", *dur.ResourceID, err)
		}
		err = json.Unmarshal([]byte(resource.Resources), &hardware)
		if err != nil {
			return fmt.Errorf("invalid resource hardware setting, %w", err)
		}
		deploy.Hardware = resource.Resources
		deploy.SKU = strconv.FormatInt(resource.ID, 10)
	} else {
		err = json.Unmarshal([]byte(deploy.Hardware), &hardware)
		if err != nil {
			return fmt.Errorf("invalid deploy hardware setting, %w", err)
		}
	}

	if dur.DeployName != nil {
		deploy.DeployName = *dur.DeployName
	}
	if dur.Env != nil {
		deploy.Env = *dur.Env
	}

	if frame != nil {
		// choose image
		containerImg := frame.FrameImage
		deploy.ImageID = containerImg
		deploy.RuntimeFramework = frame.FrameName
		deploy.ContainerPort = frame.ContainerPort
	}

	if dur.MinReplica != nil {
		deploy.MinReplica = *dur.MinReplica
	}

	if dur.MaxReplica != nil {
		deploy.MaxReplica = *dur.MaxReplica
	}

	if deploy.MaxReplica < deploy.MinReplica {
		return fmt.Errorf("invalid min/max replica %d/%d", deploy.MinReplica, deploy.MaxReplica)
	}

	if dur.Revision != nil {
		deploy.GitBranch = *dur.Revision
	}

	if dur.SecureLevel != nil {
		deploy.SecureLevel = *dur.SecureLevel
	}

	if dur.ClusterID != nil {
		deploy.ClusterID = *dur.ClusterID
	}

	if dur.EngineArgs != nil {
		deploy.EngineArgs = *dur.EngineArgs
	}

	if dur.Entrypoint != nil {
		if deploy.RuntimeFramework == string(types.LlamaCpp) {
			newVarStr, err := buildVariables(dur)
			if err != nil {
				return fmt.Errorf("build variables for llama cpp error: %w", err)
			}
			dur.Variables = &newVarStr
		}
	}

	if dur.Variables != nil {
		deploy.Variables = *dur.Variables
	}

	// update deploy table
	err = d.deployTaskStore.UpdateDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("failed to update deploy, %w", err)
	}

	return nil
}

func buildVariables(dur *types.DeployUpdateReq) (string, error) {
	varStr := ""
	if dur.Variables != nil {
		varStr = *dur.Variables
	}
	varMap, err := hubcom.JsonStrToMap(varStr)
	if err != nil {
		return "", fmt.Errorf("invalid json format of variables error: %w", err)
	}
	varMap[types.GGUFEntryPoint] = *dur.Entrypoint
	varBytes, err := json.Marshal(varMap)
	if err != nil {
		return "", fmt.Errorf("marshal variables error: %w", err)
	}
	varStr = string(varBytes)
	return varStr, nil
}

func (d *deployer) StartDeploy(ctx context.Context, deploy *database.Deploy) error {
	deploy.Status = common.Pending
	// update deploy table
	err := d.deployTaskStore.UpdateDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("failed to update deploy, %w", err)
	}

	// start model as inference/serverless task
	runTask := &database.DeployTask{
		DeployID: deploy.ID,
		TaskType: 1,
	}
	err = d.deployTaskStore.CreateDeployTask(ctx, runTask)
	if err != nil {
		return fmt.Errorf("create deploy task failed: %w", err)
	}

	runTask.Deploy = deploy
	go DeployWorkflow(nil, runTask) // runTask is the only task
	// update resource if it's a order case
	err = d.updateUserResourceByOrder(ctx, deploy)
	if err != nil {
		return err
	}

	return nil
}

func (d *deployer) startJobs() {
	go d.startAccounting()
}

func (d *deployer) getResourceMap() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resList, err := d.spaceResourceStore.FindAll(ctx)
	resources := make(map[string]string)
	if err != nil {
		slog.Error("failed to get hub resource", slog.Any("error", err))
	} else {
		for _, res := range resList {
			resources[strconv.FormatInt(res.ID, 10)] = res.Name
		}
	}
	return resources
}

func (d *deployer) startAcctMetering() {
	for {
		// run once every STARHUB_SERVER_SYNC_IN_MINUTES minutes
		// accounting interval in min, get from env config
		startTimeSec := time.Now().Unix()
		errLock := d.deployConfig.RedisLocker.GetServerAcctMeteringLock()
		if errLock != nil && errors.Is(errLock, redis.ErrLockAcquire) {
			slog.Warn("skip deployer metering only for fail getting distriubte lock", slog.Any("error", errLock))
			d.sleepAcctInterval(startTimeSec)
			continue
		}
		d.startAcctMeteringProcess()
		d.sleepAcctInterval(startTimeSec)
		if errLock != nil {
			slog.Warn("do metering with get distributed lock error", slog.Any("error", errLock))
		} else {
			ok, err := d.deployConfig.RedisLocker.ReleaseServerAcctMeteringLock()
			if err != nil {
				slog.Error("failed to release deployer metering lock", slog.Any("error", err), slog.Any("ok", ok))
			}
		}
	}
}

func (d *deployer) sleepAcctInterval(startTimeSec int64) {
	// sleep remain seconds until next metering interval
	totalSecond := int64(d.eventPub.SyncInterval * 60)
	endTimeSec := time.Now().Unix()
	remainSeconds := totalSecond - (endTimeSec - startTimeSec)
	slog.Debug("sleep until next metering interval for deployer metering", slog.Any("remain seconds", remainSeconds))
	if remainSeconds > 0 {
		time.Sleep(time.Duration(remainSeconds) * time.Second)
	}
}

func (d *deployer) getClusterMap() map[string]database.ClusterInfo {
	clusterMap := make(map[string]database.ClusterInfo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clusters, err := d.clusterStore.List(ctx)
	if err != nil {
		slog.Error("failed to get cluster list", slog.Any("error", err))
		return clusterMap
	}

	for _, cluster := range clusters {
		clusterMap[cluster.ClusterID] = cluster
	}
	return clusterMap
}

func (d *deployer) startAcctMeteringProcess() {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	runningDeploys, err := d.deployTaskStore.ListAllRunningDeploys(ctxTimeout)
	if err != nil {
		slog.Error("skip meterting due to fail to get all running deploys in deployer", slog.Any("error", err))
		return
	}
	resMap := d.getResourceMap()
	slog.Debug("get resources map", slog.Any("resMap", resMap))
	eventTime := time.Now()
	clusterMap := d.getClusterMap()
	for _, deploy := range runningDeploys {
		d.startAcctMeteringRequest(resMap, clusterMap, deploy, eventTime)
	}

	runningEvaluations, err := d.argoWorkflowStore.ListAllRunningEvaluations(ctxTimeout)
	if err != nil {
		slog.Error("skip meterting due to fail to get all running evaluations in deployer", slog.Any("error", err))
		return
	}
	for _, evaluation := range runningEvaluations {
		d.startAcctForEvaluations(clusterMap, evaluation)
	}

}

func (d *deployer) startAcctForEvaluations(clusterMap map[string]database.ClusterInfo,
	evaluation database.ArgoWorkflow) {
	// skip for cluster does not exist
	cluster, ok := clusterMap[evaluation.ClusterID]
	if !ok {
		slog.Warn("skip evaluation metering for no valid cluster found by id",
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", evaluation.ClusterID))
		return
	}

	// skip metering for cluster is not running
	if cluster.Status != types.ClusterStatusRunning {
		slog.Warn("skip evaluation metering for cluster is not running",
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region))
		return
	}

	// cluster does not have hearbeat more than 10 mins, ignore it
	if time.Now().Unix()-cluster.UpdatedAt.Unix() > int64(d.deployConfig.HeartBeatTimeInSec*2) {
		slog.Warn(fmt.Sprintf("skip evaluation metering for cluster does not have heartbeat more than %d seconds",
			(d.deployConfig.HeartBeatTimeInSec*2)),
			slog.Any("task_id", evaluation.TaskId),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region),
			slog.Any("zone", cluster.Zone), slog.Any("provider", cluster.Provider),
			slog.Any("updated_at", cluster.UpdatedAt))
		return
	}

	// skip metering for evaluation does not have heartbeat more than 2 days
	if time.Now().Unix()-evaluation.StartTime.Unix() > int64(86400*2) {
		slog.Warn(fmt.Sprintf("skip evaluation metering for cluster does not have heartbeat more than %d seconds",
			(86400*2)), slog.Any("task_id", evaluation.TaskId))
		return
	}

	// ignore deploy without sku resource
	if evaluation.ResourceId < 1 {
		return
	}

	event := types.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     evaluation.UserUUID,
		Value:        int64(d.eventPub.SyncInterval),
		ValueType:    types.TimeDurationMinType,
		Scene:        int(types.SceneEvaluation),
		OpUID:        "",
		ResourceID:   strconv.FormatInt(evaluation.ResourceId, 10),
		ResourceName: evaluation.ResourceName,
		CustomerID:   evaluation.TaskId,
		CreatedAt:    time.Now(),
	}
	str, err := json.Marshal(event)
	if err != nil {
		slog.Error("error marshal evaluation metering event", slog.Any("event", event), slog.Any("error", err))
		return
	}
	err = d.eventPub.PublishMeteringEvent(str)
	if err != nil {
		slog.Error("failed to pub evaluation metering event", slog.Any("data", string(str)), slog.Any("error", err))
	} else {
		slog.Info("pub metering evaluation event success", slog.Any("data", string(str)))
	}
}

func (d *deployer) startAcctMeteringRequest(resMap map[string]string, clusterMap map[string]database.ClusterInfo,
	deploy database.Deploy, eventTime time.Time) {
	// skip for cluster does not exist
	cluster, ok := clusterMap[deploy.ClusterID]
	if !ok {
		slog.Warn("skip metering for no valid cluster found by id",
			slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName),
			slog.Any("cluster_id", deploy.ClusterID))
		return
	}

	// skip metering for cluster is not running
	if cluster.Status != types.ClusterStatusRunning {
		slog.Warn("skip metering for cluster is not running",
			slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region),
			slog.Any("zone", cluster.Zone), slog.Any("provider", cluster.Provider))
		return
	}

	// cluster does not have hearbeat more than 10 mins, ignore it
	if time.Now().Unix()-cluster.UpdatedAt.Unix() > int64(d.deployConfig.HeartBeatTimeInSec*2) {
		slog.Warn(fmt.Sprintf("skip metering for cluster does not have heartbeat more than %d seconds",
			(d.deployConfig.HeartBeatTimeInSec*2)),
			slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName),
			slog.Any("cluster_id", cluster.ClusterID), slog.Any("Region", cluster.Region),
			slog.Any("zone", cluster.Zone), slog.Any("provider", cluster.Provider),
			slog.Any("updated_at", cluster.UpdatedAt))
		return
	}

	// ignore deploy without sku resource
	if len(deploy.SKU) < 1 {
		return
	}
	resName, exists := resMap[deploy.SKU]
	if !exists {
		slog.Warn("Did not find space resource by id for metering",
			slog.Any("id", deploy.SKU), slog.Any("deploy_id", deploy.ID), slog.Any("svc_name", deploy.SvcName))
		return
	}
	slog.Debug("metering deploy", slog.Any("deploy", deploy))
	sceneType := common.GetValidSceneType(deploy.Type)
	if sceneType == types.SceneUnknow {
		slog.Error("invalid deploy type of service for metering", slog.Any("deploy", deploy))
		return
	}
	if sceneType == types.SceneModelServerless {
		// skip metering for model serverless scene
		return
	}

	var hardware types.HardWare
	if err := json.Unmarshal([]byte(deploy.Hardware), &hardware); err != nil {
		slog.Error("Deploy hardware is invalid format", "hardware", deploy.Hardware, "deploy_id", deploy.ID)
		return
	}

	// Get replica count and multiply value by instance count
	replicaCount := 1
	//only check for single node deploy, muti-node don't support replica
	if hardware.Replicas < 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Namespace and Name are not used in GetReplica call chain, only SvcName and ClusterID are needed
		dr := types.DeployRepo{
			DeployID:  deploy.ID,
			SpaceID:   deploy.SpaceID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}
		actualReplica, _, _, err := d.GetReplica(ctx, dr)
		if err != nil {
			slog.Warn("fail to get deploy replica for metering", slog.Any("deploy_id", deploy.ID), slog.Any("error", err))
		} else if actualReplica > 0 {
			replicaCount = actualReplica
		}
	}

	extra := startAcctRequestFeeExtra(deploy, d.deployConfig.UniqueServiceName)
	event := types.MeteringEvent{
		Uuid:         uuid.New(), //v4
		UserUUID:     deploy.UserUUID,
		Value:        int64(d.eventPub.SyncInterval) * int64(replicaCount),
		ValueType:    types.TimeDurationMinType,
		Scene:        int(sceneType),
		OpUID:        "",
		ResourceID:   deploy.SKU,
		ResourceName: resName,
		CustomerID:   deploy.SvcName,
		CreatedAt:    eventTime,
		Extra:        extra,
	}
	str, err := json.Marshal(event)
	if err != nil {
		slog.Error("error marshal metering event", slog.Any("event", event), slog.Any("error", err))
		return
	}
	err = d.eventPub.PublishMeteringEvent(str)
	if err != nil {
		slog.Error("failed to pub metering event", slog.Any("data", string(str)), slog.Any("error", err))
	} else {
		slog.Debug("pub metering event success", slog.Any("data", string(str)))
	}
}

func (d *deployer) CheckResourceAvailable(ctx context.Context, clusterId string, orderDetailID int64, hardWare *types.HardWare) (bool, error) {
	// backward compatibility for old api
	if clusterId == "" {
		clusters, err := d.ListCluster(ctx)
		if err != nil {
			return false, err
		}
		if len(clusters) == 0 {
			return false, fmt.Errorf("can not list clusters")
		}
		clusterId = clusters[0].ClusterID
	}
	clusterResources, err := d.GetClusterById(ctx, clusterId)
	if err != nil {
		return false, err
	}
	err = d.checkOrderDetailByID(ctx, orderDetailID)
	if err != nil {
		return false, err
	}

	if clusterResources.Status == types.ClusterStatusUnavailable {
		return false, fmt.Errorf("failed to check cluster available resource due to cluster %s status is %s",
			clusterId, clusterResources.Status)
	}

	if clusterResources.ResourceStatus != types.StatusUncertain && !CheckResource(clusterResources, hardWare) {
		return false, fmt.Errorf("required resource on cluster %s is not enough with resource status %s",
			clusterId, clusterResources.ResourceStatus)
	}

	return true, nil
}

func CheckResource(clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
	if hardware == nil {
		slog.Error("hardware is empty for check resource", slog.Any("clusterResources", clusterResources))
		return false
	}
	mem, err := strconv.Atoi(strings.Replace(hardware.Memory, "Gi", "", -1))
	if err != nil {
		slog.Error("failed to parse hardware memory ", slog.Any("error", err))
		return false
	}
	if hardware.Replicas > 1 {
		return checkMultiNodeResource(mem, clusterResources, hardware)
	} else {
		return checkSingleNodeResource(mem, clusterResources, hardware)
	}
}

// check reousrce for sigle node
func checkSingleNodeResource(mem int, clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
	for _, node := range clusterResources.Resources {
		if float32(mem) <= node.AvailableMem {
			isAvailable := checkNodeResource(node, hardware)
			if isAvailable {
				// if true return, otherwise continue check next node
				return true
			}
		}
	}
	return false
}

func checkMultiNodeResource(mem int, clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
	ready := 0
	for _, node := range clusterResources.Resources {
		if float32(mem) <= node.AvailableMem {
			isAvailable := checkNodeResource(node, hardware)
			if isAvailable {
				ready++
				if ready >= hardware.Replicas {
					return true
				}
			}
		}
	}
	return false
}

// SubmitEvaluation
func (d *deployer) SubmitEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error) {
	env := make(map[string]string)
	env["REVISIONS"] = strings.Join(req.Revisions, ",")
	env["DATASET_REVISIONS"] = strings.Join(req.DatasetRevisions, ",")
	env["MODEL_IDS"] = strings.Join(req.ModelIds, ",")
	env["DATASET_IDS"] = strings.Join(req.Datasets, ",")
	env["USE_CUSTOM_DATASETS"] = strconv.FormatBool(req.UseCustomDataset)
	env["ACCESS_TOKEN"] = req.Token
	env["HF_ENDPOINT"] = req.DownloadEndpoint
	env["HF_HUB_DOWNLOAD_TIMEOUT"] = "30"

	common.UpdateEvaluationEnvHardware(env, req.Hardware)

	templates := []types.ArgoFlowTemplate{}
	templates = append(templates, types.ArgoFlowTemplate{
		Name:     "evaluation",
		Env:      env,
		HardWare: req.Hardware,
		Image:    req.Image,
	},
	)
	uniqueFlowName := d.snowflakeNode.Generate().Base36()
	flowReq := &types.ArgoWorkFlowReq{
		TaskName:     req.TaskName,
		TaskId:       uniqueFlowName,
		TaskType:     req.TaskType,
		TaskDesc:     req.TaskDesc,
		Image:        req.Image,
		Datasets:     req.Datasets,
		Username:     req.Username,
		UserUUID:     req.UserUUID,
		RepoIds:      req.ModelIds,
		Entrypoint:   "evaluation",
		ClusterID:    req.ClusterID,
		Templates:    templates,
		RepoType:     req.RepoType,
		ResourceId:   req.ResourceId,
		ResourceName: req.ResourceName,
	}
	if req.ResourceId == 0 {
		flowReq.ShareMode = true
	}
	return d.imageRunner.SubmitWorkFlow(ctx, flowReq)
}

func (d *deployer) DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	_, err := d.imageRunner.DeleteWorkFlow(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (d *deployer) GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.ArgoWorkFlowRes, error) {
	wf, err := d.imageRunner.GetWorkFlow(ctx, req)
	if err != nil {
		return nil, err
	}
	return wf, err
}

// CheckHeartbeatTimeout checks if any cluster has not sent a heartbeat within the timeout duration.
// It returns true if a timeout is detected, otherwise false.
func (d *deployer) CheckHeartbeatTimeout(ctx context.Context, clusterId string) (bool, error) {
	timeoutDuration := 2 * d.deployConfig.HeartBeatTimeInSec
	cluster, err := d.clusterStore.ByClusterID(ctx, clusterId)
	if err != nil {
		return false, fmt.Errorf("failed to get clusters: %w", err)
	}

	now := time.Now().UTC()

	if cluster.Status == types.ClusterStatusUnavailable {
		return true, nil
	}

	if int64(now.Sub(cluster.UpdatedAt.UTC()).Seconds()) > int64(timeoutDuration) {
		slog.Warn("detected cluster heartbeat timeout",
			slog.String("cluster_id", cluster.ClusterID),
			slog.Time("last_heartbeat", cluster.UpdatedAt))
		return true, nil
	}

	return false, nil
}

func (d *deployer) SubmitFinetuneJob(ctx context.Context, req types.FinetuneReq) (*types.ArgoWorkFlowRes, error) {
	finetunedModelName := ""
	orgModelNames := strings.Split(req.ModelId, "/")
	if len(orgModelNames) == 2 {
		// "Qwen3-0.6B-finetuned-20251023_134013"
		suffix := time.Now().Format("20060102-150405")
		finetunedModelName = fmt.Sprintf("%s-finetuned-%s", orgModelNames[1], suffix)
	}

	env := make(map[string]string)
	env["MODEL_ID"] = req.ModelId
	env["DATASET_ID"] = req.DatasetId
	env["ACCESS_TOKEN"] = req.Token
	env["HF_TOKEN"] = req.Token
	env["HF_ENDPOINT"], _ = url.JoinPath(req.DownloadEndpoint, "hf")
	env["HF_HUB_DOWNLOAD_TIMEOUT"] = "30"
	env["HF_USERNAME"] = req.Username
	env["EPOCHS"] = strconv.Itoa(req.Epochs)
	env["LEARNING_RATE"] = strconv.FormatFloat(req.LearningRate, 'f', -1, 64)
	env["CUSTOM_ARGS"] = req.CustomeArgs
	if len(finetunedModelName) > 0 {
		env["FINETUNED_MODEL_NAME"] = finetunedModelName
	}

	common.UpdateEvaluationEnvHardware(env, req.Hardware)

	templates := []types.ArgoFlowTemplate{}
	templates = append(templates, types.ArgoFlowTemplate{
		Name:     "finetune",
		Env:      env,
		HardWare: req.Hardware,
		Image:    req.Image,
	},
	)
	uniqueFlowName := d.snowflakeNode.Generate().Base36()
	flowReq := &types.ArgoWorkFlowReq{
		TaskName:           req.TaskName,
		TaskId:             uniqueFlowName,
		TaskType:           req.TaskType,
		TaskDesc:           req.TaskDesc,
		Image:              req.Image,
		Username:           req.Username,
		UserUUID:           req.UserUUID,
		Entrypoint:         "finetune",
		ClusterID:          req.ClusterID,
		Templates:          templates,
		Datasets:           []string{req.DatasetId},
		RepoIds:            []string{req.ModelId},
		RepoType:           req.RepoType,
		ResourceId:         req.ResourceId,
		ResourceName:       req.ResourceName,
		FinetunedModelName: finetunedModelName,
	}
	if req.ResourceId == 0 {
		flowReq.ShareMode = true
	}
	slog.Debug("submit finetune workflow request to runner", slog.Any("flowReq", flowReq))
	return d.imageRunner.SubmitFinetuneJob(ctx, flowReq)
}

func (d *deployer) DeleteFinetuneJob(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	_, err := d.imageRunner.DeleteWorkFlow(ctx, req)
	if err != nil {
		return fmt.Errorf("failed delete finetune workflow by runner error: %w", err)
	}
	return nil
}

func (d *deployer) GetWorkflowLogsInStream(ctx context.Context, req types.FinetuneLogReq) (*MultiLogReader, error) {
	slog.Info("GetWorkflowLogsInStream", slog.Any("req", req))
	labels := map[string]string{
		types.StreamKeyInstanceName: req.PodName,
	}

	var startTime = req.SubmitTime
	if len(req.Since) > 0 {
		startTime = parseSinceTime(req.Since)
	}

	runLog, err := d.readLogsFromLoki(ctx, types.ReadLogRequest{
		DeployID:  req.PodName,
		StartTime: startTime,
		Labels:    labels,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get workflow job logs error: %w", err)
	}

	return NewMultiLogReader(nil, runLog), nil
}

func (d *deployer) GetWorkflowLogsNonStream(ctx context.Context, req types.FinetuneLogReq) (*loki.LokiQueryResponse, error) {
	labels := map[string]string{
		types.StreamKeyInstanceName: req.PodName,
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString("{")
	first := true
	for k, v := range labels {
		if !first {
			queryBuilder.WriteString(",")
		}
		queryBuilder.WriteString(fmt.Sprintf(`%s="%s"`, k, v))
		first = false
	}
	queryBuilder.WriteString("}")
	query := queryBuilder.String()

	var startTime = req.SubmitTime
	if req.Since != "" {
		startTime = parseSinceTime(req.Since)
	}

	params := loki.QueryRangeParams{
		Query:     query,
		Start:     startTime,
		Direction: "forward",
	}

	return d.lokiClient.QueryRange(ctx, params)
}
