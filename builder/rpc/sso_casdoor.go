package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"golang.org/x/oauth2"
	"opencsg.com/csghub-server/common/errorx"
)

type casdoorClientImpl struct {
	casClient *casdoorsdk.Client
}

var (
	_ SSOInterface = (*casdoorClientImpl)(nil)
)

func NewCasdoorClient(c *casdoorsdk.AuthConfig) SSOInterface {
	client := casdoorsdk.NewClientWithConf(c)
	return &casdoorClientImpl{
		casClient: client,
	}
}

func (c *casdoorClientImpl) UpdateUserInfo(ctx context.Context, userInfo *SSOUpdateUserInfo) error {
	casu, err := c.casClient.GetUserByUserId(userInfo.UUID)
	if err != nil {
		slog.Error("GetUserByUserId failed from casdoor", "err", err, "uuid", userInfo.UUID)
		return errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("uuid", userInfo.UUID),
		)
	}

	if casu == nil {
		return fmt.Errorf("user not found in casdoor by uuid:%s", userInfo.UUID)
	}

	if userInfo.Email != "" {
		casu.Email = userInfo.Email
	}
	if userInfo.Phone != "" {
		casu.Phone = userInfo.Phone
	}

	// casdoor update user api don't allow empty display name, so we set it
	if casu.DisplayName == "" {
		casu.DisplayName = casu.Name
	}

	// get id by user name before changed
	id := c.casClient.GetId(casu.Name)
	id = url.QueryEscape(id) // wechat user's name may contain special characters
	if userInfo.Name != "" {
		casu.DisplayName = userInfo.Name
	}
	_, err = c.casClient.UpdateUserById(id, casu)
	if err != nil {
		slog.Error("UpdateUserById failed from casdoor", "err", err, "id", id, "userInfo", userInfo)
		return errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("uuid", userInfo.UUID).Set("id", id),
		)
	}

	return nil
}

func (c *casdoorClientImpl) GetUserInfo(ctx context.Context, accessToken string) (*SSOUserInfo, error) {
	claims, err := c.casClient.ParseJwtToken(accessToken)
	if err != nil {
		slog.Error("ParseJwtToken failed from casdoor", "err", err, "accessToken", accessToken)
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("accessToken", accessToken),
		)
	}

	return &SSOUserInfo{
		WeChat:         claims.WeChat,
		Name:           claims.User.Name,
		Email:          claims.User.Email,
		UUID:           claims.User.Id,
		RegProvider:    SSOTypeCasdoor,
		Gender:         claims.User.Gender,
		Phone:          claims.User.Phone,
		LastSigninTime: claims.User.LastSigninTime,
		Avatar:         claims.User.Avatar,
		Homepage:       claims.User.Homepage,
		Bio:            claims.User.Bio,
	}, nil
}

func (c *casdoorClientImpl) GetOAuthToken(ctx context.Context, code, state string) (*oauth2.Token, error) {
	token, err := c.casClient.GetOAuthToken(code, state)
	if err != nil {
		slog.Error("GetOAuthToken failed from casdoor", "err", err, "code", code, "state", state)
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("code", code).Set("state", state),
		)
	}
	return token, nil
}

func (c *casdoorClientImpl) DeleteUser(ctx context.Context, uuid string) error {
	id, err := c.casClient.GetUserByUserId(uuid)
	if err != nil {
		return err
	}
	_, err = c.casClient.DeleteUser(id)
	if err != nil {
		slog.Error("DeleteUser failed from casdoor", "err", err, "uuid", uuid)
		return errorx.ErrRemoteServiceFail
	}

	return nil
}

func (c *casdoorClientImpl) IsExistByName(ctx context.Context, name string) (bool, error) {
	user, err := c.casClient.GetUser(name)
	if err != nil {
		return false, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("name", name),
		)
	}
	return user != nil, nil
}

func (c *casdoorClientImpl) IsExistByEmail(ctx context.Context, email string) (bool, error) {
	user, err := c.casClient.GetUserByEmail(email)
	if err != nil {
		return false, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("email", email),
		)
	}
	return user != nil, nil
}

func (c *casdoorClientImpl) IsExistByPhone(ctx context.Context, phone string) (bool, error) {
	user, err := c.casClient.GetUserByPhone(phone)
	if err != nil {
		return false, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "casdoor").
				Set("phone", phone),
		)
	}
	return user != nil, nil
}
