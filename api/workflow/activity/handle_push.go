package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/callback"
)

func WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("watch space change start", "req", req)
	callbackComponent, err := callback.NewGitCallback(config)
	if err != nil {
		return fmt.Errorf("failed to create callback component, error: %w", err)
	}
	callbackComponent.SetRepoVisibility(true)
	return callbackComponent.WatchSpaceChange(ctx, req)
}

func WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("watch repo relation start", "req", req)
	callbackComponent, err := callback.NewGitCallback(config)
	if err != nil {
		return fmt.Errorf("failed to create callback component, error: %w", err)
	}
	callbackComponent.SetRepoVisibility(true)
	return callbackComponent.WatchRepoRelation(ctx, req)
}

func SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("set repo update time start", "req", req)
	callbackComponent, err := callback.NewGitCallback(config)
	if err != nil {
		return fmt.Errorf("failed to create callback component, error: %w", err)
	}
	callbackComponent.SetRepoVisibility(true)
	return callbackComponent.SetRepoUpdateTime(ctx, req)
}

func UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("update repo infos start", "req", req)
	callbackComponent, err := callback.NewGitCallback(config)
	if err != nil {
		return fmt.Errorf("failed to create callback component, error: %w", err)
	}
	callbackComponent.SetRepoVisibility(true)
	return callbackComponent.UpdateRepoInfos(ctx, req)
}

func SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("sensitive check start", "req", req)
	callbackComponent, err := callback.NewGitCallback(config)
	if err != nil {
		return fmt.Errorf("failed to create callback component, error: %w", err)
	}
	callbackComponent.SetRepoVisibility(true)
	return callbackComponent.SensitiveCheck(ctx, req)
}
