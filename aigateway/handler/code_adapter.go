package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

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

func (a *CodeAdapter) PrepareResponseWriter(ctx *gin.Context, api string, stream bool) (http.ResponseWriter, error) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	currentUser := httpbase.GetCurrentUser(ctx)

	token, err := a.usc.GetOrCreateFirstAvaiTokens(ctx.Request.Context(), currentUser, currentUser, string(types.AccessTokenAppGit), "csgbot")
	if err != nil {
		httpbase.ServerError(ctx, err)
		ctx.Abort()
		return nil, err
	}
	if len(token) == 0 {
		slog.Error("fail to get or create user first git access token", slog.Any("user", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("can not get user first available access token"))
		ctx.Abort()
		return nil, errors.New("can not get user first available access token")
	}
	ctx.Request.Header.Set("user_uuid", currentUserUUID)
	ctx.Request.Header.Set("user_name", currentUser)
	ctx.Request.Header.Set("user_token", token)

	// For code execution, we might need to handle different types of requests
	slog.Debug("code adapter preparing response", "api", api, "stream", stream, "userUUID", currentUserUUID, "request headers", ctx.Request.Header)

	if !(ctx.Request.Method == http.MethodPost && api == "/api/v1/csgbot/codeAgent") {
		return ctx.Writer, nil
	}

	// Handle code agent request
	var codeReq types.CodeAgentRequest
	if err := ctx.ShouldBindJSON(&codeReq); err != nil {
		return nil, fmt.Errorf("parse code agent request: %w", err)
	}

	if codeReq.Stream {
		ctx.Writer.Header().Set("Content-Type", "text/event-stream")
		ctx.Writer.Header().Set("Cache-Control", "no-cache")
		ctx.Writer.Header().Set("Connection", "keep-alive")
	}

	// Create session for code agent
	sessionName := common.TruncString(codeReq.Query, 50)
	sessionUUID, err := a.agentComponent.CreateSession(ctx, currentUserUUID, &types.CreateAgentInstanceSessionRequest{
		SessionUUID: &codeReq.RequestID,
		Name:        &sessionName,
		Type:        "code",
		ContentID:   &codeReq.AgentName,
	}) // Create session history for code agents
	if err != nil {
		return nil, fmt.Errorf("create code agent session: %w", err)
	}

	// Set session ID in the request
	codeReq.RequestID = sessionUUID
	data, _ := json.Marshal(codeReq)
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	ctx.Request.ContentLength = int64(len(data))

	slog.Debug("code agent session created", "sessionUUID", sessionUUID, "agentName", codeReq.AgentName)

	return ctx.Writer, nil
}
