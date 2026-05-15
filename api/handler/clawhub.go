package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func clawHubOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

func clawHubError(c *gin.Context, statusCode int, err error) {
	resp := gin.H{"error": err.Error()}
	if customErr, ok := errorx.GetFirstCustomError(err); ok {
		ce := customErr.(errorx.CustomError)
		resp["code"] = ce.Code()
		resp["context"] = ce.Context()
	}
	c.JSON(statusCode, resp)
}

func clawHubBadRequest(c *gin.Context, err error) {
	clawHubError(c, http.StatusBadRequest, err)
}

func clawHubNotFound(c *gin.Context, err error) {
	clawHubError(c, http.StatusNotFound, err)
}

func clawHubForbidden(c *gin.Context, err error) {
	clawHubError(c, http.StatusForbidden, err)
}

func clawHubServerError(c *gin.Context, err error) {
	clawHubError(c, http.StatusInternalServerError, err)
}

func NewClawHubHandler(config *config.Config) (*ClawHubHandler, error) {
	clawhub, err := component.NewClawHubComponent(config)
	if err != nil {
		return nil, err
	}
	return &ClawHubHandler{clawhub: clawhub}, nil
}

type ClawHubHandler struct {
	clawhub component.ClawHubComponent
}

// ClawHubSearch godoc
// @Summary      Search skills for ClawHub
// @Description  Search skills for ClawHub
// @Tags         ClawHub
// @Accept       json
// @Produce      json
// @Param        q query string true "search query"
// @Param        limit query int false "limit"
// @Success      200  {object}  types.ClawHubSearchResponse "OK"
// @Router       /claw/api/v1/search [get]
func (h *ClawHubHandler) Search(ctx *gin.Context) {
	query := ctx.Query("q")
	limit := 20
	if limitStr := ctx.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	response, err := h.clawhub.Search(ctx.Request.Context(), query, limit, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to search skills", slog.Any("error", err))
		clawHubServerError(ctx, fmt.Errorf("failed to search skills: %w", err))
		return
	}

	clawHubOK(ctx, response)
}

// ClawHubGetSkill godoc
// @Summary      Get skill detail for ClawHub install
// @Description  Get skill metadata for ClawHub install
// @Tags         ClawHub
// @Accept       json
// @Produce      json
// @Param        slug path string true "skill slug"
// @Success      200  {object}  types.ClawHubSkillResponse "OK"
// @Router       /claw/api/v1/skills/{slug} [get]
func (h *ClawHubHandler) GetSkill(ctx *gin.Context) {
	slug := unescapeClawHubSlug(ctx.Param("slug"))
	currentUser := httpbase.GetCurrentUser(ctx)

	response, err := h.clawhub.GetSkill(ctx.Request.Context(), slug, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get skill", slog.Any("error", err))
		if errors.Is(err, errorx.ErrSkillNotFound) {
			clawHubNotFound(ctx, err)
			return
		}
		clawHubServerError(ctx, err)
		return
	}

	clawHubOK(ctx, response)
}

