//go:build ee || saas

package handler

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/analytics"
	"opencsg.com/csghub-server/common/types"
)

const (
	modelFileDownloadURLIssued = "model_file_download_url_issued"
	modelFileDownloadSuccess   = "model_file_download_success"
	modelGitClonePackServed    = "model_git_clone_pack_served"

	downloadChannelBrowser       = "browser"
	downloadChannelSDKCompatible = "sdk_compatible"
	downloadChannelHTTPSGit      = "https_git"

	deliveryTypeDirectStream = "direct_stream"
	deliveryTypeLFSURL       = "lfs_url"
	deliveryTypeGitPack      = "git_pack"
)

type modelDownloadEvent struct {
	Name          string
	Namespace     string
	RepoName      string
	FilePath      string
	FileSize      int64
	LFSOID        string
	Ref           string
	Channel       string
	DeliveryType  string
	CorrelationID string
	DistinctID    string
	SessionID     string
	Standalone    bool
}

func (h *RepoHandler) captureDownloadResult(
	ctx *gin.Context,
	req *types.GetFileReq,
	size int64,
	eventName string,
) {
	if h.analytics == nil || req.RepoType != types.ModelRepo {
		return
	}

	analyticsContext := analyticsContextFromRequest(ctx)
	if analyticsContext.CorrelationID == "" || analyticsContext.DistinctID == "" {
		return
	}

	deliveryType := deliveryTypeDirectStream
	if req.Lfs {
		deliveryType = deliveryTypeLFSURL
	}
	captureModelDownloadEvent(ctx, h.analytics, modelDownloadEvent{
		Name:          eventName,
		Namespace:     req.Namespace,
		RepoName:      req.Name,
		FilePath:      req.Path,
		FileSize:      size,
		Ref:           req.Ref,
		Channel:       downloadChannelBrowser,
		DeliveryType:  deliveryType,
		CorrelationID: analyticsContext.CorrelationID,
		DistinctID:    analyticsContext.DistinctID,
		SessionID:     analyticsContext.SessionID,
	})
}

func captureStandaloneModelDownload(
	ctx *gin.Context,
	publisher analytics.Publisher,
	event modelDownloadEvent,
) {
	event.Standalone = true
	captureModelDownloadEvent(ctx, publisher, event)
}

func captureModelDownloadEvent(
	ctx *gin.Context,
	publisher analytics.Publisher,
	event modelDownloadEvent,
) {
	if publisher == nil || event.Name == "" {
		return
	}

	properties := map[string]any{
		"repo_type":        string(types.ModelRepo),
		"namespace":        event.Namespace,
		"repo_name":        event.RepoName,
		"download_channel": event.Channel,
		"delivery_type":    event.DeliveryType,
	}
	if event.FilePath != "" {
		properties["file_path"] = event.FilePath
		properties["file_size"] = event.FileSize
		if extension := strings.TrimPrefix(filepath.Ext(event.FilePath), "."); extension != "" {
			properties["file_format"] = strings.ToLower(extension)
		}
	}
	if event.FilePath == "" && event.FileSize > 0 {
		properties["file_size"] = event.FileSize
	}
	if event.Ref != "" {
		properties["ref"] = event.Ref
	}
	if event.LFSOID != "" {
		properties["lfs_oid"] = event.LFSOID
	}

	insertID := ""
	if event.Standalone {
		requestID := uuid.NewString()
		insertID = event.Name + ":" + requestID
		event.DistinctID = strings.TrimSpace(httpbase.GetCurrentUserUUID(ctx))
		if event.DistinctID != "" {
			properties["identity_type"] = "authenticated"
		} else {
			event.DistinctID = "anonymous-request:" + requestID
			properties["identity_type"] = "anonymous_request"
			properties["$process_person_profile"] = false
		}
	}
	if strings.TrimSpace(event.DistinctID) == "" {
		return
	}

	err := publisher.Capture(analytics.Event{
		Name:          event.Name,
		DistinctID:    event.DistinctID,
		SessionID:     event.SessionID,
		CorrelationID: event.CorrelationID,
		InsertID:      insertID,
		Properties:    properties,
	})
	if err != nil {
		slog.WarnContext(
			ctx.Request.Context(),
			"Failed to enqueue PostHog download event",
			slog.String("event", event.Name),
			slog.String("correlation_id", event.CorrelationID),
			slog.Any("error", err),
		)
	}
}
