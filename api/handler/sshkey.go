package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewSSHKeyHandler(config *config.Config) (*SSHKeyHandler, error) {
	oc, err := component.NewSSHKeyComponent(config)
	if err != nil {
		return nil, err
	}
	return &SSHKeyHandler{
		c:  oc,
		sc: component.NewSensitiveComponent(config),
	}, nil
}

type SSHKeyHandler struct {
	c  *component.SSHKeyComponent
	sc component.SensitiveChecker
}

// CreateUserSSHKey godoc
// @Security     ApiKey
// @Summary      Create a new SSH key for the given user
// @Description  create a new SSH key for the given user
// @Tags         SSH Key
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @param        body body types.CreateSSHKeyRequest true "body"
// @Success      200  {object}  types.Response{data=database.SSHKey} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/ssh_keys [post]
func (h *SSHKeyHandler) Create(ctx *gin.Context) {
	var req types.CreateSSHKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := h.sc.CheckRequest(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	req.Username = currentUser

	req.Username = ctx.Param("username")
	sk, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create SSH key", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Create SSH key succeed", slog.String("key_name", sk.Name))
	httpbase.OK(ctx, sk)
}

// GetUserSSHKeys godoc
// @Security     ApiKey
// @Summary      Get all SSH keys for the given user
// @Description  get all SSH keys for the given user
// @Tags         SSH Key
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.SSHKey,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/ssh_keys [get]
func (h *SSHKeyHandler) Index(ctx *gin.Context) {
	username := ctx.Param("username")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	sks, err := h.c.Index(ctx, username, per, page)
	if err != nil {
		slog.Error("Failed to create SSH key", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get User SSH keys succeed")
	httpbase.OK(ctx, sks)
}

// DeleteUserSSHKey godoc
// @Security     ApiKey
// @Summary      Delete specific SSH key for the given user
// @Description  delete specific SSH key for the given user
// @Tags         SSH Key
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        name path string true "key name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/ssh_key/{name} [delete]
func (h *SSHKeyHandler) Delete(ctx *gin.Context) {
	name := ctx.Param("name")
	username := ctx.Param("username")
	if name == "" || username == "" {
		err := fmt.Errorf("invalid username or key name in url")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err := h.c.Delete(ctx, username, name)
	if err != nil {
		slog.Error("Failed to delete SSH key", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Delete SSH keys succeed")
	httpbase.OK(ctx, nil)
}
