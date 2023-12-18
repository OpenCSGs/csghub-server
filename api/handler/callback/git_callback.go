package callback

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	component "opencsg.com/starhub-server/component/callback"
)

type GitCallbackHandler struct {
	cbc *component.GitCallbackComponent
}

func NewGitCallbackHandler(config *config.Config) (*GitCallbackHandler, error) {
	cbc, err := component.NewGitCallback(config)

	if err != nil {
		return nil, err
	}
	return &GitCallbackHandler{cbc}, nil
}

func (h *GitCallbackHandler) Handle(c *gin.Context) {
	event := c.Request.Header.Get("X-Gitea-Event")
	switch event {
	case "push":
		h.handlePush(c)
	default:
		slog.Error("Unknown git callback event", "event", event)
		httpbase.BadRequest(c, "unknown git callback event:"+event)
	}

}

func (h *GitCallbackHandler) handlePush(c *gin.Context) {
	var req types.GiteaCallbackPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad gitea callback request format", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	err := h.cbc.HandlePush(c.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to handle git push callback", slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}
	slog.Info("handle git callback successfully", slog.String("event", "push"))
	httpbase.OK(c, nil)
}
