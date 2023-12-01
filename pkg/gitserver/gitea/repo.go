package gitea

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/pulltheflower/gitea-go-sdk/gitea"
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

func (c *Client) UpdateModelRepo(owner string, repoPath string, repo *types.Model, req *types.UpdateModelReq) (*types.Model, error) {
	giteaRepo, _, err := c.giteaClient.EditRepo(
		owner,
		repoPath,
		gitea.EditRepoOption{
			Name:          gitea.OptionalString(req.Name),
			Description:   gitea.OptionalString(req.Description),
			Private:       gitea.OptionalBool(req.Private),
			DefaultBranch: gitea.OptionalString(req.DefaultBranch),
		},
	)

	if err != nil {
		return nil, err
	}

	repo.Name = giteaRepo.Name
	repo.Path = giteaRepo.FullName
	repo.Description = giteaRepo.Description
	repo.Private = giteaRepo.Private
	repo.DefaultBranch = giteaRepo.DefaultBranch

	return repo, nil
}

func (c *Client) CreateDatasetRepo(req *types.CreateDatasetReq) (dataset *types.Dataset, err error) {
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

	dataset = &types.Dataset{
		Path:           repo.FullName,
		Name:           repo.Name,
		Description:    repo.Description,
		Private:        repo.Private,
		Labels:         req.Labels,
		License:        req.License,
		DefaultBranch:  repo.DefaultBranch,
		RepositoryType: database.DatasetRepo,
	}

	return
}

func (c *Client) UpdateDatasetRepo(owner string, repoPath string, repo *types.Dataset, req *types.UpdateDatasetReq) (*types.Dataset, error) {
	giteaRepo, _, err := c.giteaClient.EditRepo(
		owner,
		repoPath,
		gitea.EditRepoOption{
			Name:          gitea.OptionalString(req.Name),
			Description:   gitea.OptionalString(req.Description),
			Private:       gitea.OptionalBool(req.Private),
			DefaultBranch: gitea.OptionalString(req.DefaultBranch),
		},
	)

	if err != nil {
		return nil, err
	}

	repo.Name = giteaRepo.Name
	repo.Path = giteaRepo.FullName
	repo.Description = giteaRepo.Description
	repo.Private = giteaRepo.Private
	repo.DefaultBranch = giteaRepo.DefaultBranch

	return repo, nil
}

func (c *Client) DeleteModelRepo(username, name string) error {
	_, err := c.giteaClient.DeleteRepo(username, name)
	return err
}

func (c *Client) DeleteDatasetRepo(username, name string) error {
	_, err := c.giteaClient.DeleteRepo(username, name)
	return err
}
