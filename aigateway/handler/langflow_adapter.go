package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type LangflowAdapter struct {
	cfg            *config.Config
	agentComponent component.AgentComponent
}

func NewLangflowAdapter(cfg *config.Config, agentComponent component.AgentComponent) *LangflowAdapter {
	return &LangflowAdapter{cfg: cfg, agentComponent: agentComponent}
}

func (a *LangflowAdapter) Name() string { return "langflow" }

func (a *LangflowAdapter) GetHost(ctx context.Context) (string, error) {
	return strings.TrimSuffix(a.cfg.Agent.AgentHubServiceHost, "/"), nil
}

func (a *LangflowAdapter) PrepareResponseWriter(ctx *gin.Context, api string, stream bool) (http.ResponseWriter, error) {
	q := ctx.Request.URL.Query()
	q.Set("token", a.cfg.Agent.AgentHubServiceToken)
	ctx.Request.URL.RawQuery = q.Encode()
	userUUID := httpbase.GetCurrentUserUUID(ctx)
	ctx.Request.Header.Set("user_uuid", userUUID)

	if !(ctx.Request.Method == http.MethodPost && strings.HasPrefix(api, "/api/v1/opencsg/run/")) {
		return ctx.Writer, nil
	}

	var chatReq types.AgentChatRequest
	if err := ctx.ShouldBindJSON(&chatReq); err != nil {
		return nil, fmt.Errorf("parse request body of run flow request: %w", err)
	}

	flowID := path.Base(api)
	slog.Debug("flowID", "flowID", flowID)

	// Create session for langflow agent
	sessionName := common.TruncString(chatReq.InputValue, 50)
	sessionUUID, err := a.agentComponent.CreateSession(ctx, userUUID, &types.CreateAgentInstanceSessionRequest{
		SessionUUID: chatReq.SessionID,
		Name:        &sessionName,
		Type:        "langflow",
		ContentID:   &flowID,
	}) // Create session history for langflow agents
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	chatReq.SessionID = &sessionUUID
	data, _ := json.Marshal(chatReq)
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	ctx.Request.ContentLength = int64(len(data))

	slog.Debug("session created", "sessionUUID", sessionUUID)

	return NewLangflowResponseWriterWrapper(ctx.Writer, stream, a.agentComponent), nil
}
