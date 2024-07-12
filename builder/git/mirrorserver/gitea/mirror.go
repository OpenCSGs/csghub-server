package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
)

const (
	ModelOrgPrefix        = "models_"
	DatasetOrgPrefix      = "datasets_"
	SpaceOrgPrefix        = "spaces_"
	CodeOrgPrefix         = "codes_"
	MirrorServerNamespace = "root"
)

func (c *MirrorClient) CreateMirrorRepo(ctx context.Context, req mirrorserver.CreateMirrorRepoReq) (int64, error) {
	task, _, err := c.giteaClient.MigrateRepo(gitea.MigrateRepoOption{
		RepoName:     req.Name,
		RepoOwner:    req.Namespace,
		CloneAddr:    req.CloneUrl,
		Service:      gitea.GitServicePlain,
		AuthUsername: req.Username,
		AuthPassword: req.AccessToken,
		Mirror:       true,
		Private:      req.Private,
		Description:  req.Description,
		Wiki:         false,
		Milestones:   false,
		Labels:       false,
		Issues:       false,
		PullRequests: false,
		Releases:     false,
		LFS:          req.SyncLfs,
	})
	if err != nil {
		return 0, err
	}
	return task.ID, nil
}

func (c *MirrorClient) GetMirrorTaskInfo(ctx context.Context, taskId int64) (*mirrorserver.MirrorTaskInfo, error) {
	ts, _, err := c.giteaClient.GetUserTaskInfo(taskId)
	if err != nil {
		return nil, err
	}

	mti := &mirrorserver.MirrorTaskInfo{
		Status:    mirrorserver.TaskStatus(ts.Status),
		Message:   ts.Message,
		RepoID:    ts.RepoID,
		RepoName:  ts.RepoName,
		StartedAt: ts.StartedAt,
		EndedAt:   ts.EndedAt,
	}
	return mti, nil
}

func (c *MirrorClient) CreatePushMirror(ctx context.Context, req mirrorserver.CreatePushMirrorReq) error {
	_, err := c.giteaClient.CreatePushMirror(MirrorServerNamespace, req.Name, gitea.CreatePushMirrorOption{
		RemoteAddress:  req.PushUrl,
		RemoteUsername: req.Username,
		RemotePassword: req.AccessToken,
		Interval:       "8h",
		SyncOnCommit:   true,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *MirrorClient) MirrorSync(ctx context.Context, req mirrorserver.MirrorSyncReq) error {
	_, err := c.giteaClient.MirrorSync(req.Namespace, req.Name)
	if err != nil {
		return err
	}
	return nil
}
