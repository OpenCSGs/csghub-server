package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewModelHandler(config *config.Config) (*ModelHandler, error) {
	uc, err := component.NewModelComponent(config)
	if err != nil {
		return nil, err
	}
	return &ModelHandler{
		c: uc,
	}, nil
}

type ModelHandler struct {
	c *component.ModelComponent
}

// GetVisiableModels godoc
// @Security     ApiKey
// @Summary      Get Visiable models for current user
// @Description  get visiable models for current user
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        sort query string false "sort by"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models [get]
func (h *ModelHandler) Index(ctx *gin.Context) {
	tagReqs := parseTagReqs(ctx)
	username := httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	search, sort := getFilterFromContext(ctx)
	if !slices.Contains(Sorts, sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}
	models, total, err := h.c.Index(ctx, username, search, sort, tagReqs, per, page)
	if err != nil {
		slog.Error("Failed to get models", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public models succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  models,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// CreateModel   godoc
// @Security     ApiKey
// @Summary      Create a new model
// @Description  create a new model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        body body types.CreateModelReq true "body"
// @Success      200  {object}  types.Response{data=database.Model} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models [post]
func (h *ModelHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.CreateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser

	model, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create model", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create model succeed", slog.String("model", model.Name))
	httpbase.OK(ctx, model)
}

// UpdateModel   godoc
// @Security     ApiKey
// @Summary      Update a exists model
// @Description  update a exists model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the model owner"
// @Param        body body types.UpdateModelReq true "body"
// @Success      200  {object}  types.Response{data=database.Model} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name} [put]
func (h *ModelHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.UpdateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.Username = currentUser

	model, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update model", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update model succeed", slog.String("model", model.Name))
	httpbase.OK(ctx, model)
}

// DeleteModel   godoc
// @Security     ApiKey
// @Summary      Delete a exists model
// @Description  delete a exists model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the model owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name} [delete]
func (h *ModelHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.c.Delete(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to delete model", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete model succeed", slog.String("model", name))
	httpbase.OK(ctx, nil)
}

// GetModel      godoc
// @Security     ApiKey
// @Summary      Get model detail
// @Description  get model detail
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{data=types.Model} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name} [get]
func (h *ModelHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.c.Show(ctx, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get model detail", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get model succeed", slog.String("model", name))
	httpbase.OK(ctx, detail)
}

func (h *ModelHandler) SDKModelInfo(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ref := ctx.Param("ref")
	currentUser := httpbase.GetCurrentUser(ctx)
	modelInfo, err := h.c.SDKModelInfo(ctx, namespace, name, ref, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get sdk model info", slog.String("namespace", namespace), slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, modelInfo)
}

// ModelRelations      godoc
// @Security     ApiKey
// @Summary      Get model related assets
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.Relations} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/relations [get]
func (h *ModelHandler) Relations(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.c.Relations(ctx, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get model relations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

// Predict godoc
// @Security     ApiKey
// @Summary      Invoke model prediction
// @Description  invoke model prediction
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        body body types.ModelPredictReq true "input for model prediction"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/predict [post]
func (h *ModelHandler) Predict(ctx *gin.Context) {
	var req types.ModelPredictReq
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Name = name
	req.Namespace = namespace

	resp, err := h.c.Predict(ctx, &req)
	if err != nil {
		slog.Error("fail to call predict", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

func parseTagReqs(ctx *gin.Context) (tags []database.TagReq) {
	licenseTag := ctx.Query("license_tag")
	taskTag := ctx.Query("task_tag")
	frameworkTag := ctx.Query("framework_tag")
	if licenseTag != "" {
		tags = append(tags, database.TagReq{
			Name:     strings.ToLower(licenseTag),
			Category: "license",
		})
	}

	if taskTag != "" {
		tags = append(tags, database.TagReq{
			Name:     strings.ToLower(taskTag),
			Category: "task",
		})
	}

	if frameworkTag != "" {
		tags = append(tags, database.TagReq{
			Name:     strings.ToLower(frameworkTag),
			Category: "framework",
		})
	}

	languageTag := ctx.Query("language_tag")
	if languageTag != "" {
		tags = append(tags, database.TagReq{
			Name:     strings.ToLower(languageTag),
			Category: "language",
		})
	}

	industryTag := ctx.Query("industry_tag")
	if industryTag != "" {
		tags = append(tags, database.TagReq{
			Name:     strings.ToLower(industryTag),
			Category: "industry",
		})
	}
	return
}

func convertFilePathFromRoute(path string) string {
	return strings.TrimLeft(path, "/")
}

// ModelRun      godoc
// @Security     ApiKey
// @Summary      run model as inference
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        body body types.ModelRunReq true "deploy setting of inference"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run [post]
func (h *ModelHandler) DeployRun(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, err := h.c.AllowAdminAccess(ctx, types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}
	if !allow {
		slog.Info("user not allowed to run model", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.UnauthorizedError(ctx, errors.New("user not allowed to run model"))
		return
	}

	var req types.ModelRunReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica < 0 || req.MaxReplica < 0 || req.MinReplica > req.MaxReplica {
		slog.Error("Bad request setting for replica", slog.Any("MinReplica", req.MinReplica), slog.Any("MaxReplica", req.MaxReplica))
		httpbase.BadRequest(ctx, "Bad request setting for replica")
		return
	}

	deployID, err := h.c.Deploy(ctx, namespace, name, currentUser, req)
	if err != nil {
		slog.Error("failed to deploy model as inference", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("failed to deploy space"))
		return
	}

	slog.Debug("deploy model as inference created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))
	httpbase.OK(ctx, nil)
}

// DeleteDeploy  godoc
// @Security     ApiKey
// @Summary      Delete a model inference
// @Description  delete a model inference
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run/{id} [delete]
func (h *ModelHandler) DeployDelete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.c.DeleteDeploy(ctx, types.ModelRepo, namespace, name, currentUser, id)
	if err != nil {
		slog.Error("Failed to delete deploy", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
