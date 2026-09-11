package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestSpeechBatchProxyPath(t *testing.T) {
	ctx := context.Background()
	require.Equal(t, "", speechBatchProxyPath(ctx, ""))
	require.Equal(t, "", speechBatchProxyPath(ctx, "https://host"))
	require.Equal(t, "", speechBatchProxyPath(ctx, "https://host/"))
	require.Equal(t, "/v1/audio/speech/batch", speechBatchProxyPath(ctx, "https://host/v1/audio/speech"))
	require.Equal(t, "/v1/audio/speech/batch", speechBatchProxyPath(ctx, "https://host/v1/audio/speech/batch"))
}

func TestSpeechRequest_JSONRoundTrip(t *testing.T) {
	body := `{"model":"m","input":"hi","voice":"vivian","speed":1.5,"task_type":"Base","ref_audio":"https://example.com/a.wav"}`
	var req types.SpeechRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))
	require.Equal(t, "m", req.Model)
	require.Equal(t, "hi", req.Input)
	require.Equal(t, "vivian", req.Voice)
	require.Equal(t, 1.5, req.Speed)

	req.Model = "backend"
	out, err := json.Marshal(req)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.Equal(t, "backend", m["model"])
	require.Equal(t, "Base", m["task_type"])
	require.Equal(t, "https://example.com/a.wav", m["ref_audio"])
	_, hasStream := m["stream"]
	require.False(t, hasStream)
}
