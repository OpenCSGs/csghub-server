package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMirrorCredentialRequestJSONFields verifies mirror creation requests accept explicit credentials.
func TestMirrorCredentialRequestJSONFields(t *testing.T) {
	payload := []byte(`{"username":"git-user","access_token":"git-token"}`)

	var repoMirrorReq CreateMirrorReq
	require.NoError(t, json.Unmarshal(payload, &repoMirrorReq))
	require.Equal(t, "git-user", repoMirrorReq.Username)
	require.Equal(t, "git-token", repoMirrorReq.AccessToken)

	var createReq CreateMirrorRepoReq
	require.NoError(t, json.Unmarshal(payload, &createReq))
	require.Equal(t, "git-user", createReq.Username)
	require.Equal(t, "git-token", createReq.AccessToken)

	var batchReq MirrorReq
	require.NoError(t, json.Unmarshal(payload, &batchReq))
	require.Equal(t, "git-user", batchReq.Username)
	require.Equal(t, "git-token", batchReq.AccessToken)

	var legacyReq CreateMirrorReq
	require.NoError(t, json.Unmarshal([]byte(`{"password":"legacy-token"}`), &legacyReq))
	require.Empty(t, legacyReq.AccessToken)
}

// TestCreateMirrorRepoRequestPriorityJSONField verifies callers can provide scheduling priority.
func TestCreateMirrorRepoRequestPriorityJSONField(t *testing.T) {
	var req CreateMirrorRepoReq
	require.NoError(t, json.Unmarshal([]byte(`{"priority":2}`), &req))
	require.Equal(t, HighMirrorPriority, req.Priority)
}

// TestMirrorMetricsJSONPreservesUnknownAndLargeValues guards the browser-facing contract.
func TestMirrorMetricsJSONPreservesUnknownAndLargeValues(t *testing.T) {
	large := "9007199254740993"
	zero := float64(0)
	task := MirrorMetricsTask{
		TaskID:        large,
		DownloadBPS:   &zero,
		UploadBPS:     &zero,
		UploadedBytes: &large,
		LFS:           &MirrorMetricsLFS{CompleteBytes: &large},
	}
	encoded, err := json.Marshal(task)
	require.NoError(t, err)
	var values map[string]any
	require.NoError(t, json.Unmarshal(encoded, &values))
	require.Equal(t, large, values["task_id"])
	require.Equal(t, zero, values["download_bps"])
	require.Equal(t, zero, values["upload_bps"])
	require.Equal(t, large, values["uploaded_bytes"])
	require.Contains(t, values, "instance_id")
	require.Nil(t, values["instance_id"])
	require.Nil(t, values["sampled_at_ms"])
	lfs := values["lfs"].(map[string]any)
	require.Equal(t, large, lfs["complete_bytes"])
	require.Contains(t, lfs, "total_bytes")
	require.Nil(t, lfs["total_bytes"])
	require.Nil(t, lfs["progress_percent"])
}

// TestMirrorMetricsQueueJSON exposes only the two core waiting counts.
func TestMirrorMetricsQueueJSON(t *testing.T) {
	encoded, err := json.Marshal(MirrorMetricsQueues{Repo: MirrorMetricsQueue{Waiting: 3}})
	require.NoError(t, err)
	require.JSONEq(t, `{"repo":{"waiting":3},"lfs":{"waiting":0}}`, string(encoded))
}
