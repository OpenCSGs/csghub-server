package gitaly

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	gitalypb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
)

const timeoutTime = 10 * time.Second

func (c *Client) CreateRepo(ctx context.Context, req gitserver.CreateRepoReq) (*gitserver.CreateRepoResp, error) {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	gitalyReq := &gitalypb.CreateRepositoryRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		DefaultBranch: []byte(req.DefaultBranch),
	}

	_, err := c.repoClient.CreateRepository(ctx, gitalyReq)
	if err != nil {
		return nil, err
	}

	sshCloneURL, err := url.JoinPath(c.config.APIServer.SSHDomain, repoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}
	httpCloneURL, err := url.JoinPath(c.config.APIServer.PublicDomain, repoType, req.Namespace, req.Name)
	if err != nil {
		return nil, err
	}

	return &gitserver.CreateRepoResp{
		Username:      req.Username,
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Nickname,
		Description:   req.Description,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		RepoType:      req.RepoType,
		GitPath:       strings.TrimSuffix(BuildRelativePath(repoType, req.Namespace, req.Name), ".git"),
		SshCloneURL:   sshCloneURL + ".git",
		HttpCloneURL:  httpCloneURL + ".git",
		Private:       req.Private,
	}, nil
}

func (c *Client) UpdateRepo(ctx context.Context, req gitserver.UpdateRepoReq) (*gitserver.CreateRepoResp, error) {
	return nil, nil
}

func (c *Client) DeleteRepo(ctx context.Context, req gitserver.DeleteRepoReq) error {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()
	gitalyReq := &gitalypb.RemoveRepositoryRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
	}
	_, err := c.repoClient.RemoveRepository(ctx, gitalyReq)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetRepo(ctx context.Context, req gitserver.GetRepoReq) (*gitserver.CreateRepoResp, error) {
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()
	gitalyReq := &gitalypb.FindDefaultBranchNameRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
	}
	resp, err := c.refClient.FindDefaultBranchName(ctx, gitalyReq)
	if err != nil {
		return nil, err
	}

	return &gitserver.CreateRepoResp{DefaultBranch: string(resp.Name)}, nil
}
