package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"opencsg.com/csghub-server/builder/deploy"
	deployStatus "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// get runtime framework list with type
func (c *repoComponentImpl) ListRuntimeFrameworkWithType(ctx context.Context, deployType int) ([]types.RuntimeFramework, error) {
	frames, err := c.runtimeFrameworksStore.List(ctx, deployType)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtime frameworks, error: %w", err)
	}
	var frameList []types.RuntimeFramework
	for _, frame := range frames {
		frameList = append(frameList, types.RuntimeFramework{
			ID:            frame.ID,
			FrameName:     frame.FrameName,
			FrameVersion:  frame.FrameVersion,
			FrameImage:    frame.FrameImage,
			Enabled:       frame.Enabled,
			ContainerPort: frame.ContainerPort,
			Type:          frame.Type,
			EngineArgs:    frame.EngineArgs,
			ComputeType:   frame.ComputeType,
			DriverVersion: frame.DriverVersion,
		})
	}
	return frameList, nil
}

// get runtime framework list
func (c *repoComponentImpl) ListRuntimeFramework(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFramework, error) {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	archs := repo.Archs()
	originName := repo.OriginName()
	format := repo.Format()
	frames, err := c.runtimeFrameworksStore.ListByArchsNameAndType(ctx, originName, format, archs, deployType)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtime frameworks, error: %w", err)
	}
	var frameList []types.RuntimeFramework
	for _, modelFrame := range frames {
		frameList = append(frameList, types.RuntimeFramework{
			ID:            modelFrame.ID,
			FrameName:     modelFrame.FrameName,
			FrameVersion:  modelFrame.FrameVersion,
			FrameImage:    modelFrame.FrameImage,
			Enabled:       modelFrame.Enabled,
			ContainerPort: modelFrame.ContainerPort,
			EngineArgs:    modelFrame.EngineArgs,
			ComputeType:   modelFrame.ComputeType,
			DriverVersion: modelFrame.DriverVersion,
			Description:   modelFrame.Description,
			Type:          modelFrame.Type,
		})
	}
	return frameList, nil
}

func (c *repoComponentImpl) ListRuntimeFrameworkV2(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFrameworkV2, error) {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	archs := repo.Archs()
	originName := repo.OriginName()
	format := repo.Format()
	frames, err := c.runtimeFrameworksStore.ListByArchsNameAndType(ctx, originName, format, archs, deployType)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtime frameworks, error: %w", err)
	}
	var frameList []types.RuntimeFrameworkV2
	for _, modelFrame := range frames {
		systemDriverVersion := c.config.Runner.SystemCUDAVersion
		if systemDriverVersion != "" && modelFrame.ComputeType == string(types.ResourceTypeGPU) {
			frameDriverVersion, _ := version.NewVersion(modelFrame.DriverVersion)
			systemDriverVersion, _ := version.NewVersion(systemDriverVersion)
			// ignore unsupported driver version
			if frameDriverVersion.GreaterThan(systemDriverVersion) {
				continue
			}
		}
		exist, index := c.checkFrameNameExist(modelFrame.FrameName, frameList)
		if !exist {
			frameList = append(frameList, types.RuntimeFrameworkV2{
				FrameName: modelFrame.FrameName,
			})
			index = len(frameList) - 1
		}
		frameVersion := strings.Split(modelFrame.FrameImage, ":")[1]
		frameList[index].Versions = append(frameList[index].Versions, types.RuntimeFramework{
			ID:            modelFrame.ID,
			FrameName:     modelFrame.FrameName,
			FrameVersion:  frameVersion,
			FrameImage:    modelFrame.FrameImage,
			Enabled:       modelFrame.Enabled,
			ContainerPort: modelFrame.ContainerPort,
			EngineArgs:    modelFrame.EngineArgs,
			ComputeType:   modelFrame.ComputeType,
			DriverVersion: modelFrame.DriverVersion,
			Description:   modelFrame.Description,
			Type:          modelFrame.Type,
		})
		if !slices.Contains(frameList[index].ComputeTypes, modelFrame.ComputeType) {
			frameList[index].ComputeTypes = append(frameList[index].ComputeTypes, modelFrame.ComputeType)
		}

	}
	// when deploy_type=1 (inference), put vllm first if present
	if deployType == types.InferenceType {
		for i, f := range frameList {
			if f.FrameName == "vllm" && i > 0 {
				// shift vllm to index 0 without allocating new slice
				for j := i; j > 0; j-- {
					frameList[j], frameList[j-1] = frameList[j-1], frameList[j]
				}
				break
			}
		}
	}
	return frameList, nil
}

