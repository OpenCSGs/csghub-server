package handler

import (
	"context"
	"log/slog"
	"net/url"
	"strings"

	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	// maxSpeechBatchItems bounds the number of items in one batch speech
	// request to protect the moderation service and the backend model.
	maxSpeechBatchItems = 100
	// maxSpeechBatchInputChars bounds the total input text length across all
	// items in one batch speech request.
	maxSpeechBatchInputChars = 100000
)

func supportsSpeechTask(task string) bool {
	for _, candidate := range strings.Split(task, ",") {
		switch strings.TrimSpace(candidate) {
		case string(commontypes.TextToSpeech), string(commontypes.TextToAudio):
			return true
		}
	}
	return false
}

// speechBatchProxyPath derives the upstream path for batch speech requests.
// External model endpoints are configured with the single speech API path
// (e.g. https://host/v1/audio/speech), so "/batch" is appended. CSGHub
// serverless endpoints have no path and pass the incoming request path
// (/v1/audio/speech/batch) through unchanged.
func speechBatchProxyPath(ctx context.Context, endpoint string) string {
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("endpoint", endpoint))
		return ""
	}
	path := uri.Path
	if path == "" || path == "/" {
		return ""
	}
	if strings.HasSuffix(path, "/batch") {
		return path
	}
	return strings.TrimSuffix(path, "/") + "/batch"
}
