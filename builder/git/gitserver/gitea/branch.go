package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) GetRepoBranches(ctx context.Context, req gitserver.GetBranchesReq) ([]types.Branch, error) {
	var branches []types.Branch
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	giteaBranches, _, err := c.giteaClient.ListRepoBranches(
		namespace,
		req.Name,
		gitea.ListRepoBranchesOptions{
			ListOptions: gitea.ListOptions{
				PageSize: req.Per,
				Page:     req.Page,
			},
		},
	)
	for _, giteaBranch := range giteaBranches {
		branches = append(branches, types.Branch{
			Name:    giteaBranch.Name,
			Message: giteaBranch.Commit.Message,
			Commit: types.ModelBranchCommit{
				ID: giteaBranch.Commit.ID,
			},
		})
	}
	return branches, err
}
func (c *Client) GetModelBranches(namespace, name string, per, page int) (branches []*types.ModelBranch, err error) {
	namespace = common.WithPrefix(namespace, ModelOrgPrefix)
	giteaBranches, _, err := c.giteaClient.ListRepoBranches(
		namespace,
		name,
		gitea.ListRepoBranchesOptions{
			ListOptions: gitea.ListOptions{
				PageSize: per,
				Page:     page,
			},
		},
	)
	for _, giteaBranch := range giteaBranches {
		branches = append(branches, &types.ModelBranch{
			Name:    giteaBranch.Name,
			Message: giteaBranch.Commit.Message,
			Commit: types.ModelBranchCommit{
				ID: giteaBranch.Commit.ID,
			},
		})
	}
	return
}

func (c *Client) GetDatasetBranches(namespace, name string, per, page int) (branches []*types.DatasetBranch, err error) {
	namespace = common.WithPrefix(namespace, DatasetOrgPrefix)
	giteaBranches, _, err := c.giteaClient.ListRepoBranches(
		namespace,
		name,
		gitea.ListRepoBranchesOptions{
			ListOptions: gitea.ListOptions{
				PageSize: per,
				Page:     page,
			},
		},
	)
	for _, giteaBranch := range giteaBranches {
		branches = append(branches, &types.DatasetBranch{
			Name:    giteaBranch.Name,
			Message: giteaBranch.Commit.Message,
			Commit: types.DatasetBranchCommit{
				ID: giteaBranch.Commit.ID,
			},
		})
	}
	return
}