// check if the frame name is in the list
func (c *repoComponentImpl) checkFrameNameExist(frameName string, frameList []types.RuntimeFrameworkV2) (bool, int) {
	for index, frame := range frameList {
		if frameName == frame.FrameName {
			return true, index
		}
	}
	return false, 0
}

func (c *repoComponentImpl) CreateRuntimeFramework(ctx context.Context, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error) {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, errorx.ErrForbiddenMsg("need admin permission for runtime framework")
	}
	newFrame := database.RuntimeFramework{
		FrameName:     req.FrameName,
		FrameVersion:  req.FrameVersion,
		FrameImage:    req.FrameImage,
		Enabled:       req.Enabled,
		ContainerPort: req.ContainerPort,
		ComputeType:   req.ComputeType,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
	}
	_, err = c.runtimeFrameworksStore.Add(ctx, newFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime framework, error: %w", err)
	}
	frame := &types.RuntimeFramework{
		FrameName:     req.FrameName,
		FrameVersion:  req.FrameVersion,
		FrameImage:    req.FrameImage,
		Enabled:       req.Enabled,
		ContainerPort: req.ContainerPort,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
		ComputeType:   req.ComputeType,
		DriverVersion: req.DriverVersion,
	}
	return frame, nil
}

func (c *repoComponentImpl) UpdateRuntimeFramework(ctx context.Context, id int64, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error) {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, errorx.ErrForbiddenMsg("need admin permission for runtime framework")
	}
	newFrame := database.RuntimeFramework{
		ID:            id,
		FrameName:     req.FrameName,
		FrameVersion:  req.FrameVersion,
		FrameImage:    req.FrameImage,
		Enabled:       req.Enabled,
		ContainerPort: req.ContainerPort,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
	}
	frame, err := c.runtimeFrameworksStore.Update(ctx, newFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to update runtime frameworks, error: %w", err)
	}
	return &types.RuntimeFramework{
		ID:            frame.ID,
		FrameName:     frame.FrameName,
		FrameVersion:  frame.FrameVersion,
		FrameImage:    frame.FrameImage,
		Enabled:       frame.Enabled,
		ContainerPort: frame.ContainerPort,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
	}, nil
}

func (c *repoComponentImpl) DeleteRuntimeFramework(ctx context.Context, currentUser string, id int64) error {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return errorx.ErrForbiddenMsg("need admin permission for runtime framework")
	}
	frame, err := c.runtimeFrameworksStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find runtime frameworks, error: %w", err)
	}
	err = c.runtimeFrameworksStore.Delete(ctx, *frame)
	return err
}

// generate endpoint
func (c *repoComponentImpl) GenerateEndpoint(ctx context.Context, deploy *database.Deploy) (string, string) {
	var endpoint string
	provider := ""
	cls, err := c.clusterInfoStore.ByClusterID(ctx, deploy.ClusterID)
	zone := ""
	if err != nil {
		slog.Warn("Get cluster with error", slog.Any("error", err))
	} else {
		zone = cls.Zone
		provider = cls.Provider
	}
	if len(deploy.SvcName) > 0 && deploy.Status == deployStatus.Running {
		// todo: zone.provider.endpoint to support multi-zone, multi-provider
		regionDomain := ""
		if len(zone) > 0 && len(provider) > 0 {
			regionDomain = fmt.Sprintf(".%s.%s", zone, provider)
		}
		if c.publicRootDomain == "" {
			endpoint, _ = url.JoinPath(c.serverBaseUrl, "endpoint", deploy.SvcName)
			endpoint = strings.Replace(endpoint, "http://", "", 1)
			endpoint = strings.Replace(endpoint, "https://", "", 1)
		} else {
			endpoint = fmt.Sprintf("%s%s.%s", deploy.SvcName, regionDomain, c.publicRootDomain)
		}

	}

	return endpoint, provider
}

