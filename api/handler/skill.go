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

func NewSkillHandler(config *config.Config) (*SkillHandler, error) {
	tc, err := component.NewSkillComponent(config)
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
	return &SkillHandler{
		skill:     tc,
		sensitive: sc,
		repo:      repo,
	}, nil
}

type SkillHandler struct {
	skill     component.SkillComponent
	sensitive component.SensitiveComponent
	repo      component.RepoComponent
}

// CreateSkill   godoc
// @Security     ApiKey
// @Summary      Create a new skill
// @Description  create a new skill
// @Tags         Skill
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.CreateSkillReq true "body"
// @Success      200  {object}  types.Response{data=types.Skill} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /skills [post]
func (h *SkillHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.CreateSkillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	if req.Namespace == "" {
		req.Namespace = currentUser
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	skill, err := h.skill.Create(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		} else if errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "Failed to create skill", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
		return
	}
	slog.Info("Create skill succeed", slog.String("skill", skill.Name))
	httpbase.OK(ctx, skill)
}

// GetVisiableSkills godoc
// @Security     ApiKey
// @Summary      Get Visiable skills for current user
// @Description  get visiable skills for current user
// @Tags         Skill
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Skill,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /skills [get]
func (h *SkillHandler) Index(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	filter.Tags = parseTagReqs(ctx)
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
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

	skills, total, err := h.skill.Index(ctx.Request.Context(), filter, per, page, needOpWeight)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get skills", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public skills succeed", slog.Int("count", total))
	httpbase.OKWithTotal(ctx, skills, total)
}

// UpdateSkill   godoc
// @Security     ApiKey
// @Summary      Update a exists skill
// @Description  update a exists skill
// @Tags         Skill
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.UpdateSkillReq true "body"
// @Success      200  {object}  types.Response{data=database.Skill} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /skills/{namespace}/{name} [put]
func (h *SkillHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateSkillReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}
	req.Username = currentUser

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name

	skill, err := h.skill.Update(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update skill", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update skill succeed", slog.String("skill", skill.Name))
	httpbase.OK(ctx, skill)
}

// DeleteSkill   godoc
// @Security     ApiKey
// @Summary      Delete a exists skill
// @Description  delete a exists skill
// @Tags         Skill
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /skills/{namespace}/{name} [delete]
func (h *SkillHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.skill.Delete(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete skill", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete skill succeed", slog.String("skill", name))
	httpbase.OK(ctx, nil)
}

// GetSkill      godoc
// @Security     ApiKey
// @Summary      Get skill detail
// @Description  get skill detail
// @Tags         Skill
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success      200  {object}  types.Response{data=types.Skill} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /skills/{namespace}/{name} [get]
func (h *SkillHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
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
		slog.ErrorContext(ctx.Request.Context(), "bad need_multi_sync params", slog.Any("need_multi_sync", qNeedMultiSync), slog.Any("error", err))
		needMultiSync = false
	}

	detail, err := h.skill.Show(ctx.Request.Context(), namespace, name, currentUser, needOpWeight, needMultiSync)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get skill", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get skill succeed", slog.String("skill", name))
	httpbase.OK(ctx, detail)
}

// SkillRelations      godoc
// @Security     ApiKey
// @Summary      Get skill related assets
// @Tags         Skill
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current_user"
// @Success      200  {object}  types.Response{data=types.Relations} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /skills/{namespace}/{name}/relations [get]
func (h *SkillHandler) Relations(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.skill.Relations(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get skill relations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}
