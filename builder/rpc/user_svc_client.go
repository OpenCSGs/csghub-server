package rpc

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/common/types"
)

type UserSvcClient interface {
	GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error)
	GetNameSpaceInfo(ctx context.Context, path string) (*Namespace, error)
	GetUserInfo(ctx context.Context, userName, visitorName string) (*User, error)
	GetOrCreateFirstAvaiTokens(ctx context.Context, userName, visitorName, app, tokenName string) (string, error)
	VerifyByAccessToken(ctx context.Context, token string) (*types.CheckAccessTokenResp, error)
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
		return membership.RoleUnknown, fmt.Errorf("failed to get member role: %w", err)
	}

	role, ok := r.Data.(string)
	if !ok {
		return membership.RoleUnknown, fmt.Errorf("failed to convert r.Data '%v' to membership.Role", r.Data)
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
		return nil, fmt.Errorf("failed to get namespace '%s' info: %w", path, err)
	}

	return r.Data.(*Namespace), nil
}

func (c *UserSvcHttpClient) GetUserInfo(ctx context.Context, userName, visitorName string) (*User, error) {
	url := fmt.Sprintf("/api/v1/user/%s?current_user=%s", userName, visitorName)
	var r httpbase.R
	r.Data = &User{}
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		return nil, fmt.Errorf("failed to get user '%s' info: %w", userName, err)
	}

	return r.Data.(*User), nil
}

func (c *UserSvcHttpClient) GetOrCreateFirstAvaiTokens(ctx context.Context, userName, visitorName, app, tokenName string) (string, error) {
	url := fmt.Sprintf("/api/v1/user/%s/tokens/first?current_user=%s&app=%s&token_name=%s", userName, visitorName, app, tokenName)
	var r httpbase.R
	r.Data = interface{}("")
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		return "", fmt.Errorf("failed to get user '%s' token for %s: %w", userName, app, err)
	}
	return r.Data.(string), nil
}

func (c *UserSvcHttpClient) VerifyByAccessToken(ctx context.Context, token string) (*types.CheckAccessTokenResp, error) {
	url := fmt.Sprintf("/api/v1/token/%s", token)
	var r httpbase.R
	r.Data = &types.CheckAccessTokenResp{}
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		return nil, fmt.Errorf("failed to verify access token info: %w", err)
	}

	return r.Data.(*types.CheckAccessTokenResp), nil
}
