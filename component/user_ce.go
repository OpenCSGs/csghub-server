//go:build !ee && !saas

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func (c *userComponentImpl) ListDeploys(ctx context.Context, repoType types.RepositoryType, req *types.DeployReq) ([]types.DeployRequest, int, error) {
	deploys, total, err := c.deployTaskStore.ListDeployByOwnerNamespace(ctx, req.CurrentUser, req)
	if err != nil {
		newError := fmt.Errorf("failed to get user deploys for %s with error:%w", repoType, err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRequest
	for _, deploy := range deploys {
		d := &database.Deploy{
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
			Status:    deploy.Status,
		}
		endpoint, provider := c.repoComponent.GenerateEndpoint(ctx, d)
		repoPath := strings.TrimPrefix(deploy.GitPath, string(repoType)+"s_")
		var hardware types.HardWare
		_ = json.Unmarshal([]byte(deploy.Hardware), &hardware)
		resourceType := common.ResourceType(hardware)
		tag := ""
		tags, _ := c.repoStore.TagsWithCategory(ctx, deploy.RepoID, "task")
		if len(tags) > 0 {
			tag = tags[0].Name
		}
		resDeploys = append(resDeploys, types.DeployRequest{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			Path:             repoPath,
			RepoID:           deploy.RepoID,
			SvcName:          deploy.SvcName,
			Status:           deployStatusCodeToString(deploy.Status),
			Hardware:         deploy.Hardware,
			Env:              deploy.Env,
			RuntimeFramework: deploy.RuntimeFramework,
			ImageID:          deploy.ImageID,
			MinReplica:       deploy.MinReplica,
			MaxReplica:       deploy.MaxReplica,
			GitPath:          deploy.GitPath,
			GitBranch:        deploy.GitBranch,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Type:             deploy.Type,
			Endpoint:         endpoint,
			Provider:         provider,
			ResourceType:     string(resourceType),
			RepoTag:          tag,
			Task:             string(deploy.Task),
		})
	}
	return resDeploys, total, nil
}

func (c *userComponentImpl) ListInstances(ctx context.Context, req *types.UserRepoReq) ([]types.DeployRequest, int, error) {
	deploys, total, err := c.deployTaskStore.ListFinetunesByOwnerNamespace(ctx, req.CurrentUser, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user instances error:%w", err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRequest
	for _, deploy := range deploys {
		repoPath := strings.TrimPrefix(deploy.GitPath, "models_")
		resDeploys = append(resDeploys, types.DeployRequest{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			Path:             repoPath,
			RepoID:           deploy.RepoID,
			SvcName:          deploy.SvcName,
			Status:           deployStatusCodeToString(deploy.Status),
			Hardware:         deploy.Hardware,
			Env:              deploy.Env,
			RuntimeFramework: deploy.RuntimeFramework,
			ImageID:          deploy.ImageID,
			MinReplica:       deploy.MinReplica,
			MaxReplica:       deploy.MaxReplica,
			GitPath:          deploy.GitPath,
			GitBranch:        deploy.GitBranch,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Type:             deploy.Type,
		})
	}
	return resDeploys, total, nil
}

func (c *userComponentImpl) CreateUserResource(ctx context.Context, req types.CreateUserResourceReq) error {
	return fmt.Errorf("not implemented")
}

func (c *userComponentImpl) DeleteUserResource(ctx context.Context, username string, orderDetailId int64) error {
	return fmt.Errorf("not implemented")
}

// GetUserResource
func (c *userComponentImpl) GetUserResource(ctx context.Context, req types.GetUserResourceReq) ([]types.UserResourcesResp, int, error) {
	return nil, 0, nil
}

func (c *userComponentImpl) ListNotebooks(ctx context.Context, req *types.DeployReq) ([]types.NotebookRes, int, error) {
	deploys, total, err := c.deployTaskStore.ListDeployByOwnerNamespace(ctx, req.CurrentUser, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user notebooks with error:%w", err)
	}
	var res []types.NotebookRes
	for _, deploy := range deploys {
		image := deploy.ImageID
		imagePairs := strings.Split(image, ":")
		imageVersion := "latest"
		if len(imagePairs) == 2 {
			imageVersion = imagePairs[1]
		}
		d := &database.Deploy{
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
			Status:    deploy.Status,
		}
		endpoint, provider := c.repoComponent.GenerateEndpoint(ctx, d)
		resource := ""
		var hardware types.HardWare
		_ = json.Unmarshal([]byte(deploy.Hardware), &hardware)
		resource, _ = common.GetResourceAndType(hardware)
		res = append(res, types.NotebookRes{
			ID:                      deploy.ID,
			DeployName:              deploy.DeployName,
			Status:                  deployStatusCodeToString(deploy.Status),
			CreatedAt:               deploy.CreatedAt,
			UpdatedAt:               deploy.UpdatedAt,
			RuntimeFramework:        deploy.RuntimeFramework,
			RuntimeFrameworkVersion: imageVersion,
			PayMode:                 types.PayModeFree,
			ClusterID:               deploy.ClusterID,
			ResourceName:            resource,
			Endpoint:                endpoint,
			Provider:                provider,
		})
	}
	return res, total, nil
}

func (c *userComponentImpl) ListNotebooksByNamespace(ctx context.Context, req *types.OrgNotebooksReq) ([]types.NotebookRes, int, error) {
	if req.CurrentUser != "" {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to check namespace permission: %w", err)
		}
		if !canRead {
			return nil, 0, errorx.ErrForbiddenMsg("users do not have permission to view notebooks in this namespace")
		}
	}
	deployReq := &types.DeployReq{
		PageOpts:   types.PageOpts{Page: req.Page, PageSize: req.PageSize},
		DeployType: types.NotebookType,
	}
	deploys, total, err := c.deployTaskStore.ListDeployByOwnerNamespace(ctx, req.Namespace, deployReq)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get namespace notebooks: %w", err)
	}
	var res []types.NotebookRes
	for _, deploy := range deploys {
		image := deploy.ImageID
		imagePairs := strings.Split(image, ":")
		imageVersion := "latest"
		if len(imagePairs) == 2 {
			imageVersion = imagePairs[1]
		}
		d := &database.Deploy{
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
			Status:    deploy.Status,
		}
		endpoint, provider := c.repoComponent.GenerateEndpoint(ctx, d)
		resource := ""
		var hardware types.HardWare
		_ = json.Unmarshal([]byte(deploy.Hardware), &hardware)
		resource, _ = common.GetResourceAndType(hardware)
		res = append(res, types.NotebookRes{
			ID:                      deploy.ID,
			DeployName:              deploy.DeployName,
			Status:                  deployStatusCodeToString(deploy.Status),
			CreatedAt:               deploy.CreatedAt,
			UpdatedAt:               deploy.UpdatedAt,
			RuntimeFramework:        deploy.RuntimeFramework,
			RuntimeFrameworkVersion: imageVersion,
			PayMode:                 types.PayModeFree,
			ClusterID:               deploy.ClusterID,
			ResourceName:            resource,
			Endpoint:                endpoint,
			Provider:                provider,
		})
	}
	return res, total, nil
}
