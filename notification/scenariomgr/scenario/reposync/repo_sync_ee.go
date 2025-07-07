//go:build ee || saas

package reposync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/dustin/go-humanize"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

var allowedRepoSyncStatuses = map[string]bool{
	"repo_sync_start": true,
	"lfs_sync_start":  true,
	"sync_finished":   true,
	"sync_failed":     true,
	"repo_too_large":  true,
}

var statusTemplates = map[string]string{
	"repo_sync_start": "▶️ %s 同步开始 repo",
	"lfs_sync_start":  "▶️ %s 同步开始 lfs%s",
	"sync_failed":     "❌ %s 同步失败，查看原因 %s",
	"sync_finished":   "✅ %s 同步完成%s",
	"repo_too_large":  "⚠️ %s 仓库太大，无法同步",
}

type RepoSyncNotification struct {
	Title     string
	RemoteURL string
	LocalURL  string
	SyncTime  string
}

// implement scenariomgr.GetDataFunc to get repo sync notification data
func GetRepoSyncNotification(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	var rsNotify RepoSyncNotification

	var info types.SyncInfo
	if err := json.Unmarshal([]byte(msg.Parameters), &info); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if !allowedRepoSyncStatuses[info.Status] {
		return nil, fmt.Errorf("invalid status: %s, allowed statuses: %v", info.Status, allowedRepoSyncStatuses)
	}

	rsNotify.Title = getRepoSyncMessageTitle(info)
	rsNotify.RemoteURL = info.RemoteURL
	rsNotify.LocalURL = info.LocalURL

	location, _ := time.LoadLocation(conf.Notification.RepoSyncTimezone)
	createdAt := msg.CreatedAt.In(location)
	rsNotify.SyncTime = fmt.Sprintf("🕒 %s", createdAt.Format("2006-01-02 15:04:05"))

	return &scenariomgr.NotificationData{
		MessageData: rsNotify,
		Receiver:    &notifychannel.Receiver{IsBroadcast: true},
	}, nil
}

func getRepoSyncMessageTitle(info types.SyncInfo) string {
	switch info.Status {
	case "repo_sync_start", "repo_too_large":
		return fmt.Sprintf(statusTemplates[info.Status], info.Path)
	case "lfs_sync_start", "sync_finished":
		sizeStr := getSizeStr(info.Size)
		return fmt.Sprintf(statusTemplates[info.Status], info.Path, sizeStr)
	case "sync_failed":
		taskURL, err := getTaskURL(info.LocalURL)
		if err != nil {
			slog.Error("failed to get task URL", "error", err, "localURL", info.LocalURL)
			taskURL = info.LocalURL
		}
		return fmt.Sprintf(statusTemplates[info.Status], info.Path, taskURL)
	default:
		return fmt.Sprintf("%s %s", info.Path, info.Status)
	}
}

func getSizeStr(size int64) string {
	if size <= 0 {
		return ""
	}
	return fmt.Sprintf("，大小 %s", humanize.IBytes(uint64(size)))
}

const adminPanelMirrorsPath = "/admin_panel/mirrors"

func getTaskURL(localURL string) (string, error) {
	if localURL == "" {
		return "", fmt.Errorf("localURL cannot be empty")
	}

	u, err := url.Parse(localURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.Scheme == "" {
		return "", fmt.Errorf("URL must have a scheme")
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL must have a host")
	}

	return fmt.Sprintf("%s://%s%s%s", u.Scheme, u.Host, adminPanelMirrorsPath, u.Path), nil
}
