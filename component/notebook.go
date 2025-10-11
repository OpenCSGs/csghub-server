package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type NotebookComponent interface {
	CreateNotebook(ctx context.Context, req *types.CreateNotebookReq) (*types.NotebookRes, error)
	GetNotebook(ctx context.Context, req *types.GetNotebookReq) (*types.NotebookRes, error)
	DeleteNotebook(ctx context.Context, req *types.DeleteNotebookReq) error
	UpdateNotebook(ctx context.Context, req *types.UpdateNotebookReq) error
	StartNotebook(ctx context.Context, req *types.StartNotebookReq) error
	StopNotebook(ctx context.Context, req *types.StopNotebookReq) error
	Wakeup(ctx context.Context, id int64) error
	StatusNotebook(ctx context.Context, req *types.StatusNotebookReq) (string, error)
	LogsNotebook(ctx context.Context, req *types.StatusNotebookReq) (*deploy.MultiLogReader, error)
}

func NewNotebookComponent(config *config.Config) (NotebookComponent, error) {
	c := &notebookComponentImpl{}
	c.deployTaskStore = database.NewDeployTaskStore()
	c.deployer = deploy.NewDeployer()
	c.userStore = database.NewUserStore()
	c.runtimeFrameworksStore = database.NewRuntimeFrameworksStore()
	c.spaceResourceStore = database.NewSpaceResourceStore()
	repoComponent, err := NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	c.repoComponent = repoComponent
	return c, nil
}

type notebookComponentImpl struct {
	deployTaskStore        database.DeployTaskStore
	deployer               deploy.Deployer
	userStore              database.UserStore
	runtimeFrameworksStore database.RuntimeFrameworksStore
	spaceResourceStore     database.SpaceResourceStore
	repoComponent          RepoComponent
}

func (c *notebookComponentImpl) CreateNotebook(ctx context.Context, req *types.CreateNotebookReq) (*types.NotebookRes, error) {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for notebook creation, %w", err)
	}

	frame, err := c.runtimeFrameworksStore.FindEnabledByID(ctx, req.RuntimeFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("cannot find available runtime framework, %w", err)
	}

	// put repo-type and namespace/name in annotation
	annotations := make(map[string]string)
	annotations[types.ResDeployUser] = user.Username
	annoStr, err := json.Marshal(annotations)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}

	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("cannot find resource, %w", err)
	}

	// resource available only if err is nil, err message should contain
	// the reason why resource is unavailable
	err = c.repoComponent.CheckAccountAndResource(ctx, req.CurrentUser, resource.ClusterID, req.OrderDetailID, resource)
	if err != nil {
		return nil, err
	}

	// choose image
	var hardware types.HardWare
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}

	// notebook only support single instance now
	if hardware.Replicas > 1 {
		return nil, errorx.ErrMultiHostNotebookNotSupported
	}

	// create deploy for notebook
	dp := types.DeployRepo{
		DeployName:       req.DeployName,
		SpaceID:          0,
		Hardware:         resource.Resources,
		UserID:           user.ID,
		RuntimeFramework: frame.FrameName,
		ContainerPort:    frame.ContainerPort,
		ImageID:          frame.FrameImage,
		MinReplica:       req.MinReplica,
		MaxReplica:       1,
		Annotation:       string(annoStr),
		ClusterID:        resource.ClusterID,
		SecureLevel:      2,
		Type:             types.NotebookType,
		UserUUID:         user.UUID,
		OrderDetailID:    req.OrderDetailID,
		SKU:              strconv.FormatInt(resource.ID, 10),
	}

	deployID, err := c.deployer.Deploy(ctx, dp)
	if err != nil {
		return nil, fmt.Errorf("cannot deploy notebook, %w", err)
	}
	return &types.NotebookRes{
		ID: deployID,
	}, nil
}

// GetNotebook
func (c *notebookComponentImpl) GetNotebook(ctx context.Context, req *types.GetNotebookReq) (*types.NotebookRes, error) {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return nil, err
	}
	reqReplica := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	_, _, instList, err := c.deployer.GetReplica(ctx, reqReplica)
	if err != nil {
		slog.Warn("fail to get deploy replica", slog.Any("req", req), slog.Any("error", err))
	}
	endpoint, _ := c.repoComponent.GenerateEndpoint(ctx, deploy)
	image := deploy.ImageID
	imagePairs := strings.Split(image, ":")
	imageVersion := "latest"
	if len(imagePairs) == 2 {
		imageVersion = imagePairs[1]
	}
	resource := ""
	var hardware types.HardWare
	_ = json.Unmarshal([]byte(deploy.Hardware), &hardware)
	resource, _ = common.GetResourceAndType(hardware)
	return &types.NotebookRes{
		ID:                      deploy.ID,
		DeployName:              deploy.DeployName,
		Status:                  deployStatusCodeToString(deploy.Status),
		ClusterID:               deploy.ClusterID,
		Endpoint:                endpoint,
		ResourceID:              deploy.SKU,
		ResourceName:            resource,
		MinReplica:              deploy.MinReplica,
		MaxReplica:              deploy.MaxReplica,
		Instances:               instList,
		SvcName:                 deploy.SvcName,
		RuntimeFramework:        deploy.RuntimeFramework,
		CreatedAt:               deploy.CreatedAt,
		UpdatedAt:               deploy.UpdatedAt,
		SecureLevel:             deploy.SecureLevel,
		RuntimeFrameworkVersion: imageVersion,
	}, nil
}