// check access repo permission by repo id
func (c *repoComponentImpl) AllowAccessByRepoID(ctx context.Context, repoID int64, username string) (bool, error) {
	r, err := c.repoStore.FindById(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("failed to get repository by repo_id: %d, %w", repoID, err)
	}
	if r == nil {
		return false, fmt.Errorf("invalid repository by repo_id: %d", repoID)
	}
	fields := strings.Split(r.Path, "/")
	return c.AllowReadAccess(ctx, r.RepositoryType, fields[0], fields[1], username)
}

// check access endpoint for rproxy
func (c *repoComponentImpl) AllowAccessEndpoint(ctx context.Context, currentUser string, deploy *database.Deploy) (bool, error) {
	if deploy.SecureLevel == types.EndpointPublic {
		// public endpoint
		return true, nil
	}
	return c.checkAccessDeployForUser(ctx, deploy.RepoID, currentUser, deploy)
}

// check access deploy permission
func (c *repoComponentImpl) AllowAccessDeploy(ctx context.Context, req types.DeployActReq) (bool, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo == nil {
		return false, fmt.Errorf("failed to find %s repo %s/%s", req.RepoType, req.Namespace, req.Name)
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, req.DeployID)
	if err != nil {
		return false, fmt.Errorf("fail to get deploy by ID: %v, %w", req.DeployID, err)
	}
	if deploy == nil {
		return false, fmt.Errorf("deploy not found by ID: %v", req.DeployID)
	}
	if req.DeployType == types.ServerlessType {
		return c.checkAccessDeployForServerless(ctx, repo.ID, req.CurrentUser, deploy)
	} else {
		return c.checkAccessDeployForUser(ctx, repo.ID, req.CurrentUser, deploy)
	}
}

func (c *repoComponentImpl) CheckDeployPermissionForUser(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy permission check user failed, %w", err)
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployReq.DeployID)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get user deploy %v, %w", deployReq.DeployID, err)
	}
	if deploy == nil {
		return nil, nil, fmt.Errorf("do not found user deploy %v", deployReq.DeployID)
	}

	if deploy.UserID == user.ID || c.IsAdminRole(user) || c.IsInSameOrg(ctx, user.ID, deploy.UserID) {
		return &user, deploy, nil
	}
	return nil, nil, errorx.ErrForbiddenMsg("deploy was not created by user")
}

func (c *repoComponentImpl) checkDeployPermissionForServerless(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy permission check user failed, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, nil, errorx.ErrForbiddenMsg("need admin permission for Serverless deploy")
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployReq.DeployID)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get serverless deploy:%v, %w", deployReq.DeployID, err)
	}
	if deploy == nil {
		return nil, nil, fmt.Errorf("do not found serverless deploy %v", deployReq.DeployID)
	}
	return &user, deploy, nil
}

func (c *repoComponentImpl) ListDeploy(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) ([]types.DeployRepo, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		slog.Error("Failed to query deploy", slog.Any("error", err), slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
		return nil, errors.New("invalid repository for query parameters")
	}
	if repo == nil {
		slog.Error("nothing found for deploys", slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
		return nil, errors.New("nothing found for deploys")
	}
	deploys, err := c.deployTaskStore.ListDeploy(ctx, repoType, repo.ID, user.ID)
	if err != nil {
		return nil, errors.New("fail to list user deploys")
	}
	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		resDeploys = append(resDeploys, types.DeployRepo{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			RepoID:           deploy.RepoID,
			SvcName:          deploy.SvcName,
			Status:           deployStatusCodeToString(deploy.Status),
			Hardware:         deploy.Hardware,
			Env:              deploy.Env,
			RuntimeFramework: deploy.RuntimeFramework,
			ImageID:          deploy.ImageID,
			MinReplica:       deploy.MinReplica,
			MaxReplica:       deploy.MaxReplica,
			GitBranch:        deploy.GitBranch,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Task:             string(deploy.Task),
			EngineArgs:       deploy.EngineArgs,
		})
	}
	return resDeploys, nil
}

