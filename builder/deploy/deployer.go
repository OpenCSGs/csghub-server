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

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	hubcom "opencsg.com/csghub-server/common/utils/common"
)

type Deployer interface {
	Deploy(ctx context.Context, dr types.DeployRepo) (deployID int64, err error)
	Status(ctx context.Context, dr types.DeployRepo, needDetails bool) (srvName string, status int, instances []types.Instance, err error)
	Logs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error)
	Stop(ctx context.Context, dr types.DeployRepo) (err error)
	Purge(ctx context.Context, dr types.DeployRepo) (err error)
	Wakeup(ctx context.Context, dr types.DeployRepo) (err error)
	Exist(ctx context.Context, dr types.DeployRepo) (bool, error)
	GetReplica(ctx context.Context, dr types.DeployRepo) (int, int, []types.Instance, error)
	InstanceLogs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error)
	ListCluster(ctx context.Context) ([]types.ClusterRes, error)
	GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error)
	UpdateCluster(ctx context.Context, data types.ClusterRequest) (*types.UpdateClusterResponse, error)
	UpdateDeploy(ctx context.Context, dur *types.DeployUpdateReq, deploy *database.Deploy) error
	StartDeploy(ctx context.Context, deploy *database.Deploy) error
	CheckResourceAvailable(ctx context.Context, clusterId string, orderDetailID int64, hardWare *types.HardWare) (bool, error)
	SubmitEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error)
	ListEvaluations(context.Context, string, int, int) (*types.ArgoWorkFlowListRes, error)
	DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
	GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.ArgoWorkFlowRes, error)
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
		return nil, fmt.Errorf("fail to generate uuid for deploy")
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

	go func() { _ = d.scheduler.Queue(buildTask.ID) }()

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
	// get latest Deploy
	deploy, err := d.deployTaskStore.GetLatestDeployBySpaceID(ctx, dr.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("can't get space delopyment,%w", err)
	}

	slog.Debug("get logs for space", slog.Any("deploy", deploy), slog.Int64("space_id", dr.SpaceID))
	buildLog, err := d.imageBuilder.Logs(ctx, &imagebuilder.LogsRequest{
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
	runLog, err := d.imageRunner.Logs(ctx, &types.LogsRequest{
		ID:        targetID,
		OrgName:   dr.Namespace,
		RepoName:  dr.Name,
		SvcName:   deploy.SvcName,
		ClusterID: dr.ClusterID,
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("space endpoint status not ok, status:%d", resp.StatusCode)
	}
	return nil
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
		slog.Error("deploy check result", slog.Any("resp", resp))
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

func (d *deployer) InstanceLogs(ctx context.Context, dr types.DeployRepo) (*MultiLogReader, error) {
	slog.Debug("get logs for deploy", slog.Any("deploy", dr))

	targetID := dr.SpaceID // support space only one instance
	if dr.SpaceID == 0 {
		targetID = dr.DeployID // support model deploy with multi-instance
	}
	runLog, err := d.imageRunner.InstanceLogs(ctx, &types.InstanceLogsRequest{
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
			ClusterID: c.ClusterID,
			Region:    c.Region,
			Zone:      c.Zone,
			Provider:  c.Provider,
			Resources: resources,
		})
	}
	return result, err
}

func (d *deployer) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterRes, error) {
	resp, err := d.imageRunner.GetClusterById(ctx, clusterId)
	if err != nil {
		return nil, err
	}

	// get reserved resources
	resources, err := d.getResources(ctx, clusterId, resp)
	if err != nil {
		return nil, err
	}
	result := types.ClusterRes{
		ClusterID: resp.ClusterID,
		Region:    resp.Region,
		Zone:      resp.Zone,
		Provider:  resp.Provider,
		Resources: resources,
	}
	return &result, err
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

	go func() { _ = d.scheduler.Queue(runTask.ID) }()

	// update resource if it's a order case
	err = d.updateUserResourceByOrder(ctx, deploy)
	if err != nil {
		return err
	}

	return nil
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

func (d *deployer) startAcctFeeing() {
	for {
		resMap := d.getResourceMap()
		slog.Debug("get resources map", slog.Any("resMap", resMap))
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cancel()
		status, err := d.imageRunner.StatusAll(ctxTimeout)
		if err != nil {
			slog.Error("failed to get all service status", slog.Any("error", err))
		}
		for _, svc := range status {
			d.startAcctRequestFee(resMap, svc)
		}
		// accounting interval in min, get from env config
		time.Sleep(time.Duration(d.eventPub.SyncInterval) * time.Minute)
	}
}

func (d *deployer) startAcctRequestFee(resMap map[string]string, svcRes types.StatusResponse) {
	// ignore not ready svc
	if svcRes.Code != common.Running {
		return
	}
	// ignore deploy without sku resource
	if len(svcRes.DeploySku) < 1 {
		return
	}
	resName, exists := resMap[svcRes.DeploySku]
	if !exists {
		slog.Warn("Did not find space resource by id for metering", slog.Any("id", svcRes.DeploySku), slog.Any("deploy_id", svcRes.DeployID), slog.Any("svc_name", svcRes.ServiceName))
		return
	}
	slog.Debug("metering service", slog.Any("svcRes", svcRes))
	sceneType := getValidSceneType(svcRes.DeployType)
	if sceneType == types.SceneUnknow {
		slog.Error("invalid deploy type of service for metering", slog.Any("svcRes", svcRes))
		return
	}

	extra := startAcctRequestFeeExtra(svcRes)
	event := types.METERING_EVENT{
		Uuid:         uuid.New(),
		UserUUID:     svcRes.UserID,
		Value:        int64(d.eventPub.SyncInterval),
		ValueType:    types.TimeDurationMinType,
		Scene:        int(sceneType),
		OpUID:        "",
		ResourceID:   svcRes.DeploySku,
		ResourceName: resName,
		CustomerID:   svcRes.ServiceName,
		CreatedAt:    time.Now(),
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

func getValidSceneType(deployType int) types.SceneType {
	switch deployType {
	case types.SpaceType:
		return types.SceneSpace
	case types.InferenceType:
		return types.SceneModelInference
	case types.FinetuneType:
		return types.SceneModelFinetune
	case types.ServerlessType:
		return types.SceneModelInference
	default:
		return types.SceneUnknow
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
	if !CheckResource(clusterResources, hardWare) {
		return false, fmt.Errorf("required resource is not enough")
	}

	return true, nil
}

func CheckResource(clusterResources *types.ClusterRes, hardware *types.HardWare) bool {
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
func (d *deployer) ListEvaluations(ctx context.Context, username string, per int, page int) (*types.ArgoWorkFlowListRes, error) {
	return d.imageRunner.ListWorkFlows(ctx, username, per, page)
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

func (d *deployer) startServiceConsuming() {
	d.buildStream()
	for {
		consumer, err := d.eventPub.CreateServiceConsumer()
		if err != nil {
			slog.Error("fail to create continuous polling order expired consumer", slog.Any("error", err))
		} else {
			_, err = consumer.Consume(d.serviceUpdateConsumerCallback)
			if err != nil {
				slog.Error("fail to begin consuming order expired message", slog.Any("error", err))
			} else {
				break
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func (d *deployer) buildStream() {
	var i int = 0
	for {
		i++
		err := d.eventPub.BuildServiceStream()
		if err != nil {
			tip := fmt.Sprintf("fail to build deploy service stream for the %d time", i)
			slog.Error(tip, slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

func (d *deployer) serviceUpdateConsumerCallback(msg jetstream.Msg) {
	event, err := d.eventPub.ParseServiceMessageData(msg)
	if err != nil {
		slog.Warn("fail to parse service message", slog.Any("msg", string(msg.Data())))
		err = msg.Ack()
		if err != nil {
			slog.Warn("fail to ack after processing service message", slog.Any("msg", string(msg.Data())))
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deploy, err := d.deployTaskStore.GetDeployBySvcName(ctx, event.ServiceName)
	if err != nil {
		err = msg.Ack()
		if err != nil {
			slog.Warn("fail to ack after processing service message", slog.Any("msg", string(msg.Data())))
		}
		slog.Warn("fail to get deploy by service name in event consumer", slog.Any("service_name", event.ServiceName))
		return
	}
	oldStatus := deploy.Status
	if deploy.Status != common.Deleted {
		deploy.Status = event.Status
		deploy.Message = event.Message
		deploy.Reason = event.Reason
		err = d.deployTaskStore.UpdateDeploy(ctx, deploy)
		if err != nil {
			slog.Warn("fail to update deploy status in event consumer", slog.Any("service_name", event.ServiceName), slog.Any("error", err))
			return
		}
	}
	err = msg.Ack()
	if err != nil {
		slog.Warn("fail to ack after processing service message", slog.Any("msg", string(msg.Data())))
	}

	if event.Status == common.Running && oldStatus != common.Running {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := d.sendNotification(ctx, deploy)
			if err != nil {
				slog.Error("failed to send notification", slog.Any("error", err))
			}
		}()
	}
}

// handle some extreme cases like the runner is down for a long time and svc was deleted by admin
func (d *deployer) startSyncDeployStatus() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		status, err := d.imageRunner.StatusAll(ctx)
		if err != nil {
			slog.Error("fail to get all deploy status", slog.Any("error", err))
			time.Sleep(10 * time.Minute)
			cancel()
			continue
		}
		var req types.DeployReq
		req.Page = 1
		req.PageSize = 300
		for {
			deploys, _, err := d.deployTaskStore.ListAllDeploys(ctx, req, true)
			if err != nil {
				slog.Error("fail to list deploys", slog.Any("error", err))
				break
			}
			if len(deploys) == 0 {
				break
			}
			d.startCheckAndUpdateDeploy(ctx, deploys, status)
			req.Page += 1
		}
		cancel()
		time.Sleep(1 * time.Hour)
	}
}

// The startCheckAndUpdateDeploy function checks and updates the deployment status.
// Parameters:
// - ctx context.Context: Context object for request cancellation and timeout handling.
// - deploys []database.Deploy: List of deployment objects.
// - status map[string]types.StatusResponse: Status response map.
func (d *deployer) startCheckAndUpdateDeploy(ctx context.Context, deploys []database.Deploy, status map[string]types.StatusResponse) {
	for _, deploy := range deploys {
		var newStatus int
		if deploy.Status == common.Deleted {
			//ignore the deleted deploy
			continue
		}
		if _, ok := status[deploy.SvcName]; !ok {
			newStatus = common.Stopped
		} else {
			newStatus = status[deploy.SvcName].Code
		}

		if deploy.Status != newStatus {
			deploy.Status = newStatus
			err := d.deployTaskStore.UpdateDeploy(ctx, &deploy)
			if err != nil {
				slog.Warn("fail to update deploy status in deployer", slog.Any("error", err))
			}
			slog.Info("updated deploy status", slog.Any("deploy_id", deploy.ID), slog.Int("status", newStatus))
		}
	}
}

func (d *deployer) sendNotification(ctx context.Context, deploy *database.Deploy) error {
	payload := map[string]any{
		"deploy_name": deploy.DeployName,
		"deploy_id":   deploy.ID,
		"git_path":    deploy.GitPath,
	}
	// update later after the i18n is ready in the frontend,
	// then just pass the payload and template id in message
	template := getNotificationTemplate(payload, int(deploy.Type))
	payload["deploy_type"] = template.deployType
	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        []string{deploy.UserUUID},
		NotificationType: types.NotificationDeploymentManagement,
		Title:            template.title,
		Content:          template.content,
		CreateAt:         time.Now(),
		ClickActionURL:   template.url,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}
	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioInternalNotification,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := 3
	for i := range retryCount {
		if sendErr = d.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.Warn("failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}

	return nil
}

type notificationTemplate struct {
	title      string
	content    string
	url        string
	deployType string
}

func getNotificationTemplate(payload map[string]any, deployType int) notificationTemplate {
	deployName := payload["deploy_name"].(string)
	gitPath := payload["git_path"].(string)
	deployID := payload["deploy_id"].(int64)

	switch deployType {
	case types.SpaceType:
		return notificationTemplate{
			title:      fmt.Sprintf("Space %s Deployed Successfully", gitPath),
			content:    fmt.Sprintf("Your dedicated space instance %s has been deployed and is running successfully.", gitPath),
			url:        fmt.Sprintf("/spaces/%s", gitPath),
			deployType: "space",
		}
	case types.InferenceType:
		return notificationTemplate{
			title:      fmt.Sprintf("Inference %s/%d Deployed Successfully", deployName, deployID),
			content:    fmt.Sprintf("Your Inference endpoint %s/%d has been deployed and is running successfully.", deployName, deployID),
			url:        fmt.Sprintf("/endpoints/%s/%s/%d", gitPath, deployName, deployID),
			deployType: "inference",
		}
	case types.FinetuneType:
		return notificationTemplate{
			title:   fmt.Sprintf("Finetune %s/%d Deployed Successfully", deployName, deployID),
			content: fmt.Sprintf("Your Finetune instance %s/%d has been deployed and is running successfully.", deployName, deployID),
			url:     fmt.Sprintf("/finetune/%s/%s/%d", gitPath, deployName, deployID),
		}
	case types.EvaluationType:
		return notificationTemplate{
			title:      fmt.Sprintf("Evaluation %s Starts Running", deployName),
			content:    fmt.Sprintf("Your Evaluation task %s starts running successfully.", deployName),
			deployType: "evaluation",
		}
	case types.ServerlessType:
		return notificationTemplate{
			title:      "Serverless Deployed Successfully",
			content:    "Your Serverless instance has been deployed and is running successfully.",
			deployType: "serverless",
		}
	default:
		return notificationTemplate{
			title:   "Instance Deployed Successfully",
			content: "Your instance has been deployed and is running successfully.",
		}
	}
}
