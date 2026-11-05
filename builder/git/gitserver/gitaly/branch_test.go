package gitaly

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestGitalyGetRepoBranchesPreservesSlashInBranchName(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.Background()

	tester.mocks.repoStore.EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "repo").Return(&database.Repository{
		Hashed: false,
	}, nil)
	tester.mocks.refClient.EXPECT().FindAllBranches(mock.Anything, mock.Anything).Return(
		&MockGrpcStreamClient[*gitalypb.FindAllBranchesResponse]{
			data: []*gitalypb.FindAllBranchesResponse{{
				Branches: []*gitalypb.FindAllBranchesResponse_Branch{{
					Name:   []byte("feature/foo"),
					Target: &gitalypb.GitCommit{Id: "commit-id"},
				}},
			}},
		}, nil,
	)

	branches, err := tester.GetRepoBranches(ctx, gitserver.GetBranchesReq{
		RepoType:  types.ModelRepo,
		Namespace: "ns",
		Name:      "repo",
	})

	require.NoError(t, err)
	require.Len(t, branches, 1)
	require.Equal(t, "feature/foo", branches[0].Name)
}

func TestGitalyGetRepoBranchByNamePreservesSlashInBranchName(t *testing.T) {
	tester := newGitalyTester(t)
	ctx := context.Background()

	tester.mocks.repoStore.EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "repo").Return(&database.Repository{
		Hashed: false,
	}, nil)
	tester.mocks.refClient.EXPECT().FindBranch(mock.Anything, mock.Anything).Return(
		&gitalypb.FindBranchResponse{
			Branch: &gitalypb.Branch{
				Name:         []byte("feature/foo"),
				TargetCommit: &gitalypb.GitCommit{Id: "commit-id"},
			},
		}, nil,
	)

	branch, err := tester.GetRepoBranchByName(ctx, gitserver.GetBranchReq{
		RepoType:  types.ModelRepo,
		Namespace: "ns",
		Name:      "repo",
		Ref:       "feature/foo",
	})

	require.NoError(t, err)
	require.NotNil(t, branch)
	require.Equal(t, "feature/foo", branch.Name)
}
