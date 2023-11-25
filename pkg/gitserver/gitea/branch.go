package gitea

import "git-devops.opencsg.com/product/community/starhub-server/pkg/types"

func (c *Client) GetModelBranches(*types.RepoRequest) ([]*types.ModelBranch, error) {
	return nil, nil
}

func (c *Client) GetDatasetBranches(*types.RepoRequest) ([]*types.DatasetBranch, error) {
	return nil, nil
}
