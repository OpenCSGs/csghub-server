package callback

import (
	"context"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type SyncVersionGenerator struct {
	s *database.MultiSyncStore
}

func NewSyncVersionGenerator() *SyncVersionGenerator {
	return &SyncVersionGenerator{
		s: database.NewMultiSyncStore(),
	}
}

func (g *SyncVersionGenerator) GenSyncVersion(req *types.GiteaCallbackPushReq) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	_, err := g.s.Create(ctx, database.SyncVersion{
		SourceID:       types.SyncVersionSourceOpenCSG,
		RepoPath:       fmt.Sprintf("%s/%s", namespace, repoName),
		RepoType:       types.RepositoryType(strings.TrimRight(repoType, "s")),
		LastModifiedAt: req.HeadCommit.LastModifyTime,
		ChangeLog:      req.HeadCommit.Message,
	})

	return err
}
