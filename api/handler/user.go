package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"opencsg.com/csghub-server/common/errorx"

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
		user: uc,
	}, nil
}

type UserHandler struct {
	user component.UserComponent
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
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.Datasets(ctx.Request.Context(), &req)
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
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.Models(ctx.Request.Context(), &req)
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
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.Codes(ctx.Request.Context(), &req)
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
// @Param        sdk query string false "filter by space sdk"
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
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.SDK = ctx.Query("sdk")
	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.Spaces(ctx.Request.Context(), &req)
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
	var req types.UserLikesRequest
	req.Username = ctx.Param("username")
	req.CurrentUser = currentUser
	repo_id, err := strconv.ParseInt(ctx.Param("repo_id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.RepoID = repo_id
	err = h.user.AddLikes(ctx.Request.Context(), &req)
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
	var req types.UserCollectionReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.LikesCollection(ctx.Request.Context(), &req)
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
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.Collections(ctx.Request.Context(), &req)
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
	var req types.UserLikesRequest
	req.CurrentUser = currentUser
	collectionID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.CollectionID = collectionID
	err = h.user.LikeCollection(ctx.Request.Context(), &req)
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
	var req types.UserLikesRequest
	req.CurrentUser = currentUser
	collection_id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.CollectionID = collection_id
	err = h.user.UnLikeCollection(ctx.Request.Context(), &req)
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
	var req types.UserLikesRequest
	req.Username = ctx.Param("username")
	req.CurrentUser = currentUser
	repo_id, err := strconv.ParseInt(ctx.Param("repo_id"), 10, 64)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	req.RepoID = repo_id
	// slog.Info("user.likes.delete.req=%v", req)
	err = h.user.DeleteLikes(ctx.Request.Context(), &req)
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
	var req types.UserCollectionReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per

	ms, total, err := h.user.LikesSpaces(ctx.Request.Context(), &req)
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
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.LikesCodes(ctx.Request.Context(), &req)
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
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ms, total, err := h.user.LikesModels(ctx.Request.Context(), &req)
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
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.LikesDatasets(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get user datasets", slog.Any("error", err))
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
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.DeployRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/run/{repo_type} [get]
func (h *UserHandler) GetRunDeploys(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.Error("Bad request deploy type format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	repoType := common.RepoTypeFromParam(ctx)
	if repoType != types.ModelRepo && repoType != types.SpaceRepo {
		slog.Error("Invalid repo type", slog.Any("repo_type", repoType))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("Invalid repo type"), nil))
		return
	}

	var req types.DeployReq
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	req.RepoType = repoType
	req.DeployType = deployType
	ds, total, err := h.user.ListDeploys(ctx.Request.Context(), repoType, &req)
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
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.DeployRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/finetune/instances [get]
func (h *UserHandler) GetFinetuneInstances(ctx *gin.Context) {
	var req types.UserRepoReq
	currentUser := httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.ListInstances(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get instance list", slog.Any("error", err), slog.Any("req", req))
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
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.DeployRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/run/serverless [get]
func (h *UserHandler) GetRunServerless(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var req types.DeployReq
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	req.RepoType = types.ModelRepo
	req.DeployType = types.ServerlessType
	ds, total, err := h.user.ListServerless(ctx.Request.Context(), req)
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

// CreateUserResource godoc
// @Security     ApiKey
// @Summary      create order for user's resource
// @Description  create order for user's resource
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        body body types.CreateUserResourceReq true "create order request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/order/resource [post]
func (h *UserHandler) CreateUserResource(ctx *gin.Context) {
	var req types.CreateUserResourceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	for i := range req.OrderDetails {
		req.OrderDetails[i].SkuType = types.SKUCSGHub
	}
	req.Username = ctx.Param("username")
	err := h.user.CreateUserResource(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to create order for user's resource", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteUserResource godoc
// @Security     ApiKey
// @Summary      delete user's resource by order detail id
// @Description  delete user's resource by order detail id
// @Tags         User
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/order/resources/{id} [delete]
func (h *UserHandler) DeleteUserResource(ctx *gin.Context) {
	username := ctx.Param("username")
	orderDetailIdstr := ctx.Param("id")
	orderDetailId, err := strconv.ParseInt(orderDetailIdstr, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	err = h.user.DeleteUserResource(ctx.Request.Context(), username, orderDetailId)
	if err != nil {
		slog.Error("failed to create order for user's resource", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetUserResource godoc
// @Security     ApiKey
// @Summary      get user's resource
// @Description  get user's resource
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.UserResourcesResp,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/order/resources [get]
func (h *UserHandler) GetUserResource(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	username := ctx.Param("username")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var req types.GetUserResourceReq
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.GetUserResource(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to get user's resource", slog.Any("error", err), slog.Any("username", username))
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

// GetUserPrompts godoc
// @Security     ApiKey
// @Summary      Get user prompts
// @Description  get user prompts
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.DeployRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/prompts [get]
func (h *UserHandler) Prompts(ctx *gin.Context) {
	var req types.UserPromptsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.Prompts(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get user prompts", slog.Any("error", err))
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

// GetUserEvaluations godoc
// @Security     ApiKey
// @Summary      Get user evaluations
// @Description  get user evaluations
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.DeployRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/evaluations [get]
func (h *UserHandler) GetEvaluations(ctx *gin.Context) {
	var req types.UserEvaluationReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.Evaluations(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get user evaluations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  ds,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetUserMCPs   godoc
// @Security     ApiKey
// @Summary      Get user mcp servers
// @Description  Get user mcp servers
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.MCPServer,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/mcps [get]
func (h *UserHandler) MCPServers(ctx *gin.Context) {
	var req types.UserMCPsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	mcps, total, err := h.user.MCPServers(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get user mcp servers", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  mcps,
		"total": total,
	}

	httpbase.OK(ctx, respData)
}

// GetUserLikesMCPServers godoc
// @Security     ApiKey
// @Summary      Get user likes mcp servers
// @Description  get user likes mcp servers
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.MCPServer,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/likes/mcps [get]
func (h *UserHandler) LikesMCPServers(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.UserMCPsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	data, total, err := h.user.LikesMCPServers(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get user likes mcp servers", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  data,
		"total": total,
	}

	httpbase.OK(ctx, respData)
}

// GetUserNotebooks godoc
// @Summary      get user's notebooks
// @Description  get user's notebooks
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        page query int false "page number"
// @Param        size query int false "page size"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.NotebookRes,total=int} "OK"
// @Success      200  {array}  types.ResponseWithTotal{data=[]types.NotebookRes,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/notebooks [get]
func (h *UserHandler) GetUserNotebooks(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.DeployReq
	req.CurrentUser = currentUser
	req.Page = page
	req.PageSize = per
	req.DeployType = types.NotebookType
	notebooks, total, err := h.user.ListNotebooks(ctx.Request.Context(), &req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  notebooks,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}

// GetUserFintunes godoc
// @Security     ApiKey
// @Summary      Get user backend fintunes
// @Description  get user backend fintunes
// @Tags         User
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.ArgoWorkFlowRes,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/finetune/jobs [get]
func (h *UserHandler) GetUserFinetunes(ctx *gin.Context) {
	var req types.UserEvaluationReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Page = page
	req.PageSize = per
	ds, total, err := h.user.ListFinetunes(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get user backend finetunes", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  ds,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}
