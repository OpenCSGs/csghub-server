package gitea

import (
	"context"
	"log/slog"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
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
		DefaultBranch: giteaRepo.DefaultBranch,
		RepoType:      req.RepoType,
		GitPath:       giteaRepo.FullName,
		SshCloneURL:   giteaRepo.SSHURL,
		HttpCloneURL:  portalCloneUrl(giteaRepo.CloneURL, req.RepoType, c.config.GitServer.URL, c.config.Frontend.URL),
		Private:       req.Private,
	}

	return resp, nil
}

func (c *Client) UpdateRepo(ctx context.Context, req gitserver.UpdateRepoReq) (*gitserver.CreateRepoResp, error) {
	giteaRepo, _, err := c.giteaClient.EditRepo(
		common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType)),
		req.Name,
		gitea.EditRepoOption{
			Description:   gitea.OptionalString(req.Description),
			Private:       gitea.OptionalBool(req.Private),
			DefaultBranch: gitea.OptionalString(req.DefaultBranch),
		},
	)
	if err != nil {
		slog.Error("fail to call gitea to update repository", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, err
	}

	resp := &gitserver.CreateRepoResp{
		Nickname:      giteaRepo.FullName,
		Description:   giteaRepo.Description,
		DefaultBranch: giteaRepo.DefaultBranch,
		RepoType:      req.RepoType,
		GitPath:       giteaRepo.FullName,
		Private:       giteaRepo.Private,
	}
	return resp, nil
}

func (c *Client) DeleteRepo(ctx context.Context, req gitserver.DeleteRepoReq) error {
	giteaNamespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	_, err := c.giteaClient.DeleteRepo(giteaNamespace, req.Name)
	return err
}

func (c *Client) GetRepo(ctx context.Context, req gitserver.GetRepoReq) (*gitserver.CreateRepoResp, error) {
	giteaRepo, _, err := c.giteaClient.GetRepo(
		common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType)),
		req.Name,
	)
	if err != nil {
		slog.Error("fail to call get gitea repository", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, err
	}

	resp := &gitserver.CreateRepoResp{
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      giteaRepo.FullName,
		Description:   giteaRepo.Description,
		DefaultBranch: giteaRepo.DefaultBranch,
		RepoType:      req.RepoType,
		GitPath:       giteaRepo.FullName,
		SshCloneURL:   giteaRepo.SSHURL,
		HttpCloneURL:  portalCloneUrl(giteaRepo.CloneURL, req.RepoType, c.config.GitServer.URL, c.config.Frontend.URL),
		Private:       giteaRepo.Private,
	}

	return resp, nil
}
