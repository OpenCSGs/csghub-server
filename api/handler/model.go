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
	sc := component.NewSensitiveComponent(config)
	return &ModelHandler{
		c:  uc,
		sc: sc,
	}, nil
}

type ModelHandler struct {
	c  *component.ModelComponent
	sc component.SensitiveChecker
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
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models [get]
func (h *ModelHandler) Index(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	filter.Tags = parseTagReqs(ctx)
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filter = getFilterFromContext(ctx, filter)
	if !slices.Contains(Sorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	if filter.Source != "" && !slices.Contains[[]string](Sources, filter.Source) {
		msg := fmt.Sprintf("source parameter must be one of %v", Sources)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	models, total, err := h.c.Index(ctx, filter, per, page)
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
// @Param        current_user query string false "current user"
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

	_, err := h.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

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

	_, err := h.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
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

func parseTagReqs(ctx *gin.Context) (tags []types.TagReq) {
	licenseTag := ctx.Query("license_tag")
	taskTag := ctx.Query("task_tag")
	frameworkTag := ctx.Query("framework_tag")
	if licenseTag != "" {
		tags = append(tags, types.TagReq{
			Name:     strings.ToLower(licenseTag),
			Category: "license",
		})
	}

	if taskTag != "" {
		tags = append(tags, types.TagReq{
			Name:     strings.ToLower(taskTag),
			Category: "task",
		})
	}

	if frameworkTag != "" {
		tags = append(tags, types.TagReq{
			Name:     strings.ToLower(frameworkTag),
			Category: "framework",
		})
	}

	languageTag := ctx.Query("language_tag")
	if languageTag != "" {
		tags = append(tags, types.TagReq{
			Name:     strings.ToLower(languageTag),
			Category: "language",
		})
	}

	industryTag := ctx.Query("industry_tag")
	if industryTag != "" {
		tags = append(tags, types.TagReq{
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
func (h *ModelHandler) DeployDedicated(ctx *gin.Context) {
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
	allow, err := h.c.AllowReadAccess(ctx, types.ModelRepo, namespace, name, currentUser)
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

	if req.Revision == "" {
		req.Revision = "main" // default repo branch
	}

	if req.MinReplica < 0 || req.MaxReplica < 0 || req.MinReplica > req.MaxReplica {
		slog.Error("Bad request setting for replica", slog.Any("MinReplica", req.MinReplica), slog.Any("MaxReplica", req.MaxReplica))
		httpbase.BadRequest(ctx, "Bad request setting for replica")
		return
	}

	_, err = h.sc.CheckRequest(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	epReq := types.DeployActReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.InferenceType,
	}
	deployID, err := h.c.Deploy(ctx, epReq, req)
	if err != nil {
		slog.Error("failed to deploy model as inference", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("currentUser", currentUser), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("deploy model as inference created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))

	// return deploy_id
	response := types.DeployRepo{DeployID: deployID}

	httpbase.OK(ctx, response)
}

// FinetuneCreate      godoc
// @Security     ApiKey
// @Summary      create a finetune instance
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        body body types.InstanceRunReq true "deploy setting of instance"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/finetune [post]
func (h *ModelHandler) FinetuneCreate(ctx *gin.Context) {
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
		slog.Info("user is not allowed to run model", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.UnauthorizedError(ctx, errors.New("user not allowed to run model"))
		return
	}

	var req types.InstanceRunReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	modelReq := &types.ModelRunReq{
		DeployName:         req.DeployName,
		ClusterID:          req.ClusterID,
		ResourceID:         req.ResourceID,
		RuntimeFrameworkID: req.RuntimeFrameworkID,
		MinReplica:         1,
		MaxReplica:         1,
		SecureLevel:        2,
		Revision:           req.Revision,
	}

	ftReq := types.DeployActReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.FinetuneType,
	}

	deployID, err := h.c.Deploy(ctx, ftReq, *modelReq)
	if err != nil {
		slog.Error("failed to deploy model as notebook instance", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("deploy model as instance created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))

	// return deploy_id
	response := types.DeployRepo{DeployID: deployID}

	httpbase.OK(ctx, response)
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
	delReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.InferenceType,
	}
	err = h.c.DeleteDeploy(ctx, delReq)
	if err != nil {
		slog.Error("Failed to delete deploy", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// FinetuneDelete  godoc
// @Security     ApiKey
// @Summary      Delete a finetune instance
// @Description  delete a finetune instance
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
// @Router       /models/{namespace}/{name}/finetune/{id} [delete]
func (h *ModelHandler) FinetuneDelete(ctx *gin.Context) {
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

	delReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.FinetuneType,
	}
	err = h.c.DeleteDeploy(ctx, delReq)
	if err != nil {
		slog.Error("Failed to delete deploy", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// StopDeploy    godoc
// @Security     ApiKey
// @Summary      Stop a model inference
// @Description  Stop a model inference
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
// @Router       /models/{namespace}/{name}/run/{id}/stop [put]
func (h *ModelHandler) DeployStop(ctx *gin.Context) {
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
	stopReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.InferenceType,
	}
	err = h.c.DeployStop(ctx, stopReq)
	if err != nil {
		slog.Error("Failed to stop deploy", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// StartDeploy   godoc
// @Security     ApiKey
// @Summary      Start a model inference
// @Description  Start a model inference
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "deploy id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run/{id}/start [put]
func (h *ModelHandler) DeployStart(ctx *gin.Context) {
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

	startReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.InferenceType,
	}

	err = h.c.DeployStart(ctx, startReq)
	if err != nil {
		slog.Error("Failed to start deploy", slog.Any("error", err), slog.Any("repoType", types.ModelRepo), slog.String("namespace", namespace), slog.String("name", name), slog.Any("deployID", id))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetModelsByRuntime godoc
// @Security     ApiKey
// @Summary      Get Visible models by runtime framework for current user
// @Description  get visible models by runtime framework for current user
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param        current_user query string false "current user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/models [get]
func (h *ModelHandler) ListByRuntimeFrameworkID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	models, total, err := h.c.ListModelsByRuntimeFrameworkID(ctx, currentUser, per, page, id, deployType)
	if err != nil {
		slog.Error("Failed to get models", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  models,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// FinetuneStop    godoc
// @Security     ApiKey
// @Summary      Stop a finetune instance
// @Description  Stop a finetune instance
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
// @Router       /models/{namespace}/{name}/finetune/{id}/stop [put]
func (h *ModelHandler) FinetuneStop(ctx *gin.Context) {
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
	stopReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.FinetuneType,
	}
	err = h.c.DeployStop(ctx, stopReq)
	if err != nil {
		slog.Error("Failed to stop deploy", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// FinetuneStart   godoc
// @Security     ApiKey
// @Summary      Start a finetune instance
// @Description  Start a finetune instance
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "deploy id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/finetune/{id}/start [put]
func (h *ModelHandler) FinetuneStart(ctx *gin.Context) {
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
	startReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.FinetuneType,
	}
	err = h.c.DeployStart(ctx, startReq)
	if err != nil {
		slog.Error("Failed to start deploy", slog.Any("error", err), slog.Any("repoType", types.ModelRepo), slog.String("namespace", namespace), slog.String("name", name), slog.Any("deployID", id))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetRuntime godoc
// @Security     ApiKey
// @Summary      Get all runtime frameworks for current user
// @Description  get all runtime frameworks for current user
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework [get]
func (h *ModelHandler) ListAllRuntimeFramework(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	runtimes, err := h.c.ListAllByRuntimeFramework(ctx, currentUser)
	if err != nil {
		slog.Error("Failed to get runtime frameworks", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data": runtimes,
	}
	ctx.JSON(http.StatusOK, respData)
}

// UpdateModelRuntime godoc
// @Security     ApiKey
// @Summary      Set model runtime frameworks
// @Description  set model runtime frameworks
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Param        current_user query string false "current user"
// @Param        body body types.RuntimeFrameworkModels true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id} [post]
func (h *ModelHandler) UpdateModelRuntimeFrameworks(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	var req types.RuntimeFrameworkModels
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	slog.Info("update runtime frameworks models", slog.Any("req", req), slog.Any("runtime framework id", id), slog.Any("deployType", deployType))

	list, err := h.c.SetRuntimeFrameworkModes(ctx, currentUser, deployType, id, req.Models)
	if err != nil {
		slog.Error("Failed to set models runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, list)
}

// UpdateModelRuntime godoc
// @Security     ApiKey
// @Summary      Set model runtime frameworks
// @Description  set model runtime frameworks
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Param        current_user query string false "current user"
// @Param        body body types.RuntimeFrameworkModels true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id} [delete]
func (h *ModelHandler) DeleteModelRuntimeFrameworks(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	var req types.RuntimeFrameworkModels
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	slog.Info("update runtime frameworks models", slog.Any("req", req), slog.Any("runtime framework id", id), slog.Any("deployType", deployType))

	list, err := h.c.DeleteRuntimeFrameworkModes(ctx, currentUser, deployType, id, req.Models)
	if err != nil {
		slog.Error("Failed to set models runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, list)
}

// GetRuntimeFrameworkModels godoc
// @Security     ApiKey
// @Summary      Get Visible models for all runtime frameworks for current user
// @Description  get visible models for all runtime frameworks for current user
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Param        current_user query string false "current user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param     	 deploy_type query int false "deploy_type" Enums(1, 2) default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/models [get]
func (h *ModelHandler) ListModelsOfRuntimeFrameworks(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	filter = getFilterFromContext(ctx, filter)
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
		slog.Error("Bad request per and page format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	models, total, err := h.c.ListModelsOfRuntimeFrameworks(ctx, currentUser, filter.Search, filter.Sort, per, page, deployType)
	if err != nil {
		slog.Error("fail to get models for all runtime frameworks", slog.Any("deployType", deployType), slog.Any("per", per), slog.Any("page", page), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  models,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// ModelFiles      godoc
// @Security     ApiKey
// @Summary      Get all files of a model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/all_files [get]
func (h *ModelHandler) AllFiles(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req types.GetAllFilesReq
	req.Namespace = namespace
	req.Name = name
	req.RepoType = types.ModelRepo
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	detail, err := h.c.AllFiles(ctx, req)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get model all files", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

// ModelServerless  godoc
// @Security     ApiKey
// @Summary      run model as serverless service
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        body body types.ModelRunReq true "deploy setting of serverless"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless [post]
func (h *ModelHandler) DeployServerless(ctx *gin.Context) {
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

	var req types.ModelRunReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.Revision == "" {
		req.Revision = "main" // default repo branch
	}

	if req.MinReplica < 0 || req.MaxReplica < 0 || req.MinReplica > req.MaxReplica {
		slog.Error("Bad request setting for replica", slog.Any("MinReplica", req.MinReplica), slog.Any("MaxReplica", req.MaxReplica))
		httpbase.BadRequest(ctx, "Bad request setting for replica")
		return
	}

	deployReq := types.DeployActReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.ServerlessType,
	}

	req.SecureLevel = 1 // public for serverless
	deployID, err := h.c.Deploy(ctx, deployReq, req)
	if err != nil {
		slog.Error("failed to deploy model as serverless", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("currentUser", currentUser), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("deploy model as serverless created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))

	// return deploy_id
	response := types.DeployRepo{DeployID: deployID}

	httpbase.OK(ctx, response)
}

// StartServerless   godoc
// @Security     ApiKey
// @Summary      Start a model serverless
// @Description  Start a model serverless
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "deploy id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id}/start [put]
func (h *ModelHandler) ServerlessStart(ctx *gin.Context) {
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

	startReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.ServerlessType,
	}

	err = h.c.DeployStart(ctx, startReq)
	if err != nil {
		slog.Error("Failed to start deploy", slog.Any("error", err), slog.Any("repoType", types.ModelRepo), slog.String("namespace", namespace), slog.String("name", name), slog.Any("deployID", id))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// StopServerless    godoc
// @Security     ApiKey
// @Summary      Stop a model serverless
// @Description  Stop a model serverless
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
// @Router       /models/{namespace}/{name}/serverless/{id}/stop [put]
func (h *ModelHandler) ServerlessStop(ctx *gin.Context) {
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

	stopReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    id,
		DeployType:  types.ServerlessType,
	}

	err = h.c.DeployStop(ctx, stopReq)
	if err != nil {
		slog.Error("Failed to stop deploy", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetServerless godoc
// @Security     JWT token
// @Summary      get model serverless
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless [get]
func (h *ModelHandler) GetDeployServerless(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	response, err := h.c.GetServerless(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to get model serverless endpoint", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("currentUser", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}
