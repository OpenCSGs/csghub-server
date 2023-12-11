package gitea

import (
	"github.com/pulltheflower/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

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
