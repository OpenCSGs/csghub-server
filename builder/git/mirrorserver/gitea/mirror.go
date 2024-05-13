package gitea

import (
	"context"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
)

func (c *MirrorClient) CreateMirrorRepo(ctx context.Context, req mirrorserver.CreateMirrorRepoReq) error {
	c.giteaClient.MigrateRepo(gitea.MigrateRepoOption{
		RepoName:       req.Name,
		RepoOwner:      req.Namespace,
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
		MirrorInterval: req.Interval,
		LFS:            true,
	})
	return nil
}
