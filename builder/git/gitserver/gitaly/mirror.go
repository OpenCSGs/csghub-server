package gitaly

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
)

func (c *Client) CreateMirrorRepo(ctx context.Context, req gitserver.CreateMirrorRepoReq) (int64, error) {
	var (
		remoteCheckReq *gitalypb.FindRemoteRepositoryRequest
		authorHeader   string
		err            error
	)
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	if req.MirrorToken == "" {
		remoteCheckReq = &gitalypb.FindRemoteRepositoryRequest{
			Remote:      req.CloneUrl,
			StorageName: c.config.GitalyServer.Storge,
		}

		resp, err := c.remoteClient.FindRemoteRepository(ctx, remoteCheckReq)
		if err != nil {
			return 0, err
		}
		if !resp.Exists {
			return 0, fmt.Errorf("invalid clone url")
		}
		authorHeader = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", req.Username, req.AccessToken)))
	}

	gitalyReq := &gitalypb.CreateRepositoryFromURLRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Url:    req.CloneUrl,
		Mirror: true,
	}
	if req.MirrorToken != "" {
		gitalyReq.HttpAuthorizationHeader = fmt.Sprintf("X-OPENCSG-Sync-Token%s", req.MirrorToken)
	} else if authorHeader != "" {
		gitalyReq.HttpAuthorizationHeader = authorHeader
	} else {
		gitalyReq.HttpAuthorizationHeader = ""
	}
	_, err = c.repoClient.CreateRepositoryFromURL(ctx, gitalyReq)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (c *Client) CreateMirrorForExistsRepo(ctx context.Context, req gitserver.CreateMirrorRepoReq) error {
	var authorHeader string
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, timeoutTime)
	defer cancel()

	fetchRemoteReq := &gitalypb.FetchRemoteRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Force:   true,
		NoPrune: true,
		RemoteParams: &gitalypb.Remote{
			Url: req.CloneUrl,
		},
	}

	if req.MirrorToken != "" {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = fmt.Sprintf("X-OPENCSG-Sync-Token%s", req.MirrorToken)
	} else if authorHeader != "" {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = authorHeader
	} else {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = ""
	}

	_, err := c.repoClient.FetchRemote(ctx, fetchRemoteReq)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetMirrorTaskInfo(ctx context.Context, taskId int64) (*gitserver.MirrorTaskInfo, error) {
	return nil, nil
}

func (c *Client) MirrorSync(ctx context.Context, req gitserver.MirrorSyncReq) error {
	var authorHeader string
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	fetchRemoteReq := &gitalypb.FetchRemoteRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storge,
			RelativePath: BuildRelativePath(repoType, req.Namespace, req.Name),
		},
		Force:   true,
		NoPrune: false,
		RemoteParams: &gitalypb.Remote{
			Url: req.CloneUrl,
		},
		CheckTagsChanged: true,
	}

	if req.MirrorToken != "" {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = fmt.Sprintf("X-OPENCSG-Sync-Token%s", req.MirrorToken)
	} else if authorHeader != "" {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = authorHeader
	} else {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = ""
	}

	_, err := c.repoClient.FetchRemote(ctx, fetchRemoteReq)
	if err != nil {
		return err
	}

	return nil
}