func (c *repoComponentImpl) DeleteDeploy(ctx context.Context, delReq types.DeployActReq) error {
	if delReq.DeployType == types.ServerlessType {
		repo, err := c.repoStore.FindByPath(ctx, delReq.RepoType, delReq.Namespace, delReq.Name)
		if err != nil {
			return fmt.Errorf("fail to find repo for serverless, %w", err)
		}
		d, err := c.deployTaskStore.GetServerlessDeployByRepID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("fail to get deploy for serverless, %w", err)
		}
		if d != nil {
			delReq.DeployID = d.ID
		} else {
			return fmt.Errorf("no deploy found for serverless type")
		}
	}
	user, deploy, err := c.CheckDeployPermissionForUser(ctx, delReq)
	if err != nil {
		return err
	}

	// delete service
	deployRepo := types.DeployRepo{
		SpaceID:   0,
		DeployID:  delReq.DeployID,
		Namespace: delReq.Namespace,
		Name:      delReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	// purge service
	err = c.deployer.Purge(ctx, deployRepo)
	if err != nil {
		// fail to purge deploy instance, maybe service is gone
		slog.Warn("purge deploy instance", slog.Any("error", err))
	}

	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		//add deploy id and repo in the log
		slog.Warn("fail to check deploy instance exist in remote cluster, will delete deploy instance in database", slog.Any("deploy id", delReq.DeployID), slog.Any("repo", deployRepo.Name))
	}

	if exist && err == nil {
		// fail to delete service
		return errors.New("fail to delete service")
	}

	// update database deploy
	if delReq.DeployType == types.ServerlessType {
		err = c.deployTaskStore.DeleteDeployNow(ctx, delReq.DeployID)
	} else {
		err = c.deployTaskStore.DeleteDeploy(ctx, types.RepositoryType(delReq.RepoType), deploy.RepoID, user.ID, delReq.DeployID)
	}

	if err != nil {
		return fmt.Errorf("fail to remove deploy instance, %w", err)
	}
	// release resource if it's a order case
	if deploy.OrderDetailID != 0 {
		ur, err := c.userResourcesStore.FindUserResourcesByOrderDetailId(ctx, deploy.UserUUID, deploy.OrderDetailID)
		if err != nil {
			return fmt.Errorf("fail to find user resource, %w", err)
		}
		ur.DeployId = 0
		err = c.userResourcesStore.UpdateDeployId(ctx, ur)
		if err != nil {
			return fmt.Errorf("fail to release resource, %w", err)
		}

	}

	return err
}

func (c *repoComponentImpl) DeployDetail(ctx context.Context, detailReq types.DeployActReq) (*types.DeployRepo, error) {
	var (
		deploy *database.Deploy
		err    error
	)
	if detailReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, detailReq)
	} else {
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, detailReq)
	}
	if err != nil {
		return nil, err
	}

	req := types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: detailReq.Namespace,
		Name:      detailReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	actualReplica, desiredReplica, instList, err := c.deployer.GetReplica(ctx, req)
	if err != nil {
		slog.Warn("fail to get deploy replica", slog.Any("repotype", detailReq.RepoType), slog.Any("req", req), slog.Any("error", err))
	}

	_, code, _, err := c.deployer.Status(ctx, types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: detailReq.Namespace,
		Name:      detailReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}, false)
	if err != nil {
		slog.Warn("fail to get deploy status", slog.Any("repo type", detailReq.RepoType), slog.Any("svc name", deploy.SvcName), slog.Any("error", err))
	}

	deploy.Status = code

	endpoint, _ := c.GenerateEndpoint(ctx, deploy)

	endpointPrivate := deploy.SecureLevel != types.EndpointPublic
	proxyEndPoint := ""
	if deploy.Type == types.FinetuneType {
		proxyEndPoint = endpoint + "/proxy/7860/"
	}
	repoPath := strings.TrimPrefix(deploy.GitPath, string(detailReq.RepoType)+"s_")

	varMap, err := common.JsonStrToMap(deploy.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to convert variables to map, error: %w", err)
	}
	var entrypoint string
	val, exist := varMap[types.GGUFEntryPoint]
	if exist {
		entrypoint = val
	}

	// Check if engine_args contains tool-call-parser parameter
	supportFunctionCall := strings.Contains(deploy.EngineArgs, "tool-call-parser")

	resDeploy := types.DeployRepo{
		DeployID:            deploy.ID,
		DeployName:          deploy.DeployName,
		RepoID:              deploy.RepoID,
		SvcName:             deploy.SvcName,
		Status:              deployStatusCodeToString(code),
		Hardware:            deploy.Hardware,
		Env:                 deploy.Env,
		RuntimeFramework:    deploy.RuntimeFramework,
		ImageID:             deploy.ImageID,
		MinReplica:          deploy.MinReplica,
		MaxReplica:          deploy.MaxReplica,
		GitBranch:           deploy.GitBranch,
		ClusterID:           deploy.ClusterID,
		SecureLevel:         deploy.SecureLevel,
		CreatedAt:           deploy.CreatedAt,
		UpdatedAt:           deploy.UpdatedAt,
		Endpoint:            endpoint,
		ActualReplica:       actualReplica,
		DesiredReplica:      desiredReplica,
		Instances:           instList,
		Private:             endpointPrivate,
		Path:                repoPath,
		ProxyEndpoint:       proxyEndPoint,
		SKU:                 deploy.SKU,
		Task:                string(deploy.Task),
		EngineArgs:          deploy.EngineArgs,
		Variables:           deploy.Variables,
		Entrypoint:          entrypoint,
		Reason:              deploy.Reason,
		Message:             deploy.Message,
		SupportFunctionCall: supportFunctionCall,
	}

	return &resDeploy, nil
}