// StatusNotebook
func (c *notebookComponentImpl) StatusNotebook(ctx context.Context, req *types.StatusNotebookReq) (string, error) {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return "", fmt.Errorf("cannot find notebook for status check, %w", err)
	}
	return deployStatusCodeToString(deploy.Status), nil
}

func (c *notebookComponentImpl) LogsNotebook(ctx context.Context, req *types.StatusNotebookReq) (*deploy.MultiLogReader, error) {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return nil, fmt.Errorf("cannot find notebook for status check, %w", err)
	}
	return c.deployer.InstanceLogs(ctx, types.DeployRepo{
		DeployID:     deploy.ID,
		InstanceName: req.InstanceName,
	})
}

// DeleteNotebook
func (c *notebookComponentImpl) DeleteNotebook(ctx context.Context, req *types.DeleteNotebookReq) error {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	user, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return err
	}
	// delete service
	deployRepo := types.DeployRepo{
		SpaceID:   0,
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	// purge service
	err = c.deployer.Purge(ctx, deployRepo)
	if err != nil {
		slog.Warn("faile to purge notebook instance", slog.Any("error", err))
	}
	err = c.deployTaskStore.DeleteDeployByID(ctx, user.ID, deploy.ID)
	if err != nil {
		return fmt.Errorf("cannot delete notebook, %w", err)
	}
	return nil
}

// UpdateNotebook
func (c *notebookComponentImpl) UpdateNotebook(ctx context.Context, req *types.UpdateNotebookReq) error {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return fmt.Errorf("cannot find deploy for notebook, %w", err)
	}

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		return fmt.Errorf("check deploy exists, err: %w", err)
	}
	if exist {
		// deploy instance is running
		return errors.New("stop deploy first")
	}

	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceID)
	if err != nil {
		return fmt.Errorf("cannot find resource, %w", err)
	}

	err = c.repoComponent.CheckAccountAndResource(ctx, req.CurrentUser, resource.ClusterID, deploy.OrderDetailID, resource)
	if err != nil {
		return fmt.Errorf("resource is not available, %w", err)
	}

	var hardware types.HardWare
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return errorx.InternalServerError(err, nil)
	}

	// notebook only support single instance now
	if hardware.Replicas > 1 {
		return errorx.ErrMultiHostNotebookNotSupported
	}

	dur := &types.DeployUpdateReq{
		ResourceID: &req.ResourceID,
	}

	err = c.deployer.UpdateDeploy(ctx, dur, deploy)
	return err

}

// StartNotebook
func (c *notebookComponentImpl) StartNotebook(ctx context.Context, req *types.StartNotebookReq) error {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	_, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return fmt.Errorf("cannot find deploy for notebook, %w", err)
	}

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		return fmt.Errorf("check notebook exists, err: %w", err)
	}
	if exist {
		// deploy instance is running
		return errors.New("deploy already started")
	}

	// start deploy
	err = c.deployer.StartDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("fail to start notebook, %w", err)
	}
	return nil
}

// StopNotebook
func (c *notebookComponentImpl) StopNotebook(ctx context.Context, req *types.StopNotebookReq) error {
	deployReq := &types.DeployActReq{
		DeployID:    req.ID,
		CurrentUser: req.CurrentUser,
	}
	user, deploy, err := c.repoComponent.CheckDeployPermissionForUser(ctx, *deployReq)
	if err != nil {
		return err
	}

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	// stop deploy
	err = c.deployer.Stop(ctx, deployRepo)
	if err != nil {
		return fmt.Errorf("fail to stop notebook, %w", err)
	}
	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		// fail to check service
		return err
	}

	if exist {
		// fail to delete service
		return errors.New("fail to stop notebook instance")
	}
	// update database deploy to stopped
	err = c.deployTaskStore.StopDeployByID(ctx, user.ID, deploy.ID)
	if err != nil {
		return fmt.Errorf("fail to stop notebook instance, %w", err)
	}
	return nil
}

func (c *notebookComponentImpl) Wakeup(ctx context.Context, deployId int64) error {
	// get Deploy for inference
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployId)
	if err != nil {
		return fmt.Errorf("can't get notebook delopyment,%w", err)
	}
	return c.deployer.Wakeup(ctx, types.DeployRepo{
		DeployID: deployId,
		SvcName:  deploy.SvcName,
	})
}
