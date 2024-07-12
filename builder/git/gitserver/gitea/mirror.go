package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) CreateMirrorRepo(ctx context.Context, req gitserver.CreateMirrorRepoReq) (int64, error) {
	task, _, err := c.giteaClient.MigrateRepo(gitea.MigrateRepoOption{
		RepoName:       req.Name,
		RepoOwner:      common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType)),
		CloneAddr:      req.CloneUrl,
		Service:        gitea.GitServicePlain,
		AuthUsername:   req.Username,
		AuthPassword:   req.AccessToken,
		Mirror:         true,
		Private:        req.Private,
		Description:    req.Description,
		Wiki:           false,
		Milestones:     false,
		Labels:         false,
		Issues:         false,
		PullRequests:   false,
		Releases:       false,
		LFS:            true,
		MirrorInterval: "0",
		Token:          req.MirrorToken,
	})
	if err != nil {
		return 0, err
	}
	return task.ID, nil
}

func (c *Client) GetMirrorTaskInfo(ctx context.Context, taskId int64) (*gitserver.MirrorTaskInfo, error) {
	ts, _, err := c.giteaClient.GetUserTaskInfo(taskId)
	if err != nil {
		return nil, err
	}

	mti := &gitserver.MirrorTaskInfo{
		Status:    gitserver.TaskStatus(ts.Status),
		Message:   ts.Message,
		RepoID:    ts.RepoID,
		RepoName:  ts.RepoName,
		StartedAt: ts.StartedAt,
		EndedAt:   ts.EndedAt,
	}
	return mti, nil
}

func (c *Client) MirrorSync(ctx context.Context, req gitserver.MirrorSyncReq) error {
	namespace := common.WithPrefix(req.Namespace, repoPrefixByType(req.RepoType))
	_, err := c.giteaClient.MirrorSync(namespace, req.Name)
	if err != nil {
		return err
	}
	return nil
}
