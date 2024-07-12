package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewAccessTokenComponent(config *config.Config) (*AccessTokenComponent, error) {
	c := &AccessTokenComponent{}
	c.ts = database.NewAccessTokenStore()
	c.us = database.NewUserStore()
	var err error
	c.gs, err = git.NewGitServer(config)
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

	exist, err := c.ts.IsExist(ctx, req.Username, req.TokenName, string(req.Application))
	if err != nil {
		return nil, fmt.Errorf("fail to check if token exists,error:%w", err)
	}
	if exist {
		return nil, fmt.Errorf("token name duplicated, token_name:%s, app:%s", req.TokenName, req.Application)
	}

	var token *database.AccessToken
	// csghub token is shared with git server
	if req.Application == types.AccessTokenAppGit {
		token, err = c.gs.CreateUserToken(req)
		if err != nil {
			return nil, fmt.Errorf("fail to create git user access token,error:%w", err)
		}
		token.UserID = user.ID
		token.Application = req.Application
	} else {
		tokenValue := c.genUnique()
		token = &database.AccessToken{
			Name:        req.TokenName,
			Token:       tokenValue,
			UserID:      user.ID,
			Application: req.Application,
			Permission:  req.Permission,
			IsActive:    true,
		}
	}

	if req.ExpiredAt.After(time.Now()) {
		token.ExpiredAt = req.ExpiredAt
	}
	err = c.ts.Create(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("fail to create database user access token,error:%w", err)
	}

	return token, nil
}

func (c *AccessTokenComponent) genUnique() string {
	// TODO:change
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (c *AccessTokenComponent) Delete(ctx context.Context, req *types.DeleteUserTokenRequest) error {
	ue, err := c.us.IsExist(ctx, req.Username)
	if !ue {
		return fmt.Errorf("user does not exists,error:%w", err)
	}
	te, err := c.ts.IsExist(ctx, req.Username, req.TokenName, string(req.Application))
	if !te {
		return fmt.Errorf("user access token does not exists,error:%w", err)
	}

	if req.Application == types.AccessTokenAppGit {
		err = c.gs.DeleteUserToken(req)
		if err != nil {
			return fmt.Errorf("failed to delete git user access token,error:%w", err)
		}
	}

	err = c.ts.Delete(ctx, req.Username, req.TokenName, string(req.Application))
	if err != nil {
		return fmt.Errorf("failed to delete database user access token,error,error:%w", err)
	}
	return nil
}

func (c *AccessTokenComponent) Check(ctx context.Context, req *types.CheckAccessTokenReq) (types.CheckAccessTokenResp, error) {
	var resp types.CheckAccessTokenResp
	t, err := c.ts.FindByToken(ctx, req.Token, req.Application)
	if err != nil {
		return resp, fmt.Errorf("failed to find database user access token,error:%w", err)
	}

	resp.Token = t.Token
	resp.TokenName = t.Name
	resp.Application = t.Application
	resp.Permission = t.Permission
	resp.Username = t.User.Username
	resp.UserUUID = t.User.UUID
	resp.ExpireAt = t.ExpiredAt
	return resp, nil
}

func (c *AccessTokenComponent) GetTokens(ctx context.Context, username, app string) ([]types.CheckAccessTokenResp, error) {
	var resps []types.CheckAccessTokenResp
	tokens, err := c.ts.FindByUser(ctx, username, app)
	if err != nil {
		return nil, fmt.Errorf("failed to find database user access token,error:%w", err)
	}

	for _, t := range tokens {
		var resp types.CheckAccessTokenResp
		resp.Token = t.Token
		resp.TokenName = t.Name
		resp.Application = t.Application
		resp.Permission = t.Permission
		resp.Username = t.User.Username
		resp.UserUUID = t.User.UUID
		resp.ExpireAt = t.ExpiredAt

		resps = append(resps, resp)
	}
	return resps, nil
}

func (c *AccessTokenComponent) RefreshToken(ctx context.Context, userName, tokenName, app string, newExpiredAt time.Time) (types.CheckAccessTokenResp, error) {
	var resp types.CheckAccessTokenResp
	t, err := c.ts.FindByTokenName(ctx, userName, tokenName, app)
	if err != nil {
		return types.CheckAccessTokenResp{}, fmt.Errorf("failed to find database user access token,error:%w", err)
	}

	var newTokenValue string
	req := &types.CreateUserTokenRequest{
		Username:    userName,
		TokenName:   t.Name,
		Application: t.Application,
		Permission:  t.Permission,
	}
	// csghub token is shared with git server
	if req.Application == "" || req.Application == types.AccessTokenAppCSGHub {
		// TODO:allow git client to refresh token
		// git server cannot create tokens with the same nanme
		err := c.gs.DeleteUserToken(&types.DeleteUserTokenRequest{
			Username:  userName,
			TokenName: t.Name,
		})
		if err != nil {
			return resp, fmt.Errorf("fail to delete old git user access token,error:%w", err)
		}
		newToken, err := c.gs.CreateUserToken(req)
		if err != nil {
			return resp, fmt.Errorf("fail to create git user access token,error:%w", err)
		}
		newTokenValue = newToken.Token
	} else {
		newTokenValue = c.genUnique()
	}

	newToken, err := c.ts.Refresh(ctx, t, newTokenValue, newExpiredAt)
	if err != nil {
		return resp, fmt.Errorf("fail to refresh access token with new token value,error:%w", err)
	}

	resp.Token = newToken.Token
	resp.TokenName = newToken.Name
	resp.Application = newToken.Application
	resp.Permission = newToken.Permission
	resp.Username = newToken.User.Username
	resp.UserUUID = newToken.User.UUID
	resp.ExpireAt = newToken.ExpiredAt

	return resp, nil
}
