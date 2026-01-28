package gitaly

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/errorx"
)

func (c *Client) CreateMirrorRepo(ctx context.Context, req gitserver.CreateMirrorRepoReq) (int64, error) {
	var (
		remoteCheckReq *gitalypb.FindRemoteRepositoryRequest
		authorHeader   string
		err            error
	)
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if req.MirrorToken == "" {
		remoteCheckReq = &gitalypb.FindRemoteRepositoryRequest{
			Remote:      req.CloneUrl,
			StorageName: c.config.GitalyServer.Storage,
		}

		resp, err := c.remoteClient.FindRemoteRepository(ctx, remoteCheckReq)
		if err != nil {
			return 0, err
		}
		if !resp.Exists {
			return 0, fmt.Errorf("invalid clone url")
		}
	}

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return 0, err
	}
	gitalyReq := &gitalypb.CreateRepositoryFromURLRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
		Url:    req.CloneUrl,
		Mirror: true,
	}

	if req.Username != "" && req.AccessToken != "" {
		authorHeader = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", req.Username, req.AccessToken)))
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
		return 0, errorx.ErrGitCreateMirrorFailed(err, errorx.Ctx())
	}
	return 0, nil
}

func (c *Client) CreateMirrorForExistsRepo(ctx context.Context, req gitserver.CreateMirrorRepoReq) error {
	var authorHeader string
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	fetchRemoteReq := &gitalypb.FetchRemoteRequest{
		Repository: &gitalypb.Repository{
			StorageName:  c.config.GitalyServer.Storage,
			RelativePath: relativePath,
		},
		Force:   true,
		NoPrune: true,
		RemoteParams: &gitalypb.Remote{
			Url: req.CloneUrl,
		},
	}

	if req.Username != "" && req.AccessToken != "" {
		authorHeader = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", req.Username, req.AccessToken)))
	}

	if authorHeader != "" {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = authorHeader
	}

	_, err = c.repoClient.FetchRemote(ctx, fetchRemoteReq)
	if err != nil {
		return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
	}
	return nil
}

func (c *Client) GetMirrorTaskInfo(ctx context.Context, taskId int64) (*gitserver.MirrorTaskInfo, error) {
	return nil, nil
}

func (c *Client) MirrorSync(ctx context.Context, req gitserver.MirrorSyncReq) error {
	stagingPrefix := "refs/staging"
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	relativePath, err := c.BuildRelativePath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return err
	}
	gitalyRepo := &gitalypb.Repository{
		StorageName:  c.config.GitalyServer.Storage,
		RelativePath: relativePath,
	}
	fetchRemoteReq := &gitalypb.FetchRemoteRequest{
		Repository: gitalyRepo,
		Force:      true,
		NoPrune:    false,
		RemoteParams: &gitalypb.Remote{
			Url: req.CloneUrl,
			MirrorRefmaps: []string{
				"+refs/*:" + stagingPrefix + "/*",
			},
		},
		CheckTagsChanged: true,
	}

	if req.MirrorToken != "" {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = fmt.Sprintf("X-OPENCSG-Sync-Token%s", req.MirrorToken)
	} else {
		fetchRemoteReq.RemoteParams.HttpAuthorizationHeader = ""
	}

	_, err = c.repoClient.FetchRemote(ctx, fetchRemoteReq)
	if err != nil {
		return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
	}
	refsClient, err := c.refClient.ListRefs(ctx, &gitalypb.ListRefsRequest{
		Repository: gitalyRepo,
		Patterns:   [][]byte{[]byte(stagingPrefix + "/")},
	})
	if err != nil {
		return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
	}
	var refs []*gitalypb.ListRefsResponse_Reference
	for {
		resp, err := refsClient.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
		}
		if resp == nil {
			break
		}
		refs = append(refs, resp.References...)
	}

	updateRefClient, err := c.refClient.UpdateReferences(ctx)
	if err != nil {
		return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
	}

	updates := []*gitalypb.UpdateReferencesRequest_Update{}

	for _, r := range refs {
		localRef := strings.Replace(string(r.Name), stagingPrefix, "refs", 1)
		updates = append(updates, &gitalypb.UpdateReferencesRequest_Update{
			Reference:   []byte(localRef),
			NewObjectId: []byte(r.Target),
		})
	}
	err = updateRefClient.Send(&gitalypb.UpdateReferencesRequest{
		Repository: gitalyRepo,
		Updates:    updates,
	})
	if err != nil {
		return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
	}
	_, err = updateRefClient.CloseAndRecv()
	if err != nil {
		return errorx.ErrGitMirrorSyncFailed(err, errorx.Ctx())
	}

	return nil
}
