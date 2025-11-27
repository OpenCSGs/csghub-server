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

func (a *LangflowAdapter) PrepareProxyContext(ctx *gin.Context, api string) error {
	q := ctx.Request.URL.Query()
	q.Set("token", a.cfg.Agent.AgentHubServiceToken)
	ctx.Request.URL.RawQuery = q.Encode()
	userUUID := httpbase.GetCurrentUserUUID(ctx)
	ctx.Request.Header.Set("user_uuid", userUUID)

	if ctx.Request.Method != http.MethodPost || !strings.HasPrefix(api, "/api/v1/opencsg/run/") {
		return nil
	}

	stream := ctx.Query("stream") == "true"
	if stream {
		// ctx.Writer.Header().Set("Content-Type", "text/event-stream") // the response content type is already set by the langflow server, but miss no-cache and keep-alive headers
		ctx.Writer.Header().Set("Cache-Control", "no-cache")
		ctx.Writer.Header().Set("Connection", "keep-alive")
	}

	flowID := path.Base(api)
	slog.Debug("langflow adapter preparing proxy context", "api", api, "stream", stream, "flow_id", flowID, "user_uuid", userUUID)

	var chatReq types.LangflowChatRequest
	if err := ctx.ShouldBindJSON(&chatReq); err != nil {
		slog.Error("failed to parse request body of run flow request", "api", api, "user_uuid", userUUID, "error", err)
		return fmt.Errorf("parse request body of run flow request: %w", err)
	}

	// Create session for langflow agent
	sessionName := common.TruncStringByRune(chatReq.InputValue, 255)
	sessionUUID, err := a.agentComponent.CreateSession(ctx, userUUID, &types.CreateAgentInstanceSessionRequest{
		SessionUUID: chatReq.SessionID,
		Name:        &sessionName,
		Type:        "langflow",
		ContentID:   &flowID,
	})
	if err != nil {
		slog.Error("failed to create langflow agent session", "agent_type", "langflow", "api", api, "user_uuid", userUUID, "flow_id", flowID, "error", err)
		return fmt.Errorf("create langflow agent session: %w", err)
	}

	chatReq.SessionID = &sessionUUID
	data, _ := json.Marshal(chatReq)
	ctx.Request.Body = io.NopCloser(bytes.NewReader(data))
	ctx.Request.ContentLength = int64(len(data))

	return nil
}
