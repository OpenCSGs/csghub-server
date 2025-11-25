package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type CodeAdapter struct {
	cfg            *config.Config
	agentComponent component.AgentComponent
	usc            rpc.UserSvcClient
}

func NewCodeAdapter(cfg *config.Config, agentComponent component.AgentComponent) *CodeAdapter {
	usc := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", cfg.User.Host, cfg.User.Port),
		rpc.AuthWithApiKey(cfg.APIToken))
	return &CodeAdapter{cfg: cfg, agentComponent: agentComponent, usc: usc}
}

func (a *CodeAdapter) Name() string { return "code" }

func (a *CodeAdapter) GetHost(ctx context.Context) (string, error) {
	host := fmt.Sprintf("%s:%d", a.cfg.CSGBot.Host, a.cfg.CSGBot.Port)
	return host, nil
}

func (a *CodeAdapter) PrepareProxyContext(ctx *gin.Context, api string) error {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	currentUser := httpbase.GetCurrentUser(ctx)

	token, err := a.usc.GetOrCreateFirstAvaiTokens(ctx.Request.Context(), currentUser, currentUser, string(types.AccessTokenAppGit), "csgbot")
	if err != nil {
		slog.Error("failed to get or create user first git access token", "agent_type", "code", "user_uuid", currentUserUUID, "api", api, "error", err)
		return err
	}
	if len(token) == 0 {
		slog.Error("can not get user first available access token", "agent_type", "code", "user_uuid", currentUserUUID, "api", api)
		return fmt.Errorf("can not get user first available access token")
	}

	ctx.Request.Header.Set("user_uuid", currentUserUUID)
	ctx.Request.Header.Set("user_name", currentUser)
	ctx.Request.Header.Set("user_token", token)

	slog.Debug("code adapter preparing proxy context", "agent_type", "code", "api", api, "user_uuid", currentUserUUID, "request_headers", ctx.Request.Header)

	if ctx.Request.Method != http.MethodPost || !strings.HasPrefix(api, "/api/v1/csgbot/") {
		return nil
	}

	var codeReq types.CodeAgentRequest
	if err := ctx.ShouldBindJSON(&codeReq); err != nil {
		slog.Error("failed to parse code agent request", "agent_type", "code", "api", api, "user_uuid", currentUserUUID, "error", err)
		return fmt.Errorf("parse code agent request: %w", err)
	}

	if codeReq.Stream {
		// ctx.Writer.Header().Set("Content-Type", "text/event-stream") // the response content type is already set by the code agent server
		ctx.Writer.Header().Set("Cache-Control", "no-cache")
		ctx.Writer.Header().Set("Connection", "keep-alive")
	}

	// Create session for code agent
	sessionName := common.TruncStringByRune(codeReq.Query, 255)
	sessionUUID, err := a.agentComponent.CreateSession(ctx, currentUserUUID, &types.CreateAgentInstanceSessionRequest{
		SessionUUID: &codeReq.RequestID,
		Name:        &sessionName,
		Type:        "code",
		ContentID:   &codeReq.AgentName,
	})
	if err != nil {
		slog.Error("failed to create code agent session", "agent_type", "code", "api", api, "user_uuid", currentUserUUID, "error", err)
		return fmt.Errorf("create code agent session: %w", err)
	}

	// Set session ID in the request
	codeReq.RequestID = sessionUUID
	data, _ := json.Marshal(codeReq)
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	ctx.Request.ContentLength = int64(len(data))

	return nil
}
