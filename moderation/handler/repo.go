package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/component"
)

type RepoHandler struct {
	rc component.RepoComponent
}

func NewRepoHandler(config *config.Config) (*RepoHandler, error) {
	c, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, err
	}

	return &RepoHandler{
		rc: c,
	}, nil
}

func (h *RepoHandler) FullCheck(c *gin.Context) {
	type request struct {
		Namespace string               `json:"namespace"`
		Name      string               `json:"name"`
		RepoType  types.RepositoryType `json:"repo_type"`
	}

	var req request
	// binding request and check error
	if err := c.BindJSON(&req); err != nil {
		slog.Error("invalid request for full check", slog.Any("error", err))
		httpbase.BadRequest(c, err.Error())
		return
	}

	result, err := h.rc.RepoFullCheck(c.Request.Context(), component.RepoFullCheckRequest{
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  req.RepoType,
	})
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Failed to repo full check", slog.String("namespace", req.Namespace),
			slog.String("name", req.Name), "error", err)
		httpbase.ServerError(c, err)
		return
	}
	httpbase.OK(c, result)
}
