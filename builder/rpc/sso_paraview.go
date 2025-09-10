package rpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
)

type paraviewClientImpl struct {
	endpoint     string
	clientID     string
	clientSecret string
	redirectURI  string
	apiKey       string
	apiSecret    string
}

func NewParaviewClient(config *config.Config) SSOInterface {
	return &paraviewClientImpl{
		endpoint:     config.Paraview.Endpoint,
		clientID:     config.Paraview.ClientID,
		clientSecret: config.Paraview.ClientSecret,
		redirectURI:  config.Paraview.RedirectURI,
		apiKey:       config.Paraview.ApiKey,
		apiSecret:    config.Paraview.ApiSecret,
	}
}

func (p *paraviewClientImpl) getBasicToken() string {
	secret := fmt.Sprintf("%s:%s", p.apiKey, p.apiSecret)
	token := base64.StdEncoding.EncodeToString([]byte(secret))
	return fmt.Sprintf("Basic %s", token)
}

func (p *paraviewClientImpl) GetOAuthToken(ctx context.Context, code, state string) (*oauth2.Token, error) {
	endpoint := fmt.Sprintf("%s/esc-sso/oauth2.0/accessToken?grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&client_secret=%s", p.endpoint, code, p.redirectURI, p.clientID, p.clientSecret)

	resp, err := http.Post(endpoint, "x-www-form-urlencoded", nil)
	if err != nil {
		slog.Error("GetOAuthToken Failed from paraview", slog.Any("err", err), slog.Any("url", endpoint), slog.Any("code", code))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "paraview").
				Set("code", code),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		slog.Error("GetOAuthToken Failed", slog.Any("status_code", resp.StatusCode), slog.Any("body", string(body)))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "paraview").
				Set("code", code),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (p *paraviewClientImpl) GetUserInfo(ctx context.Context, accessToken string) (*SSOUserInfo, error) {
	endpoint := fmt.Sprintf("%s/esc-sso/oauth2.0/profile?access_token=%s", p.endpoint, accessToken)
	resp, err := http.Get(endpoint)
	if err != nil {
		slog.Error("GetUserInfo Failed from paraview", slog.Any("err", err), slog.Any("url", endpoint), slog.Any("access_token", accessToken))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "paraview").
				Set("access_token", accessToken),
		)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		slog.Error("GetUserInfo Failed from paraview", slog.Any("status_code", resp.StatusCode), slog.Any("body", string(body)))
		return nil, errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "paraview").
				Set("access_token", accessToken),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user map[string]string
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	userInfo := &SSOUserInfo{
		WeChat:         "",
		Name:           user["user_name"],
		Email:          user["email"],
		UUID:           user["custom_unique"],
		RegProvider:    SSOTypeParaview,
		Gender:         user["gender"],
		Phone:          user["mobile"],
		LastSigninTime: user["last_login_time"],
	}

	return userInfo, nil
}

func (p *paraviewClientImpl) UpdateUserInfo(ctx context.Context, userInfo *SSOUpdateUserInfo) error {
	endpoint := fmt.Sprintf("%s/esc-idm/api/v1/user/sync", p.endpoint)

	params := make(map[string]string)
	params["idt_user__custom_unique"] = userInfo.UUID
	params["idt_user__user_name"] = userInfo.Name
	params["idt_user__email"] = userInfo.Email
	params["idt_user__mobile"] = userInfo.Phone
	params["idt_user__gender"] = userInfo.Gender

	reqBody, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", p.getBasicToken())
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("UpdateUserInfo Failed from paraview", slog.Any("err", err), slog.Any("url", endpoint), slog.Any("userInfo", userInfo))
		return errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "paraview").
				Set("userInfo", userInfo),
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		slog.Error("UpdateUserInfo Failed from paraview", slog.Any("status_code", resp.StatusCode), slog.Any("body", string(body)))
		return errorx.RemoteSvcFail(err,
			errorx.Ctx().Set("service", "paraview").
				Set("userInfo", userInfo),
		)
	}

	return nil
}

func (p *paraviewClientImpl) IsExistByName(ctx context.Context, name string) (bool, error) {
	return false, nil
}

func (p *paraviewClientImpl) IsExistByEmail(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func (p *paraviewClientImpl) IsExistByPhone(ctx context.Context, phone string) (bool, error) {
	return false, nil
}

func (p *paraviewClientImpl) DeleteUser(ctx context.Context, uuid string) error {
	// No interface has been provided yet
	return nil
}
