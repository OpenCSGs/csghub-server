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
