package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	common "opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type MemoryHandler struct {
	memory component.MemoryComponent
}

func NewMemoryHandler(memoryComp component.MemoryComponent) *MemoryHandler {
	return &MemoryHandler{
		memory: memoryComp,
	}
}

func (h *MemoryHandler) CreateProject(ctx *gin.Context) {
	var req types.CreateMemoryProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.OrgID == "" || req.ProjectID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("org_id and project_id are required"),
			errorx.Ctx().Set("field", "org_id,project_id"),
		))
		return
	}
	resp, err := h.memory.CreateProject(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to create memory project", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, filterProjectResponse(resp))
}

func (h *MemoryHandler) GetProject(ctx *gin.Context) {
	var req types.GetMemoryProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.OrgID == "" || req.ProjectID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("org_id and project_id are required"),
			errorx.Ctx().Set("field", "org_id,project_id"),
		))
		return
	}
	resp, err := h.memory.GetProject(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get memory project", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, filterProjectResponse(resp))
}

func (h *MemoryHandler) ListProjects(ctx *gin.Context) {
	resp, err := h.memory.ListProjects(ctx.Request.Context())
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to list memory projects", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, filterProjectListResponse(resp))
}

func (h *MemoryHandler) DeleteProject(ctx *gin.Context) {
	var req types.DeleteMemoryProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.OrgID == "" || req.ProjectID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("org_id and project_id are required"),
			errorx.Ctx().Set("field", "org_id,project_id"),
		))
		return
	}
	if err := h.memory.DeleteProject(ctx.Request.Context(), &req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to delete memory project", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, gin.H{"deleted": true})
}

func (h *MemoryHandler) AddMemories(ctx *gin.Context) {
	var req types.AddMemoriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if len(req.Messages) == 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("messages is required"),
			errorx.Ctx().Set("field", "messages"),
		))
		return
	}
	for i, msg := range req.Messages {
		if msg.Scopes != nil && (msg.Scopes.AgentID != "" || msg.Scopes.OrgID != "" || msg.Scopes.ProjectID != "" || msg.Scopes.SessionID != "") {
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
				fmt.Errorf("message scopes must not be set; use request-level scope fields (agent_id, session_id, org_id, project_id)"),
				errorx.Ctx().Set("field", fmt.Sprintf("messages[%d].scopes", i)),
			))
			return
		}
		if strings.TrimSpace(msg.Content) == "" {
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
				fmt.Errorf("message content is required"),
				errorx.Ctx().Set("field", fmt.Sprintf("messages[%d].content", i)),
			))
			return
		}
	}
	if err := validateMemoryTypes(req.Types); err != nil {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			err,
			errorx.Ctx().Set("field", "types"),
		))
		return
	}
	resp, err := h.memory.AddMemories(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to add memories", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, filterAddMemoriesResponse(resp))
}

func filterProjectResponse(resp *types.MemoryProjectResponse) *types.MemoryProjectResponse {
	if resp == nil {
		return nil
	}
	return &types.MemoryProjectResponse{
		OrgID:       resp.OrgID,
		ProjectID:   resp.ProjectID,
		Description: resp.Description,
	}
}

func filterProjectListResponse(items []*types.MemoryProjectRef) []*types.MemoryProjectRef {
	if len(items) == 0 {
		return items
	}
	out := make([]*types.MemoryProjectRef, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, &types.MemoryProjectRef{
			OrgID:     item.OrgID,
			ProjectID: item.ProjectID,
		})
	}
	return out
}

func (h *MemoryHandler) SearchMemories(ctx *gin.Context) {
	var req types.SearchMemoriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.PageSize < 0 || req.PageNum < 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("page_size and page_num must be non-negative"),
			errorx.Ctx().Set("field", "page_size,page_num"),
		))
		return
	}
	if (req.PageSize > 0 && req.PageNum <= 0) || (req.PageNum > 0 && req.PageSize <= 0) {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("page_size and page_num must be provided together"),
			errorx.Ctx().Set("field", "page_size,page_num"),
		))
		return
	}
	if req.PageSize == 0 && req.PageNum == 0 && (ctx.Query("per") != "" || ctx.Query("page") != "") {
		per, page, err := common.GetPerAndPageFromContext(ctx)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		req.PageSize = per
		req.PageNum = page
	}
	if req.TopK < 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("top_k must be non-negative"),
			errorx.Ctx().Set("field", "top_k"),
		))
		return
	}
	if req.MinSimilarity != nil && (*req.MinSimilarity < 0 || *req.MinSimilarity > 1) {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("min_similarity must be between 0 and 1"),
			errorx.Ctx().Set("field", "min_similarity"),
		))
		return
	}
	if err := validateMemoryTypes(req.Types); err != nil {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			err,
			errorx.Ctx().Set("field", "types"),
		))
		return
	}
	if req.PageSize < 0 || req.PageNum < 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("page_size and page_num must be non-negative"),
			errorx.Ctx().Set("field", "page_size,page_num"),
		))
		return
	}
	if (req.PageSize > 0 && req.PageNum <= 0) || (req.PageNum > 0 && req.PageSize <= 0) {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("page_size and page_num must be provided together"),
			errorx.Ctx().Set("field", "page_size,page_num"),
		))
		return
	}
	if req.PageSize == 0 && req.PageNum == 0 && (ctx.Query("per") != "" || ctx.Query("page") != "") {
		per, page, err := common.GetPerAndPageFromContext(ctx)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		req.PageSize = per
		req.PageNum = page
	}
	resp, err := h.memory.SearchMemories(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to search memories", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, filterSearchMemoriesResponse(resp))
}

