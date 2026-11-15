package reposyncer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockworkflow "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestPublishCommitPreservesDefaultBranch(t *testing.T) {
	git := mockgit.NewMockGitServer(t)
	workflow := mockworkflow.NewMockClient(t)
	repo := &database.Repository{DefaultBranch: "master", RepositoryType: types.ModelRepo, Private: true}
	callback := &types.GiteaCallbackPushReq{Repository: types.GiteaCallbackPushReq_Repository{DefaultBranch: "master"}}
	git.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
		return req.Ref == "refs/heads/master" && req.RelativePath == "models_ns/repo.git"
	})).Return(nil).Once()
	git.EXPECT().GetDiffBetweenTwoCommits(mock.Anything, mock.MatchedBy(func(req gitserver.GetDiffBetweenTwoCommitsReq) bool {
		return req.DefaultBranch == "master" && req.Ref == "master" && req.Private && req.RightCommitId == "new" && req.RelativePath == "models_ns/repo.git"
	})).Return(callback, nil).Once()
	workflow.EXPECT().ExecuteWorkflow(mock.Anything, mock.Anything, mock.Anything, mock.MatchedBy(func(req *types.GiteaCallbackPushReq) bool {
		return req.Ref == "master" && req.Repository.DefaultBranch == "master"
	})).Return(nil, nil).Once()
	require.NoError(t, PublishCommitAndTriggerCallback(context.Background(), git, workflow, repo, "ns", "repo", "models_ns/repo.git", "new"))
}
