//go:build gitaly_integration

package gitaly

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/common/utils/common"
)

// Loopback pins are intentional isolated fixtures passed to real Gitaly RPCs,
// not exceptions to PublicHTTPPolicy. No external origin or DNS is contacted.
func TestGitalyDirectEgressIntegration(t *testing.T) {
	fixture := buildMirrorEgressFixture(t)
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{false: "proxy negative control", true: "enforced direct egress"}[direct], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			server := startMirrorEgressServer(t, ctx, fixture, direct)
			repo := &gitalypb.Repository{StorageName: "default", RelativePath: "mirror.git"}
			const source = "http://mirror.invalid/repo.git"
			clone := func(address string) error {
				_, err := server.client.CreateRepositoryFromURL(ctx, &gitalypb.CreateRepositoryFromURLRequest{Repository: repo, Url: source, ResolvedAddress: address, Mirror: true})
				return err
			}
			if direct {
				// First address has no listener; fallback must retain the DNS snapshot.
				require.NoError(t, retryMirrorAddresses(ctx, []string{"127.0.0.2", "127.0.0.1"}, clone))
				// The caller cannot resolve trusted.invalid; only the container
				// hosts file maps it. A trusted source must leave resolution there.
				const trustedSource = "http://trusted.invalid/repo.git"
				policy := common.PublicHTTPPolicy{TrustedDomains: []string{"trusted.invalid"}}
				addresses, err := policy.ResolveURLAddresses(ctx, trustedSource)
				require.NoError(t, err)
				require.Equal(t, []string{""}, addresses)
				trustedRepo := &gitalypb.Repository{StorageName: "default", RelativePath: "trusted.git"}
				_, err = server.client.CreateRepositoryFromURL(ctx, &gitalypb.CreateRepositoryFromURLRequest{Repository: trustedRepo, Url: trustedSource, ResolvedAddress: addresses[0], Mirror: true})
				require.NoError(t, err)
				c := &Client{repoClient: server.client}
				require.NoError(t, c.fetchMirrorRemote(ctx, &gitalypb.FetchRemoteRequest{Repository: repo, RemoteParams: &gitalypb.Remote{Url: source}}, []string{"127.0.0.2", "127.0.0.1"}))
				require.NoError(t, c.fetchMirrorRemote(ctx, &gitalypb.FetchRemoteRequest{Repository: trustedRepo, RemoteParams: &gitalypb.Remote{Url: trustedSource}}, addresses))
			} else {
				require.Error(t, clone("127.0.0.1"))
			}
			counts := server.counts(t)
			if direct {
				require.Zero(t, counts["proxy"])
				require.Positive(t, counts["origin"])
			} else {
				require.Positive(t, counts["proxy"])
				require.Zero(t, counts["origin"])
			}
		})
	}
}
