//go:build !ee && !saas

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *userComponentImpl) ListDeploys(ctx context.Context, repoType types.RepositoryType, req *types.DeployReq) ([]types.DeployRepo, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	deploys, total, err := c.deployTaskStore.ListDeployByUserID(ctx, user.ID, req)
	if err != nil {
		newError := fmt.Errorf("failed to get user deploys for %s with error:%w", repoType, err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRepo
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
		resDeploys = append(resDeploys, types.DeployRepo{
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

func (c *userComponentImpl) ListInstances(ctx context.Context, req *types.UserRepoReq) ([]types.DeployRepo, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	deploys, total, err := c.deployTaskStore.ListInstancesByUserID(ctx, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user instances error:%w", err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		repoPath := strings.TrimPrefix(deploy.GitPath, "models_")
		resDeploys = append(resDeploys, types.DeployRepo{
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
