package gitea

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const (
	ModelOrgPrefix   = "models_"
	DatasetOrgPrefix = "datasets_"
	SpaceOrgPrefix   = "spaces_"
	CodeOrgPrefix    = "codes_"
)

func (c *Client) CreateRepo(ctx context.Context, req gitserver.CreateRepoReq) (*gitserver.CreateRepoResp, error) {
	giteaRepo, _, err := c.giteaClient.CreateOrgRepo(
		common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType)),
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
		slog.Error("fail to call gitea to create repository", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, err
	}

	resp := &gitserver.CreateRepoResp{
		Username:      req.Username,
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Nickname,
		Description:   req.Description,
		Labels:        req.Labels,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		RepoType:      req.RepoType,
		GitPath:       giteaRepo.FullName,
		SshCloneURL:   giteaRepo.SSHURL,
		HttpCloneURL:  portalCloneUrl(giteaRepo.CloneURL, req.RepoType, c.config.GitServer.URL, c.config.Frontend.URL),
		Private:       req.Private,
	}

	return resp, nil
}

func (c *Client) CreateModelRepo(req *types.CreateModelReq) (model *database.Model, repo *database.Repository, err error) {
	var urlSlug string
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
	if req.Nickname != "" {
		urlSlug = req.Nickname
	} else {
		urlSlug = giteaRepo.Name
	}

	model = &database.Model{
		UrlSlug:     urlSlug,
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
		RepositoryType: types.ModelRepo,
		SSHCloneURL:    giteaRepo.SSHURL,
		HTTPCloneURL:   portalCloneUrl(giteaRepo.CloneURL, types.ModelRepo, c.config.GitServer.URL, c.config.Frontend.URL),
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

	if req.Nickname != "" {
		model.UrlSlug = req.Nickname
	} else {
		model.UrlSlug = giteaRepo.Name
	}

	model.Path = path
	model.Description = giteaRepo.Description
	model.Private = giteaRepo.Private

	return
}

func (c *Client) CreateDatasetRepo(req *types.CreateDatasetReq) (dataset *database.Dataset, repo *database.Repository, err error) {
	var urlSlug string
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

	if req.Nickname != "" {
		urlSlug = req.Nickname
	} else {
		urlSlug = giteaRepo.Name
	}

	dataset = &database.Dataset{
		UrlSlug:     urlSlug,
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
		RepositoryType: types.DatasetRepo,
		SSHCloneURL:    giteaRepo.SSHURL,
		HTTPCloneURL:   portalCloneUrl(giteaRepo.CloneURL, types.DatasetRepo, c.config.GitServer.URL, c.config.Frontend.URL),
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

	if req.Nickname != "" {
		dataset.UrlSlug = req.Nickname
	} else {
		dataset.UrlSlug = giteaRepo.Name
	}

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
