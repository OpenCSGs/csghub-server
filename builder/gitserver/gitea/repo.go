package gitea

import (
	"fmt"

	"github.com/pulltheflower/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

const (
	ModelOrgPrefix   = "models_"
	DatasetOrgPrefix = "datasets_"
	SpaceOrgPrefix   = "spaces_"
)

func (c *Client) CreateModelRepo(req *types.CreateModelReq) (model *database.Model, repo *database.Repository, err error) {
	giteaRepo, _, err := c.giteaClient.CreateOrgRepo(
		common.WithPrefix(req.Namespace, ModelOrgPrefix),
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

	model = &database.Model{
		UrlSlug:     giteaRepo.Name,
		Path:        fmt.Sprintf("%s/%s", req.Namespace, req.Name),
		GitPath:     giteaRepo.FullName,
		Name:        giteaRepo.Name,
		Description: giteaRepo.Description,
		Private:     giteaRepo.Private,
	}

	repo = &database.Repository{
		Path:           fmt.Sprintf("%s/%s", req.Namespace, req.Name),
		GitPath:        giteaRepo.FullName,
		Name:           giteaRepo.Name,
		Description:    giteaRepo.Description,
		Private:        giteaRepo.Private,
		Labels:         req.Labels,
		License:        req.License,
		DefaultBranch:  giteaRepo.DefaultBranch,
		RepositoryType: database.ModelRepo,
		SSHCloneURL:    giteaRepo.SSHURL,
		HTTPCloneURL:   common.PortalCloneUrl(giteaRepo.CloneURL, ModelOrgPrefix),
	}

	return
}

func (c *Client) UpdateModelRepo(
	namespace string,
	repoPath string,
	model *database.Model,
	repo *database.Repository,
	req *types.UpdateModelReq,
) (err error) {
	giteaRepo, _, err := c.giteaClient.EditRepo(
		common.WithPrefix(namespace, ModelOrgPrefix),
		repoPath,
		gitea.EditRepoOption{
			Name:          gitea.OptionalString(req.Name),
			Description:   gitea.OptionalString(req.Description),
			Private:       gitea.OptionalBool(req.Private),
			DefaultBranch: gitea.OptionalString(req.DefaultBranch),
		},
	)

	if err != nil {
		return
	}
	path := fmt.Sprintf("%s/%s", namespace, giteaRepo.Name)

	repo.Name = giteaRepo.Name
	repo.Path = path
	repo.GitPath = giteaRepo.FullName
	repo.Description = giteaRepo.Description
	repo.Private = giteaRepo.Private
	repo.DefaultBranch = giteaRepo.DefaultBranch

	model.Name = giteaRepo.Name
	model.GitPath = giteaRepo.FullName
	model.UrlSlug = giteaRepo.Name
	model.Path = path
	model.Description = giteaRepo.Description
	model.Private = giteaRepo.Private

	return
}

func (c *Client) CreateDatasetRepo(req *types.CreateDatasetReq) (dataset *database.Dataset, repo *database.Repository, err error) {
	giteaRepo, _, err := c.giteaClient.CreateOrgRepo(
		common.WithPrefix(req.Namespace, DatasetOrgPrefix),
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

	dataset = &database.Dataset{
		UrlSlug:     giteaRepo.Name,
		Path:        fmt.Sprintf("%s/%s", req.Namespace, req.Name),
		GitPath:     giteaRepo.FullName,
		Name:        giteaRepo.Name,
		Description: giteaRepo.Description,
		Private:     giteaRepo.Private,
	}

	repo = &database.Repository{
		Path:           fmt.Sprintf("%s/%s", req.Namespace, req.Name),
		GitPath:        giteaRepo.FullName,
		Name:           giteaRepo.Name,
		Description:    giteaRepo.Description,
		Private:        giteaRepo.Private,
		Labels:         req.Labels,
		License:        req.License,
		DefaultBranch:  giteaRepo.DefaultBranch,
		RepositoryType: database.DatasetRepo,
		SSHCloneURL:    giteaRepo.SSHURL,
		HTTPCloneURL:   common.PortalCloneUrl(giteaRepo.CloneURL, ModelOrgPrefix),
	}

	return
}

func (c *Client) UpdateDatasetRepo(
	namespace string,
	repoPath string,
	dataset *database.Dataset,
	repo *database.Repository,
	req *types.UpdateDatasetReq,
) (err error) {
	giteaRepo, _, err := c.giteaClient.EditRepo(
		common.WithPrefix(namespace, DatasetOrgPrefix),
		repoPath,
		gitea.EditRepoOption{
			Name:          gitea.OptionalString(req.Name),
			Description:   gitea.OptionalString(req.Description),
			Private:       gitea.OptionalBool(req.Private),
			DefaultBranch: gitea.OptionalString(req.DefaultBranch),
		},
	)

	if err != nil {
		return
	}

	path := fmt.Sprintf("%s/%s", namespace, giteaRepo.Name)

	repo.Name = giteaRepo.Name
	repo.Path = path
	repo.GitPath = giteaRepo.FullName
	repo.Description = giteaRepo.Description
	repo.Private = giteaRepo.Private
	repo.DefaultBranch = giteaRepo.DefaultBranch

	dataset.Name = giteaRepo.Name
	dataset.GitPath = giteaRepo.FullName
	dataset.UrlSlug = giteaRepo.Name
	dataset.Path = path
	dataset.Description = giteaRepo.Description
	dataset.Private = giteaRepo.Private

	return
}

func (c *Client) DeleteModelRepo(namespace, name string) error {
	giteaNamespace := common.WithPrefix(namespace, ModelOrgPrefix)
	_, err := c.giteaClient.DeleteRepo(giteaNamespace, name)
	return err
}

func (c *Client) DeleteDatasetRepo(namespace, name string) error {
	giteaNamespace := common.WithPrefix(namespace, DatasetOrgPrefix)
	_, err := c.giteaClient.DeleteRepo(giteaNamespace, name)
	return err
}
