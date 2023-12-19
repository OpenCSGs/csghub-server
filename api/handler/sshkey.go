package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
	"opencsg.com/starhub-server/component"
)

func NewSSHKeyHandler(config *config.Config) (*SSHKeyHandler, error) {
	oc, err := component.NewSSHKeyComponent(config)
	if err != nil {
		return nil, err
	}
	return &SSHKeyHandler{
		c: oc,
	}, nil
}

type SSHKeyHandler struct {
	c *component.SSHKeyComponent
}

func (h *SSHKeyHandler) Create(ctx *gin.Context) {
	var req types.CreateSSHKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Username = ctx.Param("username")
	sk, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create SSH key", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Create SSH key succeed", slog.String("key_name", sk.Name))
	httpbase.OK(ctx, sk)
}

func (h *SSHKeyHandler) Index(ctx *gin.Context) {
	username := ctx.Param("username")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	sks, err := h.c.Index(ctx, username, per, page)
	if err != nil {
		slog.Error("Failed to create SSH key", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get User SSH keys succeed")
	httpbase.OK(ctx, sks)
}

func (h *SSHKeyHandler) Delete(ctx *gin.Context) {
	name := ctx.Param("name")
	username := ctx.Param("username")
	if name == "" || username == "" {
		err := fmt.Errorf("Invalid username or key name in url")
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	err := h.c.Delete(ctx, username, name)
	if err != nil {
		slog.Error("Failed to delete SSH key", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Delete SSH keys succeed")
	httpbase.OK(ctx, nil)
}
