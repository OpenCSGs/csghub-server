package component

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// Existing mirror tests use deterministic public DNS while retaining the real
// URL and address policy. Security tests override just the DNS answers.
func newPublicMirrorTestComponent(ctx context.Context, t *testing.T) *testMirrorWithMocks {
	m := initializeTestMirrorComponent(ctx, t)
	m.sourcePolicy.LookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	return m
}

func TestMirrorCreationRejectsInternalDestination(t *testing.T) {
	for _, source := range []string{"https://127.0.0.1/ns/repo.git", "https://private.example/ns/repo.git"} {
		t.Run(source, func(t *testing.T) {
			// No stores or Git client: rejection must precede any side effects.
			m := &mirrorComponentImpl{sourcePolicy: common.PublicHTTPPolicy{
				LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
					return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
				},
			}}
			_, err := m.CreateMirror(context.Background(), types.CreateMirrorReq{SourceUrl: source})
			require.ErrorIs(t, err, errorx.ErrMirrorSourceURLInvalid)
			_, err = m.UpdateMirror(context.Background(), types.UpdateMirrorReq{SourceUrl: source})
			require.ErrorIs(t, err, errorx.ErrMirrorSourceURLInvalid)
			_, err = m.CreateMirrorRepo(context.Background(), types.CreateMirrorRepoReq{
				CurrentUser: "user", SourceNamespace: "ns", SourceName: "repo",
				ForkNamespace: "ns", ForkName: "repo", SourceGitCloneUrl: source,
			})
			require.ErrorIs(t, err, errorx.ErrMirrorSourceURLInvalid)
		})
	}
}

func TestMirrorDestinationTrustedDomainSkipsDNS(t *testing.T) {
	m := &mirrorComponentImpl{sourcePolicy: common.PublicHTTPPolicy{
		TrustedDomains: []string{"huggingface.com"},
		LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
		},
	}}
	require.NoError(t, m.validateMirrorDestination(context.Background(), "https://huggingface.com/ns/repo.git"))
	require.Error(t, m.validateMirrorDestination(context.Background(), "https://other.example/ns/repo.git"))
}
