package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewCodeHandler(config *config.Config) (*CodeHandler, error) {
	tc, err := component.NewCodeComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	repo, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating repo component:%w", err)
	}
	return &CodeHandler{
		code:      tc,
		sensitive: sc,
		repo:      repo,
	}, nil
}

type CodeHandler struct {
	code      component.CodeComponent
	sensitive component.SensitiveComponent
	repo      component.RepoComponent
}

// CreateCode   godoc
// @Security     ApiKey
// @Summary      Create a new code
// @Description  create a new code
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.CreateCodeReq true "body"
// @Success      200  {object}  types.Response{data=types.Code} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes [post]
func (h *CodeHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.CreateCodeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	if req.Namespace == "" {
		req.Namespace = currentUser
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	code, err := h.code.Create(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		} else if errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			slog.Error("Failed to create code", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
		return
	}
	slog.Info("Create code succeed", slog.String("code", code.Name))
	httpbase.OK(ctx, code)
}

// GetVisiableCodes godoc
// @Security     ApiKey
// @Summary      Get Visiable codes for current user
// @Description  get visiable codes for current user
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Code,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes [get]
func (h *CodeHandler) Index(ctx *gin.Context) {
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
	if !slices.Contains(types.Sorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", types.Sorts)
		err := errorx.ReqParamInvalid(errors.New(msg),
			errorx.Ctx().
				Set("param", "sort").
				Set("provided", filter.Sort).
				Set("allowed", types.Sorts))
		slog.Error("Bad request format,", slog.String("error", msg))
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
		slog.Error("Bad request format,", slog.String("error", msg))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}

	codes, total, err := h.code.Index(ctx.Request.Context(), filter, per, page, needOpWeight)
	if err != nil {
		slog.Error("Failed to get codes", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public codes succeed", slog.Int("count", total))
	httpbase.OKWithTotal(ctx, codes, total)
}

// UpdateCode   godoc
// @Security     ApiKey
// @Summary      Update a exists code
// @Description  update a exists code
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.UpdateCodeReq true "body"
// @Success      200  {object}  types.Response{data=database.Code} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name} [put]
func (h *CodeHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateCodeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	req.Username = currentUser

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name

	code, err := h.code.Update(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update code succeed", slog.String("code", code.Name))
	httpbase.OK(ctx, code)
}

// DeleteCode   godoc
// @Security     ApiKey
// @Summary      Delete a exists code
// @Description  delete a exists code
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name} [delete]
func (h *CodeHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.code.Delete(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to delete code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete code succeed", slog.String("code", name))
	httpbase.OK(ctx, nil)
}

// GetCode      godoc
// @Security     ApiKey
// @Summary      Get code detail
// @Description  get code detail
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success      200  {object}  types.Response{data=types.Code} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name} [get]
func (h *CodeHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
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
		slog.Error("bad need_multi_sync params", slog.Any("need_multi_sync", qNeedMultiSync), slog.Any("error", err))
		needMultiSync = false
	}

	detail, err := h.code.Show(ctx.Request.Context(), namespace, name, currentUser, needOpWeight, needMultiSync)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to get code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get code succeed", slog.String("code", name))
	httpbase.OK(ctx, detail)
}

// CodelRelations      godoc
// @Security     ApiKey
// @Summary      Get code related assets
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current_user"
// @Success      200  {object}  types.Response{data=types.Relations} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/relations [get]
func (h *CodeHandler) Relations(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.code.Relations(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to get code relations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}
