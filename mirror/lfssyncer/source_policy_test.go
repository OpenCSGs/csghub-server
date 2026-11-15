package lfssyncer

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func TestLFSSourceClientRejectsInternalBatchAndDownload(t *testing.T) {
	for _, source := range []string{"http://169.254.169.254", "http://2130706433", "https://rebound.example"} {
		t.Run(source, func(t *testing.T) {
			policy := common.PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
			}}
			client := newLFSSourceHTTPClient(policy)
			defer client.CloseIdleConnections()
			w := &LfsSyncWorker{httpClient: client}
			_, err := w.GetLFSDownloadURLs(context.Background(), source+"/ns/repo.git", "main", "", "", []*types.Pointer{{Oid: "oid", Size: 10}})
			require.ErrorContains(t, err, "outbound")
			resp, err := w.downloadRange(context.Background(), source+"/object", nil, 0, 9)
			if resp != nil {
				resp.Body.Close()
			}
			require.ErrorContains(t, err, "outbound")
		})
	}
}

func TestLFSWorkerRejectsUnprotectedProxy(t *testing.T) {
	cfg := createTestConfig()
	cfg.Proxy.Enable = true
	cfg.Proxy.URL = "http://proxy.example:8080"
	w, err := NewLfsSyncWorker(cfg)
	require.Nil(t, w)
	require.ErrorContains(t, err, "LFS source proxy")
}
