package gitaly

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

// updateRefErrorStream injects typed send and receive failures without losing their identity.
type updateRefErrorStream struct {
	grpc.ClientStream
	sendErr       error
	receiveErr    error
	receiveCalled bool
}

// Send returns the configured request-write error.
func (s *updateRefErrorStream) Send(*gitalypb.UpdateReferencesRequest) error {
	return s.sendErr
}

// CloseAndRecv records whether the response was read and returns the configured error.
func (s *updateRefErrorStream) CloseAndRecv() (*gitalypb.UpdateReferencesResponse, error) {
	s.receiveCalled = true
	return nil, s.receiveErr
}

// TestUpdateRefPreservesErrorChain keeps RPC codes and original errors observable across both stream phases.
func TestUpdateRefPreservesErrorChain(t *testing.T) {
	for _, cause := range []error{
		context.Canceled,
		status.Error(codes.Canceled, "request canceled"),
		status.Error(codes.Unavailable, "Gitaly unavailable"),
		status.Error(codes.PermissionDenied, "access denied"),
		errors.New("reference update failed"),
	} {
		for _, phase := range []string{"send", "recv"} {
			t.Run(phase+"/"+cause.Error(), func(t *testing.T) {
				tester := newGitalyTester(t)
				tester.timeout = time.Second
				stream := &updateRefErrorStream{}
				if phase == "send" {
					stream.sendErr = cause
				} else {
					stream.receiveErr = cause
				}
				tester.mocks.refClient.EXPECT().UpdateReferences(mock.Anything).Return(stream, nil)

				err := tester.UpdateRef(context.Background(), gitserver.UpdateRefReq{
					RepoType: types.ModelRepo, RelativePath: "models_ns/repo.git", Ref: "refs/heads/main", NewObjectId: "commit",
				})

				require.ErrorIs(t, err, cause)
				require.Equal(t, status.Code(cause), status.Code(err))
				require.ErrorContains(t, err, "UpdateReferences "+phase)
				require.Equal(t, phase == "recv", stream.receiveCalled)
			})
		}
	}
}
