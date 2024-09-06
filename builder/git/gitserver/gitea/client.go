package gitea

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var _ gitserver.GitServer = (*Client)(nil)

type Client struct {
	giteaClient *gitea.Client
	config      *config.Config
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type TokenResponse struct {
	SHA1    string `json:"sha1"`
	Message string `json:"message"`
}

func NewClient(config *config.Config) (client *Client, err error) {
	httpClient := &http.Client{
		Timeout: time.Duration(config.GitServer.TimtoutSEC) * time.Second,
	}
	token, err := findOrCreateAccessToken(context.Background(), config)
	if err != nil {
		slog.Error("Failed to find or create token", slog.String("error: ", err.Error()))
		return nil, err
	}
	giteaClient, err := gitea.NewClient(
		config.GitServer.Host,
		gitea.SetHTTPClient(httpClient),
		gitea.SetToken(token.Token),
		gitea.SetBasicAuth(config.GitServer.Username, config.GitServer.Password),
	)
	if err != nil {
		return nil, err
	}

	return &Client{giteaClient: giteaClient, config: config}, nil
}

func findOrCreateAccessToken(ctx context.Context, config *config.Config) (*database.GitServerAccessToken, error) {
	gs := database.NewGitServerAccessTokenStore()
	tokens, err := gs.FindByType(ctx, "git")
	if err != nil {
		slog.Error("Fail to get git server access token from database", slog.String("error: ", err.Error()))
		return nil, err
	}

	if len(tokens) == 0 {
		access_token, err := generateAccessTokenFromGitea(config)
		if err != nil {
			slog.Error("Fail to create git server access token", slog.String("error: ", err.Error()))
			return nil, err
		}
		gToken := &database.GitServerAccessToken{
			Token:      access_token,
			ServerType: "git",
		}

		gToken, err = gs.Create(ctx, gToken)
		if err != nil {
			slog.Error("Fail to create git server access token", slog.String("error: ", err.Error()))
			return nil, err
		}

		return gToken, nil
	}
	return &tokens[0], nil
}

func encodeCredentials(username, password string) string {
	credentials := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(credentials))
}

func generateAccessTokenFromGitea(config *config.Config) (string, error) {
	username := config.GitServer.Username
	password := config.GitServer.Password
	giteaUrl := fmt.Sprintf("%s/api/v1/users/%s/tokens", config.GitServer.Host, username)
	authHeader := encodeCredentials(username, password)
	data := map[string]any{
		"name": "access_token",
		"scopes": []string{
			"write:user",
			"write:admin",
			"write:organization",
			"write:repository",
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("Error encoding JSON data:", err)
		return "", err
	}

	req, err := http.NewRequest("POST", giteaUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Error creating request:", err)
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+authHeader)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading response body:", err)
		return "", err
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		slog.Error("Error decoding JSON response:", err)
		return "", err
	}

	if tokenResponse.Message != "" {
		return "", errors.New(tokenResponse.Message)
	}

	return tokenResponse.SHA1, nil
}
