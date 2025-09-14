package rpc

import (
	"context"
	"fmt"
	"os"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"golang.org/x/oauth2"
	"opencsg.com/csghub-server/common/config"
)

const (
	SSOTypeCasdoor  = "casdoor"
	SSOTypeParaview = "paraview"
)

type SSOInterface interface {
	UpdateUserInfo(ctx context.Context, userInfo *SSOUpdateUserInfo) error
	GetUserInfo(ctx context.Context, accessToken string) (*SSOUserInfo, error)
	GetOAuthToken(ctx context.Context, code, state string) (*oauth2.Token, error)
	DeleteUser(ctx context.Context, uuid string) error
	IsExistByName(ctx context.Context, name string) (bool, error)
	IsExistByEmail(ctx context.Context, email string) (bool, error)
	IsExistByPhone(ctx context.Context, phone string) (bool, error)
}

func NewSSOClient(config *config.Config) (SSOInterface, error) {
	switch config.SSOType {
	case SSOTypeCasdoor:
		certData, err := os.ReadFile(config.Casdoor.Certificate)
		if err != nil {
			return nil, fmt.Errorf("failed to read casdoor certificate file,error:%w", err)
		}
		return NewCasdoorClient(&casdoorsdk.AuthConfig{
			Endpoint:         config.Casdoor.Endpoint,
			ClientId:         config.Casdoor.ClientID,
			ClientSecret:     config.Casdoor.ClientSecret,
			Certificate:      string(certData),
			OrganizationName: config.Casdoor.OrganizationName,
			ApplicationName:  config.Casdoor.ApplicationName,
		}), nil
	case SSOTypeParaview:
		return NewParaviewClient(config), nil
	default:
		return nil, fmt.Errorf("invalid sso type: %s", config.SSOType)
	}
}

type SSOUserInfo struct {
	WeChat         string
	Name           string
	Email          string
	UUID           string
	RegProvider    string
	Gender         string
	Phone          string
	PhoneArea      string
	LastSigninTime string
	Avatar         string
	Homepage       string
	Bio            string
}

type SSOUpdateUserInfo struct {
	UUID      string
	Name      string
	Email     string
	Gender    string
	Phone     string
	PhoneArea string
}
