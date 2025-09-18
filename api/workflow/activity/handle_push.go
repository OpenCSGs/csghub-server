package activity

import (
	"context"
	"log/slog"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/types"
)

func (a *Activities) WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] watch space change start", slog.Any("req", req))
	a.callback.SetRepoVisibility(true)
	return a.callback.WatchSpaceChange(ctx, req)
}

func (a *Activities) WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] watch repo relation start", slog.Any("req", req))
	a.callback.SetRepoVisibility(true)
	return a.callback.WatchRepoRelation(ctx, req)
}

func (a *Activities) GenSyncVersion(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] generate sync version start", slog.Any("req", req))
	a.callback.SetRepoVisibility(true)
	return a.callback.GenSyncVersion(ctx, req)
}

func (a *Activities) SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] set repo update time start", slog.Any("req", req))
	a.callback.SetRepoVisibility(true)
	return a.callback.SetRepoUpdateTime(ctx, req)
}

func (a *Activities) UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] update repo infos start", slog.Any("req", req))
	a.callback.SetRepoVisibility(true)
	return a.callback.UpdateRepoInfos(ctx, req)
}

func (a *Activities) SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] sensitive check start", slog.Any("req", req))
	a.callback.SetRepoVisibility(true)
	return a.callback.SensitiveCheck(ctx, req)
}

func (a *Activities) MCPScan(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("[git_callback] mcp scan start", slog.Any("req", req))
	return a.callback.MCPScan(ctx, req)
}
