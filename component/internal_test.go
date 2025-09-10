package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestInternalComponent_Allowed(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	allowed, err := ic.Allowed(ctx)
	require.Nil(t, err)
	require.True(t, allowed)
}

func TestInternalComponent_SSHAllowed(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	ic.mocks.gitServer.EXPECT().BuildRelativePath(mock.Anything, types.ModelRepo, "ns", "n").Return("models_ns/n.git", nil)
	ic.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{ID: 123, Private: true}, nil,
	)

	ic.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		ID: 321,
	}, nil)
	ic.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{ID: 123, Private: true}, nil,
	)
	ic.mocks.stores.SSHMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SSHKey{
		ID:   111,
		User: &database.User{ID: 11, Username: "user"},
	}, nil)
	ic.mocks.components.repo.EXPECT().AllowWriteAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)
	ic.mocks.components.repo.EXPECT().AllowReadAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)

	req := types.SSHAllowedReq{
		RepoType:  types.ModelRepo,
		Changes:   "abc main ref",
		Namespace: "ns",
		Name:      "n",
		KeyID:     "1",
		Action:    "git-receive-pack",
	}
	resp, err := ic.SSHAllowed(ctx, req)
	require.Nil(t, err)
	expected := &types.SSHAllowedResp{
		Success:          true,
		Message:          "allowed",
		Repo:             req.Repo,
		UserID:           "11",
		KeyType:          "ssh",
		KeyID:            111,
		ProjectID:        123,
		RootNamespaceID:  321,
		GitConfigOptions: []string{"uploadpack.allowFilter=true", "uploadpack.allowAnySHA1InWant=true"},
		Gitaly: types.Gitaly{
			Repo: pb.Repository{
				RelativePath: "models_ns/n.git",
				GlRepository: "models/ns/n",
			},
		},
		StatusCode: 200,
	}

	require.Equal(t, expected, resp)

	req.Action = "git-upload-pack"
	resp, err = ic.SSHAllowed(ctx, req)
	require.Nil(t, err)
	require.Equal(t, expected, resp)

}

func TestInternalComponent_GetAuthorizedKeys(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	ic.mocks.stores.SSHMock().EXPECT().FindByFingerpringSHA256(
		ctx, "dUQ5GwtKsCPC8Scv1OLnOEvIW0QWULVSWyj5bZwQHwM",
	).Return(&database.SSHKey{}, nil)
	key, err := ic.GetAuthorizedKeys(ctx, "foobar")
	require.Nil(t, err)
	require.Equal(t, &database.SSHKey{}, key)
}

func TestInternalComponent_GetCommitDiff(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	req := types.GetDiffBetweenTwoCommitsReq{
		Namespace:     "ns",
		Name:          "n",
		RepoType:      types.ModelRepo,
		Ref:           "main",
		LeftCommitId:  "l",
		RightCommitId: "r",
	}
	ic.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{}, nil,
	)
	ic.mocks.gitServer.EXPECT().GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     req.Namespace,
		Name:          req.Name,
		RepoType:      req.RepoType,
		Ref:           req.Ref,
		LeftCommitId:  req.LeftCommitId,
		RightCommitId: req.RightCommitId,
	}).Return(&types.GiteaCallbackPushReq{Ref: "main"}, nil)

	resp, err := ic.GetCommitDiff(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.GiteaCallbackPushReq{Ref: "main"}, resp)
}

func TestInternalComponent_LfsAuthenticate(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	ic.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{Private: true}, nil,
	)
	ic.mocks.stores.SSHMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SSHKey{
		ID:   111,
		User: &database.User{ID: 11, Username: "user"},
	}, nil)
	ic.mocks.components.repo.EXPECT().AllowReadAccess(
		ctx, types.ModelRepo, "ns", "n", "user",
	).Return(true, nil)
	ic.mocks.stores.AccessTokenMock().EXPECT().GetUserGitToken(ctx, "user").Return(
		&database.AccessToken{Token: "token"}, nil,
	)

	resp, err := ic.LfsAuthenticate(ctx, types.LfsAuthenticateReq{
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.ModelRepo,
		KeyID:     "1",
	})
	require.Nil(t, err)
	require.Equal(t, &types.LfsAuthenticateResp{
		Username: "user",
		LfsToken: "token",
		RepoPath: "/models/ns/n.git",
	}, resp)

}

func TestInternalComponent_TriggerDataviewerWorkflow(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	req := types.UpdateViewerReq{
		Namespace: "ns",
		Name:      "n",
		Branch:    "main",
		RepoType:  types.DatasetRepo,
	}

	result := &types.WorkFlowInfo{
		Namespace:  req.Namespace,
		Name:       req.Name,
		Branch:     req.Branch,
		RepoType:   req.RepoType,
		WorkFlowID: "xxxxx",
	}

	ic.mocks.dataviewerClient.EXPECT().TriggerWorkflow(mock.Anything, req).Return(result, nil)

	res, err := ic.TriggerDataviewerWorkflow(ctx, req)
	require.Nil(t, err)
	require.Equal(t, result, res)
}

func TestInternalComponent_CheckGitCallback(t *testing.T) {
	ctx := context.TODO()
	ic := initializeTestInternalComponent(ctx, t)

	ic.mocks.checker.EXPECT().Check(ctx, mock.Anything).Return(true, nil)

	valid, err := ic.CheckGitCallback(ctx, types.GitalyAllowedReq{})
	require.Nil(t, err)
	require.Equal(t, true, valid)
}
