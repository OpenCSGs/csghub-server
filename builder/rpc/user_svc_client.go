package rpc

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/git/membership"
)

type UserSvcClient interface {
	GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error)
	GetNameSpaceInfo(ctx context.Context, path string) (*Namespace, error)
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
	r.Data = membership.RoleUnkown
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		return membership.RoleUnkown, fmt.Errorf("failed to get member role: %w", err)
	}

	role, ok := r.Data.(string)
	if !ok {
		return membership.RoleUnkown, fmt.Errorf("failed to convert r.Data '%v' to membership.Role", r.Data)
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
