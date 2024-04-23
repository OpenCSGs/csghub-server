package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewUserHandler(config *config.Config) (*UserHandler, error) {
	uc, err := component.NewUserComponent(config)
	if err != nil {
		return nil, err
	}
	return &UserHandler{
		c: uc,
	}, nil
}

type UserHandler struct {
	c *component.UserComponent
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
func (h *UserHandler) Create(ctx *gin.Context) {
	var req *types.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	slog.Debug("Creating user", slog.Any("req", req))
	user, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Create user succeed", slog.String("user", user.Username))
	httpbase.OK(ctx, user)
}

// UpdateUser godoc
// @Security     ApiKey
// @Summary      Update user
// @Description  update user
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        body   body  types.UpdateUserRequest true "body"
// @Success      200  {object}  types.Response{data=database.User} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /users/{username} [put]
func (h *UserHandler) Update(ctx *gin.Context) {
	var req *types.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	user, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update user", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update user succeed", slog.String("user", user.Username))
	httpbase.OK(ctx, user)
}

// GetUserDatasets godoc
// @Security     ApiKey
// @Summary      Get user datasets
// @Description  get user datasets
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Dataset,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/datasets [get]
func (h *UserHandler) Datasets(ctx *gin.Context) {
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ds, total, err := h.c.Datasets(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user datasets", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user datasets succeed", slog.String("user", req.Owner))
	respData := gin.H{
		"message": "OK",
		"data":    ds,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetUserModels godoc
// @Security     ApiKey
// @Summary      Get user models
// @Description  get user models
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/models [get]
func (h *UserHandler) Models(ctx *gin.Context) {
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.c.Models(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user models", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user models succeed", slog.String("user", req.Owner))

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetUserCodes godoc
// @Security     ApiKey
// @Summary      Get user codes
// @Description  get user codes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Code,total=int} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Router       /user/{username}/codes [get]
func (h *UserHandler) Codes(ctx *gin.Context) {
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.c.Codes(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user codes", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user codes succeed", slog.String("user", req.Owner))

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetUserSpaces godoc
// @Security     ApiKey
// @Summary      Get user spaces
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Space,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/spaces [get]
func (h *UserHandler) Spaces(ctx *gin.Context) {
	var req types.UserSpacesReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.c.Spaces(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}

	ctx.JSON(http.StatusOK, respData)
}

// AddUserLikes  godoc
// @Security     ApiKey
// @Summary      Add user likes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        repo_id path string true "repo id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/{repoid} [put]
func (h *UserHandler) LikesAdd(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.UserLikesRequest
	req.Username = ctx.Param("username")
	req.CurrentUser = currentUser
	repo_id, err := strconv.ParseInt(ctx.Param("repo_id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.Repo_id = repo_id
	err = h.c.AddLikes(ctx, &req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteUserlikes  godoc
// @Security     ApiKey
// @Summary      Delete user likes
// @Description  Delete user likes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        repo_id path string true "repo id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/{repoid} [delete]
func (h *UserHandler) LikesDelete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.UserLikesRequest
	req.Username = ctx.Param("username")
	req.CurrentUser = currentUser
	repo_id, err := strconv.ParseInt(ctx.Param("repo_id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.Repo_id = repo_id
	// slog.Info("user.likes.delete.req=%v", req)
	err = h.c.DeleteLikes(ctx, &req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetUserLikesSpaces  godoc
// @Security     ApiKey
// @Summary      Get user likes spaces
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Space,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/spaces [get]
func (h *UserHandler) LikesSpaces(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.UserSpacesReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per

	ms, total, err := h.c.LikesSpaces(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}

	ctx.JSON(http.StatusOK, respData)
}

// GetUserLikesCodes godoc
// @Security     ApiKey
// @Summary      Get user likes codes
// @Description  get user likes codes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Code,total=int} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Router       /user/{username}/likes/codes [get]
func (h *UserHandler) LikesCodes(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ms, total, err := h.c.LikesCodes(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user codes", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user likes codes succeed", slog.String("user", req.Owner))

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetUserLikesModels godoc
// @Security     ApiKey
// @Summary      Get user likes models
// @Description  get user likes models
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/models [get]
func (h *UserHandler) LikesModels(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ms, total, err := h.c.LikesModels(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user models", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user models succeed", slog.String("user", req.Owner))

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetUserLikesDatasets godoc
// @Security     ApiKey
// @Summary      Get user likes datasets
// @Description  get user likes datasets
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Dataset,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/datasets [get]
func (h *UserHandler) LikesDatasets(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ds, total, err := h.c.LikesDatasets(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user datasets", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get user datasets succeed", slog.String("user", req.Owner))
	respData := gin.H{
		"message": "OK",
		"data":    ds,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}
