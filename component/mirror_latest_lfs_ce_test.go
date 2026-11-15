//go:build !ee && !saas

package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorLatestLFSReadinessOmittedInCE(t *testing.T) {
	c := &mirrorComponentImpl{latestLFSReady: func(context.Context, int64, int64) (bool, error) {
		t.Fatal("CE must not query readiness")
		return false, nil
	}}
	response := &types.Mirror{}
	require.NoError(t, c.populateLatestLFSReady(context.Background(), 10, &database.Mirror{CurrentTaskID: 20}, response))
	data, err := json.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(data), "latest_lfs_ready")
}
