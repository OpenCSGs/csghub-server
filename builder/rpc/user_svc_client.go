package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type UserSvcClient interface {
	GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error)
	GetNameSpaceInfo(ctx context.Context, path string) (*Namespace, error)
	GetUserInfo(ctx context.Context, userName, visitorName string) (*User, error)
	GetOrCreateFirstAvaiTokens(ctx context.Context, userName, visitorName, app, tokenName string) (string, error)
	VerifyByAccessToken(ctx context.Context, token string) (*types.CheckAccessTokenResp, error)
	GetUserByName(ctx context.Context, userName string) (*types.User, error)
	FindByUUIDs(ctx context.Context, uuids []string) (map[string]*types.User, error)
	GetUserUUIDs(ctx context.Context, per, page int) ([]string, int, error)
	GetEmails(ctx context.Context, per, page int) ([]string, int, error)
}

//go:generate mockgen -destination=mocks/client.go -package=mocks . Client

type UserSvcHttpClient struct {
	hc *HttpClient
}

func NewUserSvcHttpClient(endpoint string, opts ...RequestOption) UserSvcClient {
	return &UserSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *UserSvcHttpClient) GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error) {
	// write code to call user service api "/api/v1/organization/{orgName}/members/{userName}"
	url := fmt.Sprintf("/api/v1/organization/%s/members/%s?current_user=%s", orgName, userName, userName)
	var r httpbase.R
	r.Data = membership.RoleUnknown
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return membership.RoleUnknown, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get member role").
				Set("orgName", orgName).
				Set("userName", userName))
	}

	role, ok := r.Data.(string)
	if !ok {
		return membership.RoleUnknown, errorx.InternalServerError(fmt.Errorf("failed to convert r.Data '%v' to membership.Role", r.Data), nil)
	}
	return membership.Role(role), nil
}

func (c *UserSvcHttpClient) GetNameSpaceInfo(ctx context.Context, path string) (*Namespace, error) {
	// write code to call user service api "/api/v1/namespace/{path}"
	url := fmt.Sprintf("/api/v1/namespace/%s", path)
	var r httpbase.R
	r.Data = &Namespace{}
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get namespace info").
				Set("path", path))
	}

	return r.Data.(*Namespace), nil
}

func (c *UserSvcHttpClient) GetUserInfo(ctx context.Context, userName, visitorName string) (*User, error) {
	url := fmt.Sprintf("/api/v1/user/%s?current_user=%s", userName, visitorName)
	var r httpbase.R
	r.Data = &User{}
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get user info").
				Set("userName", userName).
				Set("visitorName", visitorName))
	}

	return r.Data.(*User), nil
}

func (c *UserSvcHttpClient) GetOrCreateFirstAvaiTokens(ctx context.Context, userName, visitorName, app, tokenName string) (string, error) {
	url := fmt.Sprintf("/api/v1/user/%s/tokens/first?current_user=%s&app=%s&token_name=%s", userName, visitorName, app, tokenName)
	var r httpbase.R
	r.Data = interface{}("")
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return "", errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get or create tokens").
				Set("userName", userName).
				Set("app", app))
	}
	return r.Data.(string), nil
}

func (c *UserSvcHttpClient) VerifyByAccessToken(ctx context.Context, token string) (*types.CheckAccessTokenResp, error) {
	url := fmt.Sprintf("/api/v1/token/%s", token)
	var r httpbase.R
	r.Data = &types.CheckAccessTokenResp{}
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "verify access token"))
	}

	return r.Data.(*types.CheckAccessTokenResp), nil
}

func (c *UserSvcHttpClient) GetUserByName(ctx context.Context, userName string) (*types.User, error) {
	url := fmt.Sprintf("/api/v1/user/%s", userName)
	var r httpbase.R
	r.Data = &types.User{}
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get user by name").
				Set("userName", userName))
	}

	return r.Data.(*types.User), nil
}

func (c *UserSvcHttpClient) FindByUUIDs(ctx context.Context, uuids []string) (map[string]*types.User, error) {
	params := url.Values{}
	for _, uuid := range uuids {
		params.Add("uuids", uuid)
	}
	url := fmt.Sprintf("/api/v1/users/by-uuids?%s", params.Encode())
	var resp struct {
		Msg  string        `json:"msg"`
		Data []*types.User `json:"data"`
	}
	err := c.hc.Get(ctx, url, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "find users by uuids").
				Set("uuidCount", len(uuids)))
	}
	result := make(map[string]*types.User)
	if resp.Data != nil {
		for _, user := range resp.Data {
			if user != nil && user.UUID != "" {
				result[user.UUID] = user
			}
		}
	}
	return result, nil
}

func (c *UserSvcHttpClient) GetUserUUIDs(ctx context.Context, per, page int) ([]string, int, error) {
	url := fmt.Sprintf("/api/v1/user/user_uuids?per=%d&page=%d", per, page)
	var resp struct {
		Data struct {
			UserUUIDs []string `json:"data"`
			Total     int      `json:"total"`
		} `json:"data"`
	}
	err := c.hc.Get(ctx, url, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, 0, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get user uuids").
				Set("per", per).
				Set("page", page))
	}
	return resp.Data.UserUUIDs, resp.Data.Total, nil
}

func (c *UserSvcHttpClient) GetEmails(ctx context.Context, per, page int) ([]string, int, error) {
	url := fmt.Sprintf("/api/v1/internal/user/emails?per=%d&page=%d", per, page)
	var resp struct {
		Msg   string   `json:"msg"`
		Data  []string `json:"data"`
		Total int      `json:"total"`
	}
	err := c.hc.Get(ctx, url, &resp)
	if err != nil {
		slog.ErrorContext(ctx, "call user service failed", slog.String("error", err.Error()))
		return nil, 0, errorx.RemoteSvcFail(err,
			errorx.Ctx().
				Set("service", "user service").
				Set("action", "get user emails").
				Set("per", per).
				Set("page", page))
	}
	return resp.Data, resp.Total, nil
}
