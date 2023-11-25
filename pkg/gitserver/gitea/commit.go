package gitea

import "git-devops.opencsg.com/product/community/starhub-server/pkg/types"

func (c *Client) GetModelCommits(*types.RepoRequest) ([]*types.Commit, error) {
	return nil, nil
}

func (c *Client) GetModelLastCommit(*types.RepoRequest) (*types.Commit, error) {
	return nil, nil
}

func (c *Client) GetDatasetCommits(*types.RepoRequest) ([]*types.Commit, error) {
	return nil, nil
}

func (c *Client) GetDatasetLastCommit(*types.RepoRequest) (*types.Commit, error) {
	return nil, nil
}