// ClawHubGetSkillVersion godoc
// @Summary      Get skill version detail for ClawHub install
// @Description  Get skill metadata for a specified ClawHub version
// @Tags         ClawHub
// @Accept       json
// @Produce      json
// @Param        slug path string true "skill slug"
// @Param        version path string true "skill version"
// @Success      200  {object}  types.ClawHubSkillVersionResponse "OK"
// @Router       /claw/api/v1/skills/{slug}/versions/{version} [get]
func (h *ClawHubHandler) GetSkillVersion(ctx *gin.Context) {
	slug := unescapeClawHubSlug(ctx.Param("slug"))
	version := unescapeClawHubSlug(ctx.Param("version"))
	currentUser := httpbase.GetCurrentUser(ctx)

	response, err := h.clawhub.GetSkillVersion(ctx.Request.Context(), slug, version, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get skill version", slog.Any("error", err))
		if errors.Is(err, errorx.ErrSkillNotFound) {
			clawHubNotFound(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrSkillVersionNotFound) {
			clawHubNotFound(ctx, err)
			return
		}
		clawHubServerError(ctx, err)
		return
	}

	clawHubOK(ctx, response)
}

// ClawHubPublishSkill godoc
// @Summary      Publish skill for ClawHub
// @Description  Publish a new skill or version for ClawHub
// @Tags         ClawHub
// @Accept       multipart/form-data
// @Produce      json
// @Param        payload formData string true "JSON payload with skill metadata"
// @Param        files formData file true "skill files"
// @Success      200  {object}  types.ClawHubPublishSkillResponse "OK"
// @Router       /claw/api/v1/skills [post]
func (h *ClawHubHandler) PublishSkill(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	req, err := parseClawHubPublishRequest(ctx)
	if err != nil {
		clawHubBadRequest(ctx, fmt.Errorf("invalid payload format: %w", err))
		return
	}
	if req.Slug == "" {
		clawHubBadRequest(ctx, fmt.Errorf("missing slug parameter"))
		return
	}
	req.Slug, req.DisplayName = component.NormalizeClawHubSkillIdentity(req.Slug, req.DisplayName)

	form, err := ctx.MultipartForm()
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to parse multipart form", slog.Any("error", err))
		clawHubBadRequest(ctx, fmt.Errorf("invalid multipart form: %w", err))
		return
	}

	files, err := readClawHubFiles(form)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to read uploaded files", slog.Any("error", err))
		clawHubServerError(ctx, fmt.Errorf("failed to read uploaded file: %w", err))
		return
	}

	response, err := h.clawhub.PublishSkill(ctx.Request.Context(), &req, files, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to publish skill", slog.Any("error", err))
		if errors.Is(err, errorx.ErrForbidden) {
			clawHubForbidden(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrSkillPublishFileCountExceeded) || errors.Is(err, errorx.ErrSkillPublishFileSizeExceeded) {
			clawHubBadRequest(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrSkillPublishFailed) {
			clawHubServerError(ctx, err)
			return
		}
		clawHubServerError(ctx, err)
		return
	}

	clawHubOK(ctx, response)
}

func parseClawHubPublishRequest(ctx *gin.Context) (types.ClawHubPublishRequest, error) {
	var req types.ClawHubPublishRequest

	payloadStr := strings.TrimSpace(formValue(ctx, "payload"))
	if payloadStr != "" {
		if strings.HasPrefix(payloadStr, "{") {
			if err := json.Unmarshal([]byte(payloadStr), &req); err != nil {
				return req, err
			}
		} else if req.Slug == "" {
			req.Slug = filepath.Base(payloadStr)
		}
	}

	if req.Slug == "" {
		req.Slug = baseFormValue(ctx, "slug")
	}
	if req.DisplayName == "" {
		req.DisplayName = strings.TrimSpace(formValue(ctx, "displayName"))
	}
	if req.Version == "" {
		req.Version = strings.TrimSpace(formValue(ctx, "version"))
	}
	if req.Changelog == "" {
		req.Changelog = strings.TrimSpace(formValue(ctx, "changelog"))
	}

	return req, nil
}

func baseFormValue(ctx *gin.Context, key string) string {
	value := strings.TrimSpace(formValue(ctx, key))
	if value == "" {
		return ""
	}
	return filepath.Base(value)
}

func formValue(ctx *gin.Context, key string) string {
	if ctx.Request != nil && ctx.Request.MultipartForm != nil {
		if values := ctx.Request.MultipartForm.Value[key]; len(values) > 0 {
			return values[0]
		}
	}
	return ctx.PostForm(key)
}

func readClawHubFiles(form *multipart.Form) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for key, fileHeaders := range form.File {
		if key != "files" {
			continue
		}
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil {
				return nil, err
			}
			content, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil {
				return nil, readErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			files[fileHeader.Filename] = content
		}
	}
	return files, nil
}

// ClawHubDownloadSkill godoc
// @Summary      Download skill for ClawHub install
// @Description  Download skill zip for ClawHub install
// @Tags         ClawHub
// @Produce      application/zip
// @Param        slug query string true "skill slug"
// @Param        version query string false "skill version"
// @Success      200  {file}  file "OK"
// @Router       /claw/api/v1/download [get]
func (h *ClawHubHandler) DownloadSkill(ctx *gin.Context) {
	h.downloadSkill(ctx, ctx.Query("slug"))
}

