package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/user/workflow/common"
)

func DeleteUserAndRelations(ctx context.Context, user common.User, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("delete user and relations start", "username", user.Username, "operator", user.Operator)
	userComponent, err := component.NewUserComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create user component, error: %w", err)
	}
	return userComponent.Delete(ctx, user.Operator, user.Username)
}

func SoftDeleteUserAndRelations(ctx context.Context, user common.User, req types.CloseAccountReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("delete user and relations start", "username", user.Username, "operator", user.Operator)
	userComponent, err := component.NewUserComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create user component, error: %w", err)
	}
	return userComponent.SoftDelete(ctx, user.Operator, user.Username, req)
}