func deployStatusCodeToString(code int) string {
	// Pending    = 0
	// DeployBuildPending    = 10
	// DeployBuildInProgress = 11
	// DeployBuildFailed     = 12
	// DeployBuildSucceed    = 13
	// DeployBuildSkip       = 14
	//
	// DeployPrepareToRun = 20
	// DeployStartUp      = 21
	// DeployRunning      = 22
	// DeployRunTimeError = 23
	// DeployStopped      = 26
	// DeployRunDeleted   = 27 // end user trigger delete action for deploy

	// simplified status for frontend show
	var txt string
	switch code {
	case 0:
		txt = SpaceStatusPending
	case 10:
		txt = SpaceStatusBuilding // need to change it to queue? This requires UI modification as well
	case 11:
		txt = SpaceStatusBuilding
	case 12:
		txt = SpaceStatusBuildFailed
	case 13:
		txt = SpaceStatusDeploying
	case 20:
		txt = SpaceStatusDeploying
	case 21:
		txt = SpaceStatusDeployFailed
	case 22:
		txt = SpaceStatusDeploying
	case 23:
		txt = SpaceStatusRunning
	case 24:
		txt = SpaceStatusRuntimeError
	case 25:
		txt = SpaceStatusSleeping
	case 26:
		txt = SpaceStatusStopped
	case 27:
		txt = RepoStatusDeleted
	case 28:
		txt = ResourceUnhealthy
	default:
		txt = SpaceStatusStopped
	}
	return txt
}

func (c *repoComponentImpl) DeployInstanceLogs(ctx context.Context, logReq types.DeployActReq) (*deploy.MultiLogReader, error) {
	var (
		deploy *database.Deploy
		err    error
	)
	if logReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, logReq)
	} else {
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, logReq)
	}

	if err != nil {
		return nil, err
	}
	return c.deployer.InstanceLogs(ctx, types.DeployRepo{
		DeployID:     deploy.ID,
		SpaceID:      deploy.SpaceID,
		ModelID:      deploy.ModelID,
		Namespace:    logReq.Namespace,
		Name:         logReq.Name,
		ClusterID:    deploy.ClusterID,
		SvcName:      deploy.SvcName,
		InstanceName: logReq.InstanceName,
		Since:        logReq.Since,
		CommitID:     logReq.CommitID,
	})
}

// common check function for apiserver and rproxy
func (c *repoComponentImpl) checkAccessDeployForUser(ctx context.Context, repoID int64, currentUser string, deploy *database.Deploy) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, errors.New("user does not exist")
	}
	if deploy.RepoID != repoID {
		return false, errors.New("invalid deploy found")
	}
	if deploy.UserID == user.ID || c.IsAdminRole(user) || c.IsInSameOrg(ctx, user.ID, deploy.UserID) {
		return true, nil
	}
	return false, errorx.ErrForbiddenMsg("deploy was not created by user")
}

