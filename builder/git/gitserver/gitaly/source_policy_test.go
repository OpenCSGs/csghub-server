package gitaly

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func TestMirrorRPCsRejectUnsafeSourcesBeforeRPC(t *testing.T) {
	for _, source := range []string{"http://127.0.0.1/ns/repo.git", "https://private.example/ns/repo.git", "http://2130706433/ns/repo.git"} {
		t.Run(source, func(t *testing.T) {
			c := &Client{timeout: time.Second, gitFetchTimeout: time.Second,
				sourcePolicy: common.PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
					return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
				}},
			}
			// Nil RPC dependencies prove rejection precedes remote access. A sync
			// token must not grant permission to bypass the destination policy.
			req := gitserver.CreateMirrorRepoReq{CloneUrl: source, MirrorToken: "trusted-token"}
			_, err := c.CreateMirrorRepo(context.Background(), req)
			require.Error(t, err)
			require.Error(t, c.CreateMirrorForExistsRepo(context.Background(), req))
			require.Error(t, c.MirrorSync(context.Background(), gitserver.MirrorSyncReq{CloneUrl: source, MirrorToken: "trusted-token"}))
		})
	}
}

func TestMirrorCreationPinsDNSWithoutUnprotectedProbe(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "clone", true: "fetch existing"}[existing], func(t *testing.T) {
			c := newGitalyTester(t)
			c.timeout = time.Second
			c.config.RepoTemplate.EmptyRepoType = string(types.ModelRepo)
			lookups := 0
			c.sourcePolicy.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
				lookups++
				if lookups > 1 {
					return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
				}
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}
			req := gitserver.CreateMirrorRepoReq{CloneUrl: "https://example.com/ns/repo.git", Namespace: "ns", Name: "repo", RepoType: types.ModelRepo}
			if existing {
				c.mocks.repoClient.EXPECT().FetchRemote(mock.Anything, mock.MatchedBy(func(r *gitalypb.FetchRemoteRequest) bool {
					return r.RemoteParams.Url == req.CloneUrl && r.RemoteParams.ResolvedAddress == "8.8.8.8"
				})).Return(&gitalypb.FetchRemoteResponse{}, nil).Once()
				require.NoError(t, c.CreateMirrorForExistsRepo(context.Background(), req))
				require.Error(t, c.CreateMirrorForExistsRepo(context.Background(), req))
			} else {
				c.mocks.repoClient.EXPECT().CreateRepositoryFromURL(mock.Anything, mock.MatchedBy(func(r *gitalypb.CreateRepositoryFromURLRequest) bool {
					return r.Url == req.CloneUrl && r.ResolvedAddress == "8.8.8.8" && r.Mirror
				})).Return(&gitalypb.CreateRepositoryFromURLResponse{}, nil).Once()
				_, err := c.CreateMirrorRepo(context.Background(), req)
				require.NoError(t, err)
				_, err = c.CreateMirrorRepo(context.Background(), req)
				require.Error(t, err)
			}
			require.Equal(t, 2, lookups)
		})
	}
}

func TestMirrorSyncPinsHostnameAndRevalidatesNextAttempt(t *testing.T) {
	c := newGitalyTester(t)
	c.gitFetchTimeout = time.Second
	lookups := 0
	c.sourcePolicy.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		if lookups > 1 {
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	remoteErr := errors.New("remote unavailable")
	c.mocks.repoClient.EXPECT().FetchRemote(mock.Anything, mock.MatchedBy(func(r *gitalypb.FetchRemoteRequest) bool {
		return r.RemoteParams.Url == "https://example.com/ns/repo.git" && r.RemoteParams.ResolvedAddress == "8.8.8.8"
	})).Return(nil, remoteErr).Once()
	req := gitserver.MirrorSyncReq{CloneUrl: "https://example.com/ns/repo.git", RelativePath: "models_ns/repo.git"}
	require.ErrorContains(t, c.MirrorSync(context.Background(), req), remoteErr.Error())
	require.ErrorContains(t, c.MirrorSync(context.Background(), req), "non-public")
}

// Trusted domains must be resolved by Gitaly itself, not by the API host.
func TestMirrorTrustedDomainUsesGitalyDNS(t *testing.T) {
	c := newGitalyTester(t)
	c.timeout = time.Second
	c.gitFetchTimeout = time.Second
	c.config.RepoTemplate.EmptyRepoType = string(types.ModelRepo)
	c.sourcePolicy.TrustedDomains = []string{"huggingface.com"}
	c.sourcePolicy.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("trusted source must not be resolved by the caller")
		return nil, nil
	}
	req := gitserver.CreateMirrorRepoReq{CloneUrl: "https://huggingface.com/ns/repo.git", Namespace: "ns", Name: "repo", RepoType: types.ModelRepo}
	c.mocks.repoClient.EXPECT().CreateRepositoryFromURL(mock.Anything, mock.MatchedBy(func(r *gitalypb.CreateRepositoryFromURLRequest) bool {
		return r.Url == req.CloneUrl && r.ResolvedAddress == ""
	})).Return(&gitalypb.CreateRepositoryFromURLResponse{}, nil).Once()
	_, err := c.CreateMirrorRepo(context.Background(), req)
	require.NoError(t, err)
	c.mocks.repoClient.EXPECT().FetchRemote(mock.Anything, mock.MatchedBy(func(r *gitalypb.FetchRemoteRequest) bool {
		return r.RemoteParams.Url == req.CloneUrl && r.RemoteParams.ResolvedAddress == ""
	})).Return(&gitalypb.FetchRemoteResponse{}, nil).Twice()
	require.NoError(t, c.CreateMirrorForExistsRepo(context.Background(), req))
	refErr := errors.New("stop after successful trusted fetch")
	c.mocks.refClient.EXPECT().ListRefs(mock.Anything, mock.Anything).Return(nil, refErr).Once()
	require.ErrorContains(t, c.MirrorSync(context.Background(), gitserver.MirrorSyncReq{CloneUrl: req.CloneUrl, RelativePath: "models_ns/repo.git"}), refErr.Error())
}
