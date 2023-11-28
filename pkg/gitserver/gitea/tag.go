package gitea

import "git-devops.opencsg.com/product/community/starhub-server/pkg/types"

func (c *Client) GetDatasetTags(*types.RepoRequest) ([]*types.DatasetTag, error) {
	return nil, nil
}

func (c *Client) GetModelTags(*types.RepoRequest) ([]*types.ModelTag, error) {
	return nil, nil
}
