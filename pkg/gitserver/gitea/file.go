package gitea

import "git-devops.opencsg.com/product/community/starhub-server/pkg/types"

func (c *Client) GetModelFileTree(*types.RepoRequest) ([]*types.File, error) {
	return nil, nil
}

func (c *Client) GetDatasetFileTree(*types.RepoRequest) ([]*types.File, error) {
	return nil, nil
}

func (c *Client) GetDatasetFileRaw(*types.RepoRequest) (string, error) {
	return "", nil
}

func (c *Client) GetModelFileRaw(*types.RepoRequest) (string, error) {
	return "", nil
}
