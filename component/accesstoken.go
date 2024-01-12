package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewAccessTokenComponent(config *config.Config) (*AccessTokenComponent, error) {
	c := &AccessTokenComponent{}
	c.ts = database.NewAccessTokenStore()
	c.us = database.NewUserStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type AccessTokenComponent struct {
	ts *database.AccessTokenStore
	us *database.UserStore
	gs gitserver.GitServer
}

func (c *AccessTokenComponent) Create(ctx context.Context, req *types.CreateUserTokenRequest) (*database.AccessToken, error) {
	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to find user,error:%w", err)
	}

	token, err := c.gs.CreateUserToken(req)
	if err != nil {
		return nil, fmt.Errorf("fail to create git user access token,error:%w", err)
	}

	token.UserID = user.ID
	err = c.ts.Create(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("fail to create database user access token,error:%w", err)
	}
	return token, nil
}

func (c *AccessTokenComponent) Delete(ctx context.Context, req *types.DeleteUserTokenRequest) error {
	ue, err := c.us.IsExist(ctx, req.Username)
	if !ue {
		return fmt.Errorf("user does not exists,error:%w", err)
	}
	te, err := c.ts.IsExist(ctx, req.Username, req.Name)
	if !te {
		return fmt.Errorf("user access token does not exists,error:%w", err)
	}

	err = c.gs.DeleteUserToken(req)
	if err != nil {
		return fmt.Errorf("failed to delete git user access token,error:%w", err)
	}

	err = c.ts.Delete(ctx, req.Username, req.Name)
	if err != nil {
		return fmt.Errorf("failed to delete database user access token,error,error:%w", err)
	}
	return nil
}
