package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	apicomponent "opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/user/component"
)

type UserHandler struct {
	c                        *component.UserComponent
	sc                       apicomponent.SensitiveChecker
	publicDomain             string
	EnableHTTPS              bool
	signinSuccessRedirectURL string
}

func NewUserHandler(config *config.Config) (*UserHandler, error) {
	h := &UserHandler{}
	var err error
	h.c, err = component.NewUserComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create user component: %w", err)
	}
	h.sc = apicomponent.NewSensitiveComponent(config)
	domainParsedUrl, err := url.Parse(config.APIServer.PublicDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public domain '%s': %w", config.APIServer.PublicDomain, err)
	}
	h.publicDomain = domainParsedUrl.Hostname()
	h.EnableHTTPS = config.EnableHTTPS
	h.signinSuccessRedirectURL = config.User.SigninSuccessRedirectURL
	return h, err
}

// CreateUser godoc
// @Security     ApiKey
// @Summary      Create a new user
// @Description  create a new user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        body   body  types.CreateUserRequest true "body"
// @Success      200  {object}  types.Response{data=database.User} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /users [post]
// func (h *UserHandler) Create(ctx *gin.Context) {
// 	var req *types.CreateUserRequest
// 	if err := ctx.ShouldBindJSON(&req); err != nil {
// 		slog.Error("Bad request format", "error", err)
// 		httpbase.BadRequest(ctx, err.Error())
// 		return
// 	}

// 	slog.Debug("Creating user", slog.Any("req", req))
// 	user, err := h.c.Create(ctx, req)
// 	if err != nil {
// 		slog.Error("Failed to create user", slog.Any("error", err))
// 		httpbase.ServerError(ctx, err)
// 		return
// 	}

// 	slog.Info("Create user succeed", slog.String("user", user.Username))
// 	httpbase.OK(ctx, user)
// }

// UpdateUser godoc
// @Security     ApiKey
// @Summary      Update user. If change user name, should only send 'new_username' in the request body.
// @Description  update user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user  query  string true "current user"
// @Param        body   body  types.UpdateUserRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username} [put]
func (h *UserHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var err error
	_, err = h.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	userName := ctx.Param("username")
	req.Username = userName

	if req.NewUserName != nil {
		err = h.c.ChangeUserName(ctx, req.Username, *req.NewUserName, currentUser)
	} else {
		err = h.c.Update(ctx, req, currentUser)
	}
	if err != nil {
		slog.Error("Failed to update user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update user succeed", slog.String("user", req.Username))
	httpbase.OK(ctx, nil)
}

// DeleteUser godoc
// @Security     ApiKey
// @Summary      Delete user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user  query  string true "current user"
// @Param        body   body  types.UpdateUserRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username} [delete]
func (h *UserHandler) Delete(ctx *gin.Context) {
	userName := ctx.Param("username")
	err := h.c.Delete(ctx, userName)
	if err != nil {
		slog.Error("Failed to update user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update user succeed", slog.String("userName", userName))
	httpbase.OK(ctx, nil)
}

// GetUser godoc
// @Security     ApiKey
// @Summary      Get user info. Admin and the user self can see full info, other users can only see basic info.
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user  query  string true "current user"
// @Success      200  {object}  types.Response{data=types.User} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username} [get]
func (h *UserHandler) Get(ctx *gin.Context) {
	visitorName := httpbase.GetCurrentUser(ctx)
	userName := ctx.Param("username")
	user, err := h.c.Get(ctx, userName, visitorName)
	if err != nil {
		slog.Error("Failed to get user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user succeed", slog.String("userName", userName))
	httpbase.OK(ctx, user)
}

func (h *UserHandler) Casdoor(ctx *gin.Context) {
	code := ctx.Query("code")
	state := ctx.Query("state")
	slog.Debug("get casdoor callback", slog.String("code", code), slog.String("state", state))

	jwtToken, signed, err := h.c.Signin(ctx.Request.Context(), code, state)
	if err != nil {
		slog.Error("Failed to signin", slog.Any("error", err), slog.String("code", code), slog.String("state", state))
		httpbase.ServerError(ctx, fmt.Errorf("failed to signin: %w", err))
		return
	}
	expire := jwtToken.ExpiresAt
	targetUrl := fmt.Sprintf("%s?jwt=%s&expire=%d", h.signinSuccessRedirectURL, signed, expire.Unix())
	ctx.Redirect(http.StatusMovedPermanently, targetUrl)
}
