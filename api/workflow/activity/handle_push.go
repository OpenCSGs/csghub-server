package activity

import (
	"context"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/types"
)

func (a *Activities) WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("watch space change start", "req", req)
	a.callback.SetRepoVisibility(true)
	return a.callback.WatchSpaceChange(ctx, req)
}

func (a *Activities) WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("watch repo relation start", "req", req)
	a.callback.SetRepoVisibility(true)
	return a.callback.WatchRepoRelation(ctx, req)
}

func (a *Activities) SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("set repo update time start", "req", req)
	a.callback.SetRepoVisibility(true)
	return a.callback.SetRepoUpdateTime(ctx, req)
}

func (a *Activities) UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("update repo infos start", "req", req)
	a.callback.SetRepoVisibility(true)
	return a.callback.UpdateRepoInfos(ctx, req)
}

func (a *Activities) SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	logger := activity.GetLogger(ctx)
	logger.Info("sensitive check start", "req", req)
	a.callback.SetRepoVisibility(true)
	return a.callback.SensitiveCheck(ctx, req)
}