func (c *repoComponentImpl) checkAccessDeployForServerless(ctx context.Context, repoID int64, currentUser string, deploy *database.Deploy) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, fmt.Errorf("user %s does not exist", currentUser)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return false, errorx.ErrForbiddenMsg("need admin permission to see Serverless deploy instances")
	}
	if deploy.RepoID != repoID {
		// deny access for invalid repo
		return false, errors.New("invalid deploy found")
	}
	return true, nil
}

func (c *repoComponentImpl) DeployStop(ctx context.Context, stopReq types.DeployActReq) error {
	var (
		user   *database.User
		deploy *database.Deploy
		err    error
	)
	if stopReq.DeployType == types.ServerlessType {
		user, deploy, err = c.checkDeployPermissionForServerless(ctx, stopReq)
	} else {
		user, deploy, err = c.CheckDeployPermissionForUser(ctx, stopReq)
	}
	if err != nil {
		return fmt.Errorf("fail to check permission for stop deploy, %w", err)
	}

	// delete service
	deployRepo := types.DeployRepo{
		DeployID:      stopReq.DeployID,
		SpaceID:       deploy.SpaceID,
		ModelID:       deploy.ModelID,
		Namespace:     stopReq.Namespace,
		Name:          stopReq.Name,
		SvcName:       deploy.SvcName,
		ClusterID:     deploy.ClusterID,
		OrderDetailID: deploy.OrderDetailID,
		UserUUID:      user.UUID,
	}
	err = c.deployer.Stop(ctx, deployRepo)
	if err != nil {
		// fail to stop deploy instance, maybe service is gone
		slog.Warn("stop deploy instance with error", slog.Any("error", err), slog.Any("stopReq", stopReq))
	}

	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		// fail to check service
		return err
	}

	if exist {
		// fail to delete service
		return errors.New("fail to stop deploy instance")
	}

	// update database deploy to stopped
	err = c.deployTaskStore.StopDeploy(ctx, stopReq.RepoType, deploy.RepoID, deploy.UserID, stopReq.DeployID)
	if err != nil {
		return fmt.Errorf("fail to stop deploy instance, %w", err)
	}

	return err
}

func (c *repoComponentImpl) AllowReadAccessByDeployID(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string, deployID int64) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, errors.New("user does not exist")
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployID)
	if err != nil {
		return false, err
	}
	if deploy == nil {
		return false, errors.New("fail to get deploy by ID")
	}
	if deploy.UserID != user.ID {
		return false, errors.New("deploy was not created by user")
	}
	if deploy.RepoID != repo.ID {
		return false, errors.New("found incorrect repo")
	}
	return c.AllowReadAccessRepo(ctx, repo, currentUser)
}

func (c *repoComponentImpl) DeployStatus(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployID int64) (types.ModelStatusEventData, error) {
	var status types.ModelStatusEventData
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployID)
	if err != nil {
		status.Status = SpaceStatusStopped
		return status, err
	}
	// request deploy status by deploy id
	_, code, instances, err := c.deployer.Status(ctx, types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}, true)
	if err != nil {
		slog.Error("error happen when get deploy status", slog.Any("error", err), slog.String("path", deploy.GitPath))
		status.Status = SpaceStatusStopped
		status.Details = instances
		return status, err
	}
	status.Status = deployStatusCodeToString(code)
	status.Details = instances
	status.Message = deploy.Message
	status.Reason = deploy.Reason
	return status, nil
}

func (c *repoComponentImpl) GetDeployBySvcName(ctx context.Context, svcName string) (*database.Deploy, error) {
	d, err := c.deployTaskStore.GetDeployBySvcName(ctx, svcName)
	if err != nil {
		return nil, fmt.Errorf("failed to get deploy by svc name:%s, %w", svcName, err)
	}
	if d == nil {
		return nil, fmt.Errorf("do not found deploy by svc name:%s", svcName)
	}
	return d, nil
}

