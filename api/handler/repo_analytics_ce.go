//go:build !ee && !saas

package handler

import (
	"github.com/gin-gonic/gin"
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

func (*RepoHandler) captureDownloadResult(*gin.Context, *types.GetFileReq, int64, string) {}

func captureStandaloneModelDownload(*gin.Context, analytics.Publisher, modelDownloadEvent) {}
