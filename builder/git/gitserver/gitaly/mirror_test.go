package gitaly

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

// TestMirrorSyncAuthorization verifies remote fetch authentication and token precedence.
func TestMirrorSyncAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		request    gitserver.MirrorSyncReq
		wantHeader string
	}{
		{
			name: "basic auth",
			request: gitserver.MirrorSyncReq{
				Username:    "user",
				AccessToken: "secret",
			},
			wantHeader: "Basic dXNlcjpzZWNyZXQ=",
		},
		{
			name: "mirror token takes precedence",
			request: gitserver.MirrorSyncReq{
				Username:    "user",
				AccessToken: "secret",
				MirrorToken: "sync-token",
			},
			wantHeader: "X-OPENCSG-Sync-Tokensync-token",
		},
		{
			name:       "public repository",
			wantHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := newGitalyTester(t)
			tester.gitFetchTimeout = time.Second
			tt.request.Namespace = "ns"
			tt.request.Name = "repo"
			tt.request.RepoType = types.ModelRepo
			tt.request.CloneUrl = "https://example.com/ns/repo.git"
			tt.request.RelativePath = "models_ns/repo.git"

			tester.mocks.repoClient.EXPECT().FetchRemote(
				mock.Anything,
				mock.MatchedBy(func(req *gitalypb.FetchRemoteRequest) bool {
					return req.Repository.RelativePath == tt.request.RelativePath &&
						req.RemoteParams.HttpAuthorizationHeader == tt.wantHeader
				}),
			).Return(&gitalypb.FetchRemoteResponse{}, nil)
			tester.mocks.refClient.EXPECT().ListRefs(mock.Anything, mock.Anything).
				Return(&MockGrpcStreamClient[*gitalypb.ListRefsResponse]{}, nil)
			updateStream := &MockGrpcStreamClientFull[
				*gitalypb.UpdateReferencesRequest,
				*gitalypb.UpdateReferencesResponse,
			]{data: []*gitalypb.UpdateReferencesResponse{{}}}
			tester.mocks.refClient.EXPECT().UpdateReferences(mock.Anything).Return(updateStream, nil)

			err := tester.MirrorSync(context.Background(), tt.request)
			require.NoError(t, err)
		})
	}
}
