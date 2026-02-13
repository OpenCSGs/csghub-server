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
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// GetVisiableModels godoc
// @Security     ApiKey
// @Summary      Get Visiable models for current user
// @Description  get visiable models for current user
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag, deprecated"
// @Param        framework_tag query string false "filter by framework tag, deprecated"
// @Param        license_tag query string false "filter by license tag, deprecated"
// @Param        language_tag query string false "filter by language tag, deprecated"
// @Param        tag_category query string false "filter by tag category"
// @Param        tag_name query string false "filter by tag name"
// @Param        tag_group query string false "filter by tag group"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        xnet_migration_status query string false "filter by xnet migration status" Enums(pending, running, completed, failed)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param        model_tree query string false "example: base_model:finetune:1"
// @Param        list_serverless query bool false "list serverless" default(false)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models [get]
func (h *ModelHandler) Index(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	filter.Tags = parseTagReqs(ctx)
	tree, err := parseTreeReqs(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	filter.Tree = tree
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	filter = getFilterFromContext(ctx, filter)
	if !slices.Contains(types.Sorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", types.Sorts)
		err := errorx.ReqParamInvalid(errors.New(msg),
			errorx.Ctx().
				Set("param", "sort").
				Set("provided", filter.Sort).
				Set("allowed", types.Sorts))
		slog.ErrorContext(ctx.Request.Context(), "Bad request format,", slog.String("error", msg))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	if filter.Source != "" && !slices.Contains(types.Sources, filter.Source) {
		msg := fmt.Sprintf("source parameter must be one of %v", types.Sources)
		err := errorx.ReqParamInvalid(errors.New(msg),
			errorx.Ctx().
				Set("param", "source").
				Set("provided", filter.Source).
				Set("allowed", types.Sources))
		slog.ErrorContext(ctx.Request.Context(), "Bad request format,", slog.String("error", msg))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}
	listServerlessQ := ctx.Query("list_serverless")
	if listServerlessQ != "" {
		listServerless, _ := strconv.ParseBool(listServerlessQ)
		filter.ListServerless = listServerless
	}
	models, total, err := h.model.Index(ctx.Request.Context(), filter, per, page, needOpWeight)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get models", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public models succeed", slog.Int("count", total))
	httpbase.OKWithTotal(ctx, models, total)
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
	var req *types.CreateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	req.Username = currentUser

	if req.Namespace == "" {
		req.Namespace = currentUser
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}

	model, err := h.model.Create(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "Failed to create model", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
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
	var req *types.UpdateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.Username = currentUser

	model, err := h.model.Update(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update model", slog.Any("error", err))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	err = h.model.Delete(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete model", slog.Any("error", err))
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
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success      200  {object}  types.Response{data=types.Model} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name} [get]
func (h *ModelHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}
	qNeedMultiSync := ctx.Query("need_multi_sync")
	needMultiSync, err := strconv.ParseBool(qNeedMultiSync)
	if err != nil {
		needMultiSync = false
	}
	detail, err := h.model.Show(ctx.Request.Context(), namespace, name, currentUser, needOpWeight, needMultiSync)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get model detail", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get model succeed", slog.String("model", name))
	httpbase.OK(ctx, detail)
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.model.Relations(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get model relations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

// SetRelation   godoc
// @Security     ApiKey
// @Summary      Set dataset relation for model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        req body types.RelationDatasets true  "set dataset relation"
// @Success      200  {object}  types.Response{data=types.Relations} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/relations [put]
func (h *ModelHandler) SetRelations(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var req types.RelationDatasets
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.model.SetRelationDatasets(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to set datasets for model", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// AddDatasetRelation   godoc
// @Security     ApiKey
// @Summary      add dataset relation for model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        req body types.RelationDataset true  "add dataset relation"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/relations/dataset [post]
func (h *ModelHandler) AddDatasetRelation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var req types.RelationDataset
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.model.AddRelationDataset(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to add dataset for model", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteDatasetRelation  godoc
// @Security     ApiKey
// @Summary      delete dataset relation for model
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        req body types.RelationDataset true  "delelet dataset relation"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/relations/dataset [delete]
func (h *ModelHandler) DelDatasetRelation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var req types.RelationDataset
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.model.DelRelationDataset(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete dataset for model", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// model_tree: base_model:finetune:1
func parseTreeReqs(ctx *gin.Context) (*types.TreeReq, error) {
	modelTreeQuery := ctx.Query("model_tree")
	if modelTreeQuery == "" {
		return nil, nil
	}
	modelTreeQuerys := strings.Split(modelTreeQuery, ":")
	var tree = &types.TreeReq{}
	if len(modelTreeQuerys) == 3 {
		repoID, err := strconv.ParseInt(modelTreeQuerys[2], 10, 64)
		if err != nil {
			err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "model_tree"))
			return nil, fmt.Errorf("failed to parse model tree: %w", err)
		}
		tree.Relation = types.ModelRelation(modelTreeQuerys[1])
		tree.RepoId = repoID
		return tree, nil
	} else {
		err := errorx.ReqParamInvalid(errors.New("invalid model_tree param"), errorx.Ctx().Set("query", "model_tree"))
		return nil, fmt.Errorf("failed to parse model tree: %w", err)
	}
}

func parseTagReqs(ctx *gin.Context) (tags []types.TagReq) {
	tagCategories := ctx.QueryArray("tag_category")
	tagNames := ctx.QueryArray("tag_name")
	tagGroups := ctx.QueryArray("tag_group")
	if len(tagCategories) > 0 {
		for i, category := range tagCategories {
			var tag types.TagReq
			tag.Category = strings.ToLower(category)
			if len(tagCategories) == len(tagNames) {
				tag.Name = strings.ToLower(tagNames[i])
			}
			if len(tagCategories) == len(tagGroups) {
				tag.Group = strings.ToLower(tagGroups[i])
			}
			tags = append(tags, tag)
		}
		return
	}

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

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	syncing, err := h.repo.IsSyncing(ctx.Request.Context(), types.ModelRepo, namespace, name)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check if model is syncing", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	if syncing {
		slog.ErrorContext(ctx.Request.Context(), "model is syncing", "error", err)
		httpbase.Conflict(ctx, errors.New("model is syncing, please try again later"))
		return
	}

	allow, err := h.repo.AllowReadAccess(ctx.Request.Context(), types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Info("user do not allowed to create deploy", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user is not authorized to read this repository for create deploy"))
		return
	}

	var req types.ModelRunReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	if req.MinReplica < 0 || req.MaxReplica < 0 || req.MinReplica > req.MaxReplica {
		slog.ErrorContext(ctx.Request.Context(), "Bad request setting for replica", slog.Any("MinReplica", req.MinReplica), slog.Any("MaxReplica", req.MaxReplica))
		ext := errorx.Ctx().Set("body", "MinReplica or MaxReplica")
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, ext))
		return
	}
	// for reserved resource,no scaling
	if req.OrderDetailID != 0 {
		req.MinReplica = 1
		req.MaxReplica = 1
	}

	_, err = h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}

	epReq := types.DeployActReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.InferenceType,
	}

	valid, err := common.IsValidName(req.DeployName)
	if !valid {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	if len(req.EngineArgs) > 0 {
		_, err = common.JsonStrToMap(req.EngineArgs)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
	}

	deployID, err := h.model.Deploy(ctx.Request.Context(), epReq, req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to deploy model as inference", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("currentUser", currentUser), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("deploy model as inference created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))
	h.createAgentInstanceTask(ctx.Request.Context(), req.Agent, fmt.Sprintf("%d", deployID), types.AgentTaskTypeInference, currentUser)

	h.createAgentInstanceTask(ctx.Request.Context(), req.Agent, fmt.Sprintf("%d", deployID), types.AgentTaskTypeInference, currentUser)

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

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	syncing, err := h.repo.IsSyncing(ctx.Request.Context(), types.ModelRepo, namespace, name)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check if model is syncing", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	if syncing {
		slog.ErrorContext(ctx.Request.Context(), "model is syncing", "error", err)
		httpbase.Conflict(ctx, errors.New("model is syncing, please try again later"))
		return
	}

	allow, err := h.repo.AllowReadAccess(ctx.Request.Context(), types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}
	if !allow {
		slog.Info("user is not allowed to run model", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to run model"))
		return
	}

	var req types.InstanceRunReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
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
		OrderDetailID:      req.OrderDetailID,
	}

	ftReq := types.DeployActReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.FinetuneType,
	}

	valid, err := common.IsValidName(req.DeployName)
	if !valid {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	if len(req.EngineArgs) > 0 {
		_, err = common.JsonStrToMap(req.EngineArgs)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
	}

	deployID, err := h.model.Deploy(ctx.Request.Context(), ftReq, *modelReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to deploy model as notebook instance", slog.String("namespace", namespace),
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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
	err = h.repo.DeleteDeploy(ctx.Request.Context(), delReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to delete inference", slog.Any("error", err), slog.Any("req", delReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete inference", slog.Any("error", err), slog.Any("req", delReq))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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
	err = h.repo.DeleteDeploy(ctx.Request.Context(), delReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "not allowed to delete finetune", slog.Any("error", err), slog.Any("req", delReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete finetune", slog.Any("error", err), slog.Any("req", delReq))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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
	err = h.repo.DeployStop(ctx.Request.Context(), stopReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to stop inference", slog.Any("error", err), slog.Any("req", stopReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to stop inference", slog.Any("error", err), slog.Any("req", stopReq))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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

	err = h.repo.DeployStart(ctx.Request.Context(), startReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to start inference", slog.Any("error", err), slog.Any("req", startReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to start inference", slog.Any("error", err), slog.Any("req", startReq))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// WakeupDeploy   godoc
// @Security     ApiKey
// @Summary      Wake up a model inference
// @Description  Wake up  a model inference
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
// @Router       /models/{namespace}/{name}/run/{id}/wakeup [put]
func (h *ModelHandler) DeployWakeup(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	err = h.model.Wakeup(ctx.Request.Context(), namespace, name, id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to wakeup inference", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("failed to wakeup inference"))
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
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "deploy_type"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	models, total, err := h.model.ListModelsByRuntimeFrameworkID(ctx.Request.Context(), currentUser, per, page, id, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get models", slog.Any("error", err))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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
	err = h.repo.DeployStop(ctx.Request.Context(), stopReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to stop finetune", slog.Any("req", stopReq), slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to stop finetune", slog.Any("req", stopReq), slog.Any("error", err))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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
	err = h.repo.DeployStart(ctx.Request.Context(), startReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to start finetune", slog.Any("error", err), slog.Any("req", startReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to start finetune", slog.Any("error", err), slog.Any("req", startReq))
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
// @Param 		 deploy_type query int false "deploy_type" Enums(1, 2, 3, 4, 5) default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework [get]
func (h *ModelHandler) ListAllRuntimeFramework(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		deployTypeStr = "0"
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "deploy_type"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	runtimes, err := h.model.ListAllByRuntimeFramework(ctx.Request.Context(), currentUser, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get runtime frameworks", slog.Any("error", err))
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
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/models [put]
func (h *ModelHandler) UpdateModelRuntimeFrameworks(ctx *gin.Context) {
	var req types.RuntimeFrameworkModels
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "deploy_type"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	slog.Info("update runtime frameworks models", slog.Any("req", req), slog.Any("runtime framework id", id), slog.Any("deployType", deployType))

	list, err := h.model.SetRuntimeFrameworkModes(ctx.Request.Context(), deployType, id, req.Models)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to set models runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, list)
}

// DeleteModelRuntime godoc
// @Security     ApiKey
// @Summary      Set model runtime frameworks
// @Description  set model runtime frameworks
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Param        body body types.RuntimeFrameworkModels true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/models [delete]
func (h *ModelHandler) DeleteModelRuntimeFrameworks(ctx *gin.Context) {
	var req types.RuntimeFrameworkModels
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "deploy_type"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	slog.Info("update runtime frameworks models", slog.Any("req", req), slog.Any("runtime framework id", id), slog.Any("deployType", deployType))

	list, err := h.model.DeleteRuntimeFrameworkModes(ctx.Request.Context(), deployType, id, req.Models)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to set models runtime framework", slog.Any("error", err))
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
	filter = getFilterFromContext(ctx, filter)
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request deploy type format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "deploy_type"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request per and page format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	models, total, err := h.model.ListModelsOfRuntimeFrameworks(ctx.Request.Context(), currentUser, filter.Search, filter.Sort, per, page, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to get models for all runtime frameworks", slog.Any("deployType", deployType), slog.Any("per", per), slog.Any("page", page), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  models,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
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

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var req types.ModelRunReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	if req.MinReplica < 0 || req.MaxReplica < 0 || req.MinReplica > req.MaxReplica {
		slog.ErrorContext(ctx.Request.Context(), "Bad request setting for replica", slog.Any("MinReplica", req.MinReplica), slog.Any("MaxReplica", req.MaxReplica))
		ext := errorx.Ctx().Set("body", "MinReplica or MaxReplica")
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, ext))
		return
	}

	deployReq := types.DeployActReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.ServerlessType,
	}

	req.SecureLevel = 1 // public for serverless

	valid, err := common.IsValidName(req.DeployName)
	if !valid {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	if len(req.EngineArgs) > 0 {
		_, err = common.JsonStrToMap(req.EngineArgs)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
	}

	deployID, err := h.model.Deploy(ctx.Request.Context(), deployReq, req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to deploy model as serverless", slog.Any("error", err), slog.Any("deploy_req", deployReq))

			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "failed to deploy model as serverless", slog.Any("deploy_req", deployReq), slog.Any("run_req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("deploy model as serverless created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID), slog.String("current_user", currentUser))

	// return deploy_id
	response := types.DeployRepo{DeployID: deployID}

	httpbase.OK(ctx, response)
}

// RemoveServerless  godoc
// @Security     ApiKey
// @Summary      remove a serverless service
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless [delete]
func (h *ModelHandler) RemoveServerless(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	startReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployType:  types.ServerlessType,
	}

	err = h.repo.DeleteDeploy(ctx.Request.Context(), startReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to remove model serverless deploy", slog.Any("error", err), slog.Any("req", startReq))

			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Info("failed to remove model serverless deploy", slog.Any("error", err), slog.Any("req", startReq))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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

	err = h.repo.DeployStart(ctx.Request.Context(), startReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to start model serverless deploy", slog.Any("error", err), slog.Any("req", startReq))

			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Info("failed to start model serverless deploy", slog.Any("error", err), slog.Any("req", startReq))
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
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

	err = h.repo.DeployStop(ctx.Request.Context(), stopReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to stop deploy", slog.Any("error", err))
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
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	response, err := h.model.GetServerless(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get model serverless endpoint", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("currentUser", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// ListQuantization      godoc
// @Security     ApiKey
// @Summary      list all gguf quantizations
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/quantizations [get]
func (h *ModelHandler) ListQuantizations(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	files, err := h.model.ListQuantizations(ctx.Request.Context(), namespace, name)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to list quantizations", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, files)
}

// CreateInferenceVersion      godoc
// @Security     ApiKey
// @Summary      create a new inference version
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        req body types.CreateInferenceVersionReq true "req"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run/versions/{id} [post]
func (h *ModelHandler) CreateInferenceVersion(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if id == 0 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	var versionReq types.CreateInferenceVersionReq
	if err := ctx.ShouldBindJSON(&versionReq); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to bind json", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	versionReq.DeployId = id
	err = h.model.CreateInferenceVersion(ctx.Request.Context(), versionReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to create inference version", "error", err, "req", versionReq)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// ListInferenceVersions      godoc
// @Security     ApiKey
// @Summary      list all inference versions
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Success      200  {object}  types.Response{data=[]types.ListInferenceVersionsResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run/versions/{id} [get]
func (h *ModelHandler) ListInferenceVersions(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if id == 0 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	versions, err := h.model.ListInferenceVersions(ctx.Request.Context(), id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to list inference versions", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, versions)
}

// UpdateInferenceVersionTraffic      godoc
// @Security     ApiKey
// @Summary      update inference version traffic percent
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        req body []types.UpdateInferenceVersionTrafficReq true "req"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run/versions/{id}/traffic [put]
func (h *ModelHandler) UpdateInferenceTraffic(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if id == 0 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var trafficReq []types.UpdateInferenceVersionTrafficReq
	if err := ctx.ShouldBindJSON(&trafficReq); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to bind json", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	err = h.model.UpdateInferenceVersionTraffic(ctx.Request.Context(), id, trafficReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to update inference version traffic", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// DeleteInferenceVersion      godoc
// @Security     ApiKey
// @Summary      delete inference version
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        commit_id path string true "commit_id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/run/versions/{id}/{commit_id} [delete]
func (h *ModelHandler) DeleteInferenceVersion(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	commit_id := ctx.Param("commit_id")

	err = h.model.DeleteInferenceVersion(ctx.Request.Context(), id, commit_id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to delete inference version", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
