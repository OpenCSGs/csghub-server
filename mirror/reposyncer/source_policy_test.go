package reposyncer

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/utils/common"
)

func TestSyncRepoRevalidatesPersistedSourceBeforeSideEffects(t *testing.T) {
	for _, source := range []string{"http://169.254.169.254/ns/repo.git", "https://rebound.example/ns/repo.git"} {
		t.Run(source, func(t *testing.T) {
			w := &RepoSyncWorker{sourcePolicy: common.PublicHTTPPolicy{
				LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
					return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
				},
			}}
			mirror := &database.Mirror{SourceUrl: source, Repository: &database.Repository{}}
			// A resumed task must not skip validation either.
			task := &database.MirrorTask{BeforeLastCommitID: "before", AfterLastCommitID: "after"}
			result, err := w.SyncRepo(context.Background(), mirror, task)
			require.ErrorContains(t, err, "invalid mirror source")
			require.Same(t, task, result.Task)
		})
	}
}
