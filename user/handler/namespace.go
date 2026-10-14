package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/user/component"
)

type NamespaceHandler struct {
	c                       component.NamespaceComponent
	maxNamespaceLookupUUIDs int
}

func NewNamespaceHandler(config *config.Config) (*NamespaceHandler, error) {
	nc, err := component.NewNamespaceComponent(config)
	if err != nil {
		return nil, err
	}
	maxNamespaceLookupUUIDs := config.User.NamespaceLookupMaxUUIDs
	if maxNamespaceLookupUUIDs <= 0 {
		maxNamespaceLookupUUIDs = 100
	}
	return &NamespaceHandler{
		c:                       nc,
		maxNamespaceLookupUUIDs: maxNamespaceLookupUUIDs,
	}, nil
}

// GetInfo godoc
// @Security     ApiKey
// @Summary      Get namespace info [Internal Only].
// @Description  get namespace info
// @Tags         Namespace, InternalOnly
// @Accept       json
// @Produce      json
// @Param        path path string true "namespace"
// @Success      200 {object} types.Response{data=types.Namespace} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespace/{path} [get]
func (h *NamespaceHandler) GetInfo(ctx *gin.Context) {
	path := ctx.Param("path")
	ns, err := h.c.GetInfo(ctx, path)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ns)
}

// GetInfoByUUID godoc
// @Security     ApiKey
// @Summary      Get namespace info by UUID [Internal Only].
// @Description  get namespace info by uuid
// @Tags         Namespace, InternalOnly
// @Accept       json
// @Produce      json
// @Param        uuid path string true "namespace uuid"
// @Success      200 {object} types.Response{data=types.Namespace} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespace/uuid/{uuid} [get]
func (h *NamespaceHandler) GetInfoByUUID(ctx *gin.Context) {
	uuid := ctx.Param("uuid")
	ns, err := h.c.GetInfoByUUID(ctx, uuid)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ns)
}

// GetInfoByUUIDs godoc
// @Summary      Get namespace info by UUIDs.
// @Description  get namespace info by uuids
// @Tags         Namespace
// @Accept       json
// @Produce      json
// @Param        uuids body []string true "namespace uuids"
// @Success      200 {object} types.Response{data=[]types.Namespace} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /admin/namespaces/lookup [post]
func (h *NamespaceHandler) GetInfoByUUIDs(ctx *gin.Context) {
	var uuids []string
	if err := ctx.ShouldBindJSON(&uuids); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format for namespace uuids", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	uuids, err := h.normalizeNamespaceLookupUUIDs(uuids)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	namespaces, err := h.c.GetInfoByUUIDs(ctx.Request.Context(), uuids)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get namespaces by uuids", slog.Any("error", err), slog.Any("uuids", uuids))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, namespaces)
}

func (h *NamespaceHandler) normalizeNamespaceLookupUUIDs(uuids []string) ([]string, error) {
	if len(uuids) == 0 {
		return nil, errorx.ErrReqBodyEmpty
	}
	if len(uuids) > h.maxNamespaceLookupUUIDs {
		return nil, errorx.ReqParamInvalid(
			fmt.Errorf("namespace uuid count must not exceed %d", h.maxNamespaceLookupUUIDs),
			errorx.Ctx().Set("field", "uuids").Set("limit", h.maxNamespaceLookupUUIDs),
		)
	}

	normalized := make([]string, 0, len(uuids))
	seen := make(map[string]struct{}, len(uuids))
	for i, rawUUID := range uuids {
		if strings.TrimSpace(rawUUID) == "" {
			return nil, errorx.ReqParamInvalid(
				errors.New("namespace uuid cannot be empty"),
				errorx.Ctx().Set("field", "uuids").Set("index", i),
			)
		}
		parsedUUID, err := uuid.Parse(rawUUID)
		if err != nil {
			return nil, errorx.ReqParamInvalid(
				fmt.Errorf("namespace uuid %q is invalid: %w", rawUUID, err),
				errorx.Ctx().Set("field", "uuids").Set("index", i),
			)
		}
		normalizedUUID := parsedUUID.String()
		if _, ok := seen[normalizedUUID]; ok {
			continue
		}
		seen[normalizedUUID] = struct{}{}
		normalized = append(normalized, normalizedUUID)
	}
	return normalized, nil
}
