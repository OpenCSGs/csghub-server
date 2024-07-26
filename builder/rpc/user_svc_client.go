package rpc

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/git/membership"
)

type UserSvcClien interface {
	GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error)
}

//go:generate mockgen -destination=mocks/client.go -package=mocks . Client

type UserSvcHttpClient struct {
	hc *HttpClient
}

func NewUserSvcHttpClient(endpoint string, opts ...RequestOption) UserSvcClien {
	return &UserSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *UserSvcHttpClient) GetMemberRole(ctx context.Context, orgName, userName string) (membership.Role, error) {
	// write code to call user service api "/api/v1/organization/{orgName}/members/{userName}"
	url := fmt.Sprintf("/api/v1/organization/%s/members/%s", orgName, userName)
	var r httpbase.R
	err := c.hc.Get(ctx, url, &r)
	if err != nil {
		return membership.RoleUnkown, fmt.Errorf("failed to get member role: %w", err)
	}

	role, ok := r.Data.(string)
	if !ok {
		return membership.RoleUnkown, fmt.Errorf("failed to convert r.Data to membership.Role")
	}
	return membership.Role(role), nil
}
