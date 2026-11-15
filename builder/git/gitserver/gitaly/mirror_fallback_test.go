package gitaly

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorRPCFallbackUsesOneValidatedDNSSnapshot(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "clone", true: "fetch"}[existing], func(t *testing.T) {
			c := newGitalyTester(t)
			c.timeout = time.Second
			c.config.RepoTemplate.EmptyRepoType = string(types.ModelRepo)
			lookups := 0
			c.sourcePolicy.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
				lookups++
				require.Equal(t, 1, lookups)
				return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}, nil
			}
			req := gitserver.CreateMirrorRepoReq{CloneUrl: "https://example.com/ns/repo.git", Namespace: "ns", Name: "repo", RepoType: types.ModelRepo}
			if existing {
				c.mocks.repoClient.EXPECT().FetchRemote(mock.Anything, mock.MatchedBy(func(r *gitalypb.FetchRemoteRequest) bool { return r.RemoteParams.ResolvedAddress == "8.8.8.8" })).Return(nil, status.Error(codes.Internal, `fetch remote: "fatal: unable to access 'https://example.com/ns/repo.git/': Failed to connect to example.com port 443: Couldn't connect to server\n": exit status 128`)).Once()
				c.mocks.repoClient.EXPECT().FetchRemote(mock.Anything, mock.MatchedBy(func(r *gitalypb.FetchRemoteRequest) bool {
					return r.RemoteParams.ResolvedAddress == "1.1.1.1" && r.RemoteParams.Url == req.CloneUrl
				})).Return(&gitalypb.FetchRemoteResponse{}, nil).Once()
				require.NoError(t, c.CreateMirrorForExistsRepo(context.Background(), req))
			} else {
				c.mocks.repoClient.EXPECT().CreateRepositoryFromURL(mock.Anything, mock.MatchedBy(func(r *gitalypb.CreateRepositoryFromURLRequest) bool { return r.ResolvedAddress == "8.8.8.8" })).Return(nil, status.Error(codes.Internal, "creating repository: cloning repository: exit status 128")).Once()
				c.mocks.repoClient.EXPECT().CreateRepositoryFromURL(mock.Anything, mock.MatchedBy(func(r *gitalypb.CreateRepositoryFromURLRequest) bool {
					return r.ResolvedAddress == "1.1.1.1" && r.Url == req.CloneUrl
				})).Return(&gitalypb.CreateRepositoryFromURLResponse{}, nil).Once()
				_, err := c.CreateMirrorRepo(context.Background(), req)
				require.NoError(t, err)
			}
		})
	}
}

func TestMirrorAddressFallbackDoesNotRetryUncertainOrCommittedFailures(t *testing.T) {
	for _, cause := range []error{
		status.Error(codes.Unavailable, "connection lost"),
		status.Error(codes.DeadlineExceeded, "timeout"),
		status.Error(codes.Canceled, "canceled"),
		status.Error(codes.Internal, "committing reference update: failed"),
		status.Error(codes.Internal, `fetch remote: "From example.com\n ! [rejected] main -> main\n": exit status 1`),
		status.Error(codes.Internal, "fetch remote: exit status 128"),
		status.Error(codes.Internal, `fetch remote: "fatal: unable to access 'https://example.com/repo.git/': Recv failure: Connection reset by peer\n": exit status 128`),
		status.Error(codes.AlreadyExists, "repository exists already"),
	} {
		t.Run(cause.Error(), func(t *testing.T) {
			attempts := 0
			err := retryMirrorAddresses(context.Background(), []string{"8.8.8.8", "1.1.1.1"}, func(string) error { attempts++; return cause })
			require.Equal(t, cause, err)
			require.Equal(t, 1, attempts)
		})
	}
}
