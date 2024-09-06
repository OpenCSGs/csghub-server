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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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

// GetLikesCollections godoc
// @Security     ApiKey
// @Summary      Get user likes collections
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Collection,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/collections [get]
func (h *UserHandler) LikesCollections(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.UserCollectionReq
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
	ms, total, err := h.c.LikesCollection(ctx, &req)
	if err != nil {
		slog.Error("Failed to get user collections", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  ms,
		"total": total,
	}

	ctx.JSON(http.StatusOK, respData)
}

// GetUserCollections godoc
// @Security     ApiKey
// @Summary      Get user's collections
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Collection,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/collections [get]
func (h *UserHandler) UserCollections(ctx *gin.Context) {
	var req types.UserCollectionReq
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
	ms, total, err := h.c.Collections(ctx, &req)
	if err != nil {
		slog.Error("Failed to get user collections", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  ms,
		"total": total,
	}

	ctx.JSON(http.StatusOK, respData)
}

// AddCollectionLikes  godoc
// @Security     ApiKey
// @Summary      Add collection likes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        id path string true "collection id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/collections/{id} [put]
func (h *UserHandler) LikeCollection(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.UserLikesRequest
	req.CurrentUser = currentUser
	collection_id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.Collection_id = collection_id
	err = h.c.LikeCollection(ctx, &req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteCollectionLikes  godoc
// @Security     ApiKey
// @Summary      delete collection likes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        id path string true "collection id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/collections/{id} [delete]
func (h *UserHandler) UnLikeCollection(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.UserLikesRequest
	req.CurrentUser = currentUser
	collection_id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.Collection_id = collection_id
	err = h.c.UnLikeCollection(ctx, &req)
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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

func (h *UserHandler) UserPermission(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	response := types.WhoamiResponse{
		Name: currentUser,
		Auth: types.Auth{
			AccessToken: types.AccessToken{
				DisplayName: currentUser,
				Role:        "write",
			},
			Type: "Bearer",
		},
	}
	ctx.JSON(http.StatusOK, response)
}

// GetUserRunDeploys godoc
// @Security     ApiKey
// @Summary      Get user running deploys
// @Description  Get user running deploys
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        repo_type path string true "model,space" Enums(model,space)
// @Param 		 deploy_type query int false "deploy type(0-space,1-inference,2-finetune)" Enums(0, 1, 2) default(1)
// @Param        per query int false "per" default(50)
// @Param        page query int false "page index" default(1)
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/run/{repo_type} [get]
func (h *UserHandler) GetRunDeploys(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	username := ctx.Param("username")
	if currentUser != username {
		slog.Warn("invalid user to list deploys", slog.String("currentUser", currentUser), slog.String("username", username))
		httpbase.ServerError(ctx, errors.New("invalid user"))
		return
	}

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.Error("Bad request deploy type format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromParam(ctx)
	if repoType != types.ModelRepo && repoType != types.SpaceRepo {
		slog.Error("Invalid repo type", slog.Any("repo_type", repoType))
		httpbase.BadRequest(ctx, "Invalid repo type")
		return
	}

	var req types.DeployReq
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	req.RepoType = repoType
	req.DeployType = deployType
	ds, total, err := h.c.ListDeploys(ctx, repoType, &req)
	if err != nil {
		slog.Error("Failed to get deploy repo list", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"message": "OK",
		"data":    ds,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetFinetuneInstances godoc
// @Security     ApiKey
// @Summary      Get user running notebook instances
// @Description  Get user running notebook instances
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        per query int false "per" default(50)
// @Param        page query int false "page index" default(1)
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/finetune/instances [get]
func (h *UserHandler) GetFinetuneInstances(ctx *gin.Context) {
	respData := gin.H{
		"message": "OK",
		"data":    nil,
		"total":   0,
	}

	var req types.UserRepoReq
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		ctx.JSON(http.StatusOK, respData)
		return
	}

	username := ctx.Param("username")
	if currentUser != username {
		slog.Warn("invalid user to list deploys", slog.String("currentUser", currentUser), slog.String("username", username))
		ctx.JSON(http.StatusOK, respData)
		return
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ds, total, err := h.c.ListInstances(ctx, &req)
	if err != nil {
		slog.Error("Failed to get instance list", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	respData = gin.H{
		"message": "OK",
		"data":    ds,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetServerless godoc
// @Security     ApiKey
// @Summary      Get serverless deploys
// @Description  Get serverless deploys
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        per query int false "per" default(50)
// @Param        page query int false "page index" default(1)
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/run/serverless [get]
func (h *UserHandler) GetRunServerless(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	username := ctx.Param("username")
	if currentUser != username {
		slog.Warn("invalid user to list serverless", slog.String("currentUser", currentUser), slog.String("username", username))
		httpbase.ServerError(ctx, errors.New("invalid user"))
		return
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.DeployReq
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	req.RepoType = types.ModelRepo
	req.DeployType = types.ServerlessType
	ds, total, err := h.c.ListServerless(ctx, req)
	if err != nil {
		slog.Error("Failed to get serverless list", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"message": "OK",
		"data":    ds,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}
