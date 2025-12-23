//go:build !saas && !ee

package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type userPhoneComponentImpl struct {
	sso    rpc.SSOInterface
	config *config.Config
}

func NewUserPhoneComponent(config *config.Config) (UserPhoneComponent, error) {
	sso, err := rpc.NewSSOClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sso client, error: %w", err)
	}
	return &userPhoneComponentImpl{
		sso:    sso,
		config: config,
	}, nil
}

func (c *userPhoneComponentImpl) NeedPhoneChange() bool {
	return true
}

func (c *userPhoneComponentImpl) isSSOUser(regProvider string) bool {
	if regProvider == "" {
		return false
	}
	return regProvider == c.config.SSOType
}

func (c *userPhoneComponentImpl) CanChangePhone(ctx context.Context, user *database.User, newPhone string) (bool, error) {
	if !c.isSSOUser(user.RegProvider) {
		return true, nil
	}

	exist, err := c.sso.IsExistByPhone(ctx, newPhone)
	if err != nil {
		slog.ErrorContext(ctx, "failed to check new phone existence in sso", "error", err)
		return false, err
	}

	if exist {
		return false, errorx.ErrPhoneAlreadyExistsInSSO
	}

	return true, nil
}

func (c *userPhoneComponentImpl) UpdatePhone(ctx context.Context, uid string, req types.UpdateUserPhoneRequest) error {
	return nil
}

func (c *userPhoneComponentImpl) SendSMSCode(ctx context.Context, uid string, req types.SendSMSCodeRequest) (*types.SendSMSCodeResponse, error) {
	return nil, nil
}

func (c *userPhoneComponentImpl) SendPublicSMSCode(ctx context.Context, req types.SendPublicSMSCodeRequest) (*types.SendSMSCodeResponse, error) {
	return nil, nil
}

func (c *userPhoneComponentImpl) VerifyPublicSMSCode(ctx context.Context, req types.VerifyPublicSMSCodeRequest) error {
	return nil
}
