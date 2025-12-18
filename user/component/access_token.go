package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var ErrUserNotFound = errors.New("user not found, please login first")

type AccessTokenComponent interface {
	Create(ctx context.Context, req *types.CreateUserTokenRequest) (*database.AccessToken, error)
	Delete(ctx context.Context, req *types.DeleteUserTokenRequest) error
	Check(ctx context.Context, req *types.CheckAccessTokenReq) (types.CheckAccessTokenResp, error)
	GetTokens(ctx context.Context, username, app string) ([]types.CheckAccessTokenResp, error)
	RefreshToken(ctx context.Context, userName, tokenName, app string, newExpiredAt time.Time) (types.CheckAccessTokenResp, error)
	GetOrCreateFirstAvaiToken(ctx context.Context, userName, app, tokenName string) (string, error)
}

func NewAccessTokenComponent(config *config.Config) (AccessTokenComponent, error) {
	var err error
	ac, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create accounting clent,error:%w", err)
	}
	c := &accessTokenComponentImpl{}
	c.ts = database.NewAccessTokenStore()
	c.us = database.NewUserStore()
	c.gs, err = git.NewGitServer(config)
	c.acctClient = ac
	c.config = config
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.ErrorContext(context.Background(), newError.Error())
		return nil, newError
	}
	return c, nil
}

type accessTokenComponentImpl struct {
	ts         database.AccessTokenStore
	us         database.UserStore
	gs         gitserver.GitServer
	acctClient accounting.AccountingClient
	config     *config.Config
}

func (c *accessTokenComponentImpl) Create(ctx context.Context, req *types.CreateUserTokenRequest) (*database.AccessToken, error) {
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
		if c.gs != nil {
			token, err = c.gs.CreateUserToken(req)
			if err != nil {
				return nil, fmt.Errorf("fail to create git user access token,error:%w", err)
			}
		} else {
			tokenContent := c.genUnique()
			token = &database.AccessToken{
				Name:        req.TokenName,
				Token:       tokenContent,
				UserID:      user.ID,
				Application: req.Application,
				Permission:  req.Permission,
				IsActive:    true,
			}
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

	err = c.createUserToken(ctx, token, user)
	if err != nil {
		return nil, fmt.Errorf("fail to create database user access token,error:%w", err)
	}

	if req.Application == types.AccessTokenAppMirror {
		quota, err := c.acctClient.GetQuotaByID(req.Username)
		if err != nil {
			return nil, fmt.Errorf("fail to get quota by username,error:%w", err)
		}
		if quota == nil {
			_, err := c.acctClient.CreateOrUpdateQuota(req.Username, types.AcctQuotaReq{
				RepoCountLimit: c.config.MultiSync.DefaultRepoCountLimit,
				SpeedLimit:     c.config.MultiSync.DefaultSpeedLimit,
				TrafficLimit:   c.config.MultiSync.DefaultTrafficLimit,
			})
			if err != nil {
				return nil, fmt.Errorf("fail to create quota for new mirror token,error:%w", err)
			}
		}
	}

	return token, nil
}

func (c *accessTokenComponentImpl) genUnique() string {
	// TODO:change
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (c *accessTokenComponentImpl) Delete(ctx context.Context, req *types.DeleteUserTokenRequest) error {
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

func (c *accessTokenComponentImpl) Check(ctx context.Context, req *types.CheckAccessTokenReq) (types.CheckAccessTokenResp, error) {
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

func (c *accessTokenComponentImpl) GetTokens(ctx context.Context, username, app string) ([]types.CheckAccessTokenResp, error) {
	var resps []types.CheckAccessTokenResp
	tokens, err := c.ts.FindByUser(ctx, username, app)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

func (c *accessTokenComponentImpl) RefreshToken(ctx context.Context, userName, tokenName, app string, newExpiredAt time.Time) (types.CheckAccessTokenResp, error) {
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

func (c *accessTokenComponentImpl) GetOrCreateFirstAvaiToken(ctx context.Context, userName, app, tokenName string) (string, error) {
	tokens, err := c.GetTokens(ctx, userName, app)
	if err != nil {
		return "", fmt.Errorf("failed to select user %s access %s tokens, error:%w", userName, app, err)
	}
	if len(tokens) > 0 {
		return tokens[0].Token, nil
	}

	req := types.CreateUserTokenRequest{
		Username:    userName,
		TokenName:   tokenName,
		Application: types.AccessTokenApp(app),
		Permission:  "",
	}

	token, err := c.Create(ctx, &req)
	if err != nil {
		return "", fmt.Errorf("failed to create user %s access %s token, error:%w", userName, app, err)
	}

	return token.Token, nil
}

func (c *accessTokenComponentImpl) createUserToken(ctx context.Context, newToken *database.AccessToken, user database.User) error {
	err := c.ts.Create(ctx, newToken)
	if err != nil {
		return fmt.Errorf("fail to create user %s new %s token, error: %w", user.Username, newToken.Application, err)
	}

	if newToken.Application == types.AccessTokenAppStarship {
		// charge 100 credit for create starship token by call accounting service
		err = c.presentForNewAccessToken(user)
		if err != nil {
			slog.ErrorContext(ctx, "fail to charge for new starship user with retry 3 times", slog.Any("user.uuid", user.UUID), slog.Any("err", err))
		}
	}

	return nil
}

func (c *accessTokenComponentImpl) presentForNewAccessToken(user database.User) error {
	var err error
	req := types.ACTIVITY_REQ{
		ID:     types.StarShipNewUser.ID,
		Value:  types.StarShipNewUser.Value,
		OpUID:  user.Username,
		OpDesc: types.StarShipNewUser.OpDesc,
	}
	// retry 3 time
	for i := 0; i < 3; i++ {
		_, err = c.acctClient.PresentAccountingUser(user.UUID, req)
		if err == nil {
			return nil
		}
	}
	return err
}