func (h *MemoryHandler) ListMemories(ctx *gin.Context) {
	var req types.ListMemoriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.PageSize < 0 || req.PageNum < 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("page_size and page_num must be non-negative"),
			errorx.Ctx().Set("field", "page_size,page_num"),
		))
		return
	}
	if (req.PageSize > 0 && req.PageNum <= 0) || (req.PageNum > 0 && req.PageSize <= 0) {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("page_size and page_num must be provided together"),
			errorx.Ctx().Set("field", "page_size,page_num"),
		))
		return
	}
	if req.PageSize == 0 && req.PageNum == 0 && (ctx.Query("per") != "" || ctx.Query("page") != "") {
		per, page, err := common.GetPerAndPageFromContext(ctx)
		if err != nil {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		req.PageSize = per
		req.PageNum = page
	}
	if err := validateMemoryTypes(req.Types); err != nil {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			err,
			errorx.Ctx().Set("field", "types"),
		))
		return
	}
	resp, err := h.memory.ListMemories(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to list memories", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, filterListMemoriesResponse(resp))
}

func (h *MemoryHandler) DeleteMemories(ctx *gin.Context) {
	var req types.DeleteMemoriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	if req.UID == "" && len(req.UIDs) == 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("uid or uids is required"),
			errorx.Ctx().Set("field", "uid,uids"),
		))
		return
	}
	if err := h.memory.DeleteMemories(ctx.Request.Context(), &req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to delete memories", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, gin.H{"deleted": true})
}

func (h *MemoryHandler) Health(ctx *gin.Context) {
	resp, err := h.memory.Health(ctx.Request.Context())
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check memory health", slog.Any("error", err))
		h.respondMemoryError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

func (h *MemoryHandler) respondMemoryError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, errorx.ErrUnauthorized) || errors.Is(err, errorx.ErrUserNotFound) || errors.Is(err, errorx.ErrNeedAPIKey) {
		httpbase.UnauthorizedError(ctx, err)
		return
	}
	if errors.Is(err, errorx.ErrForbidden) {
		httpbase.ForbiddenError(ctx, err)
		return
	}
	if errors.Is(err, errorx.ErrNotFound) || errors.Is(err, errorx.ErrDatabaseNoRows) {
		httpbase.NotFoundError(ctx, err)
		return
	}
	if errors.Is(err, errorx.ErrAlreadyExists) || errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
		httpbase.ConflictError(ctx, err)
		return
	}
	if errors.Is(err, errorx.ErrRemoteServiceFail) {
		httpbase.ServiceUnavailableError(ctx, err)
		return
	}
	if customErr, ok := errorx.GetFirstCustomError(err); ok {
		custom := customErr.(errorx.CustomError)
		if strings.HasPrefix(custom.Code(), "REQ-ERR") {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
	}
	httpbase.ServerError(ctx, err)
}

func validateMemoryTypes(typesList []types.MemoryType) error {
	if len(typesList) == 0 {
		return nil
	}
	for _, t := range typesList {
		if t != types.MemoryTypeEpisodic && t != types.MemoryTypeSemantic {
			return fmt.Errorf("unsupported memory type: %s", t)
		}
	}
	return nil
}

func filterAddMemoriesResponse(resp *types.AddMemoriesResponse) *types.AddMemoriesResponse {
	if resp == nil {
		return nil
	}
	return &types.AddMemoriesResponse{
		Created: filterMemoryMessages(resp.Created),
	}
}

func filterSearchMemoriesResponse(resp *types.SearchMemoriesResponse) *types.SearchMemoriesResponse {
	if resp == nil {
		return nil
	}
	return &types.SearchMemoriesResponse{
		Status:  resp.Status,
		Content: filterMemoryMessages(resp.Content),
	}
}

func filterListMemoriesResponse(resp *types.ListMemoriesResponse) *types.ListMemoriesResponse {
	if resp == nil {
		return nil
	}
	return &types.ListMemoriesResponse{
		Status:  resp.Status,
		Content: filterMemoryMessages(resp.Content),
	}
}

func filterMemoryMessages(messages []types.MemoryMessage) []types.MemoryMessage {
	if len(messages) == 0 {
		return messages
	}
	out := make([]types.MemoryMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, types.MemoryMessage{
			UID:        msg.UID,
			Content:    msg.Content,
			Timestamp:  msg.Timestamp,
			Role:       msg.Role,
			Scopes:     msg.Scopes,
			UserID:     msg.UserID,
			MetaData:   msg.MetaData,
			Similarity: msg.Similarity,
		})
	}
	return out
}