func (c *repoComponentImpl) DeployUpdate(ctx context.Context, updateReq types.DeployActReq, req *types.DeployUpdateReq) error {
	var (
		deploy *database.Deploy
		err    error
	)
	if updateReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, updateReq)
	} else {
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, updateReq)
	}
	if err != nil {
		return fmt.Errorf("failed to check permission for update deploy, %w", err)
	}
	// check user balance if resource changed
	if req.ResourceID != nil {
		// don't support switch reserved resource
		if deploy.OrderDetailID != 0 {
			return fmt.Errorf("don't support switch reserved resource so far")
		}
		// resource available only if err is nil, err message should contain
		// the reason why resource is unavailable
		resource, err := c.spaceResourceStore.FindByID(ctx, *req.ResourceID)
		if err != nil {
			return fmt.Errorf("cannot find available resource, %w", err)
		}
		err = c.CheckAccountAndResource(ctx, updateReq.CurrentUser, resource.ClusterID, deploy.OrderDetailID, resource)
		if err != nil {
			return err
		}
		if req.RuntimeFrameworkID == nil {
			frame, err := c.runtimeFrameworksStore.FindEnabledByName(ctx, deploy.RuntimeFramework)
			if err != nil {
				return fmt.Errorf("cannot find available runtime framework by name , %w", err)
			}
			// update runtime image once user changed cpu to gpu
			req.RuntimeFrameworkID = &frame.ID
		}
	}

	if req.ClusterID != nil {
		_, err = c.clusterInfoStore.ByClusterID(ctx, *req.ClusterID)
		if err != nil {
			return fmt.Errorf("invalid cluster %v, %w", *req.ClusterID, err)
		}
	}

	// check service
	deployRepo := types.DeployRepo{
		DeployID:  updateReq.DeployID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: updateReq.Namespace,
		Name:      updateReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		return fmt.Errorf("check deploy exists, err: %w", err)
	}

	if needRestartDeploy(req) && exist {
		// deploy instance is running
		return errors.New("stop deploy first")
	}

	if req.EngineArgs != nil {
		_, err = common.JsonStrToMap(*req.EngineArgs)
		if err != nil {
			return fmt.Errorf("invalid engine args, %w", err)
		}
	}

	// update inference service and keep deploy_id and svc_name unchanged
	err = c.deployer.UpdateDeploy(ctx, req, deploy)
	return err
}

func needRestartDeploy(req *types.DeployUpdateReq) bool {
	if req.ClusterID != nil || req.RuntimeFrameworkID != nil || req.ResourceID != nil ||
		req.MaxReplica != nil || req.MinReplica != nil || req.Env != nil ||
		req.EngineArgs != nil || req.Variables != nil || req.Entrypoint != nil {
		return true
	}
	return false
}

func (c *repoComponentImpl) DeployStart(ctx context.Context, startReq types.DeployActReq) error {
	var (
		deploy *database.Deploy
		err    error
	)
	if startReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, startReq)
	} else {
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, startReq)
	}

	if err != nil {
		return fmt.Errorf("failed to check permission for start deploy, %w", err)
	}
	// check user balance
	resourceId, err := strconv.ParseInt(deploy.SKU, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse resource id, %w", err)
	}
	resource, err := c.spaceResourceStore.FindByID(ctx, resourceId)
	if err != nil {
		return fmt.Errorf("failed to find resource, %w", err)
	}
	// check resource available
	err = c.CheckAccountAndResource(ctx, startReq.CurrentUser, deploy.ClusterID, deploy.OrderDetailID, resource)
	if err != nil {
		return err
	}

	// check service
	deployRepo := types.DeployRepo{
		DeployID:  startReq.DeployID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: startReq.Namespace,
		Name:      startReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		return err
	}

	if exist {
		// check deploy status
		_, status, _, err := c.deployer.Status(ctx, deployRepo, false)
		if err != nil {
			return fmt.Errorf("failed to get deploy status, %w", err)
		}

		// if deploy is in running status, return error
		const deployStatusRunning = 4
		if status == deployStatusRunning {
			return errors.New("stop deploy first")
		}

		// if deploy exists but not running, stop it first
		err = c.deployer.Stop(ctx, deployRepo)
		if err != nil {
			return fmt.Errorf("failed to stop existing deploy, %w", err)
		}
	}

	// start deploy
	err = c.deployer.StartDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("fail to start deploy, %w", err)
	}

	return err
}
