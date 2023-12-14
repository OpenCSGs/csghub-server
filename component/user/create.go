package user

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/types"
)

func (c *Controller) Create(ctx *gin.Context) (*database.User, error) {
	var req types.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	slog.Debug("Creating user", slog.Any("req", req))
	respUser, err := c.gitServer.CreateUser(&req)
	if err != nil {
		slog.Error("Failed to create user in git server", slog.Any("req", req), slog.Any("error", err))
		return nil, fmt.Errorf("failed to create user in git server,cause:%w", err)
	}
	namespace := &database.Namespace{
		Path: respUser.Username,
	}
	err = c.userStore.Create(ctx, respUser, namespace)
	if err != nil {
		slog.Error("Failed to create user in database", slog.Any("respUser", respUser),
			slog.Any("namespace", namespace), slog.Any("error", err))
		return nil, fmt.Errorf("failed to create user in database,cause:%w", err)
	}

	return respUser, err
}