// ClawHubDownloadSkillByPath godoc
// @Summary      Download skill by path for ClawHub install
// @Description  Download skill zip for ClawHub install
// @Tags         ClawHub
// @Produce      application/zip
// @Param        slug path string true "skill slug"
// @Param        version query string false "skill version"
// @Success      200  {file}  file "OK"
// @Router       /claw/api/v1/download/{slug} [get]
func (h *ClawHubHandler) DownloadSkillByPath(ctx *gin.Context) {
	h.downloadSkill(ctx, unescapeClawHubSlug(ctx.Param("slug")))
}

func (h *ClawHubHandler) downloadSkill(ctx *gin.Context, slug string) {
	version := ctx.Query("version")
	if version == "" {
		version = "latest"
	}
	currentUser := httpbase.GetCurrentUser(ctx)

	zipData, actualVersion, err := h.clawhub.DownloadSkill(ctx.Request.Context(), slug, version, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to download skill", slog.Any("error", err))
		if errors.Is(err, errorx.ErrSkillNotFound) || errors.Is(err, errorx.ErrSkillVersionNotFound) {
			clawHubNotFound(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrForbidden) {
			clawHubForbidden(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrSkillDownloadFailed) {
			clawHubServerError(ctx, err)
			return
		}
		clawHubServerError(ctx, err)
		return
	}

	filename := fmt.Sprintf("%s-%s.zip", slug, formatVersionForFilename(actualVersion))
	ctx.Header("Content-Type", "application/zip")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	ctx.Data(http.StatusOK, "application/zip", zipData)
}

func formatVersionForFilename(version string) string {
	version = strings.TrimPrefix(version, "v")
	return strings.ReplaceAll(version, ".", "-")
}

func unescapeClawHubSlug(slug string) string {
	unescapedSlug, err := url.PathUnescape(slug)
	if err != nil {
		return slug
	}
	return unescapedSlug
}

// ClawHubResolveSkill godoc
// @Summary      Resolve skill for ClawHub install
// @Description  Resolve skill version for ClawHub install
// @Tags         ClawHub
// @Accept       json
// @Produce      json
// @Param        slug query string true "skill slug"
// @Success      200  {object}  types.ClawHubResolveResponse "OK"
// @Router       /claw/api/v1/resolve [get]
func (h *ClawHubHandler) ResolveSkill(ctx *gin.Context) {
	h.resolveSkill(ctx, ctx.Query("slug"))
}

// ClawHubResolveSkillByPath godoc
// @Summary      Resolve skill by path for ClawHub install
// @Description  Resolve skill version for ClawHub install
// @Tags         ClawHub
// @Accept       json
// @Produce      json
// @Param        slug path string true "skill slug"
// @Success      200  {object}  types.ClawHubResolveResponse "OK"
// @Router       /claw/api/v1/resolve/{slug} [get]
func (h *ClawHubHandler) ResolveSkillByPath(ctx *gin.Context) {
	h.resolveSkill(ctx, unescapeClawHubSlug(ctx.Param("slug")))
}

func (h *ClawHubHandler) resolveSkill(ctx *gin.Context, slug string) {
	currentUser := httpbase.GetCurrentUser(ctx)

	result, err := h.clawhub.ResolveSkill(ctx.Request.Context(), slug, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to resolve skill", slog.Any("error", err))
		if errors.Is(err, errorx.ErrSkillNotFound) {
			clawHubNotFound(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrForbidden) {
			clawHubForbidden(ctx, err)
			return
		}
		clawHubServerError(ctx, err)
		return
	}

	clawHubOK(ctx, result)
}

// ClawHubWhoami godoc
// @Summary      Get current ClawHub login user
// @Description  Get current ClawHub login user
// @Tags         ClawHub
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ClawHubUserResponse "OK"
// @Router       /claw/api/v1/whoami [get]
func (h *ClawHubHandler) Whoami(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	response, err := h.clawhub.Whoami(ctx.Request.Context(), currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get current user", slog.Any("error", err))
		if errors.Is(err, errorx.ErrSkillUserNotFound) {
			clawHubNotFound(ctx, err)
			return
		}
		clawHubServerError(ctx, err)
		return
	}

	clawHubOK(ctx, response)
}
