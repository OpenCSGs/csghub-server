package gitea

import (
	"code.gitea.io/sdk/gitea"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
)

func (c *Client) CreateModelRepo(req *types.CreateModelReq) (model *types.Model, err error) {
	repo, _, err := c.giteaClient.AdminCreateRepo(
		req.Username,
		gitea.CreateRepoOption{
			Name:          req.Name,
			Description:   req.Description,
			Private:       req.Private,
			IssueLabels:   req.Labels,
			License:       req.License,
			Readme:        req.Readme,
			DefaultBranch: req.DefaultBranch,
		},
	)
	if err != nil {
		return
	}

	model = &types.Model{
		Path:           repo.FullName,
		Name:           repo.Name,
		Description:    repo.Description,
		Private:        repo.Private,
		Labels:         req.Labels,
		License:        req.License,
		DefaultBranch:  repo.DefaultBranch,
		RepositoryType: database.ModelRepo,
	}

	return
}

func (c *Client) UpdateModelRepo(*types.Model) (*types.Model, error) {
	return nil, nil
}

func (c *Client) CreateDatasetRepo(*types.CreateModelReq) (*types.Dataset, error) {
	return nil, nil
}

func (c *Client) UpdateDatasetRepo(*types.Dataset) (*types.Dataset, error) {
	return nil, nil
}
