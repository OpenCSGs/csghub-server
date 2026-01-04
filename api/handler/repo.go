package handler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/component/validator"
)

func NewRepoHandler(config *config.Config) (*RepoHandler, error) {
	uc, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	m, err := component.NewModelComponent(config)
	if err != nil {
		return nil, err
	}
	d, err := component.NewDatasetComponent(config)
	if err != nil {
		return nil, err
	}
	return &RepoHandler{
		c:                         uc,
		m:                         m,
		d:                         d,
		deployStatusCheckInterval: 5 * time.Second,
		config:                    config,
	}, nil
}

type RepoHandler struct {
	c                         component.RepoComponent
	m                         component.ModelComponent
	d                         component.DatasetComponent
	deployStatusCheckInterval time.Duration
	config                    *config.Config
}

// CreateRepo godoc
// @Security     ApiKey
// @Summary      Create a new repository, compatible with hf api
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        req body types.CreateRepoReq true  "create repo request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /repos/create [post]
func (h *RepoHandler) CreateRepo(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	var req *types.CreateRepoReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if req.RepoType == "" {
		req.RepoType = types.ModelRepo
	}

	req.Username = userName
	req.Namespace = userName
	switch req.RepoType {
	case types.ModelRepo:
		modelReq := &types.CreateModelReq{
			CreateRepoReq: *req,
		}
		resp, err := h.m.Create(ctx.Request.Context(), modelReq)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				resp := &types.Model{
					URL: fmt.Sprintf("%s/%s", req.Namespace, req.Name),
				}
				ctx.JSON(http.StatusConflict, resp)
				return
			}
			slog.ErrorContext(ctx.Request.Context(), "Failed to create model repo", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
			httpbase.ServerError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, resp)
	case types.DatasetRepo:
		datasetReq := &types.CreateDatasetReq{
			CreateRepoReq: *req,
		}
		resp, err := h.d.Create(ctx.Request.Context(), datasetReq)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				resp := &types.Dataset{
					URL: fmt.Sprintf("%s/%s", req.Namespace, req.Name),
				}
				ctx.JSON(http.StatusConflict, resp)
				return
			}
			slog.ErrorContext(ctx.Request.Context(), "Failed to create dataset repo", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
			httpbase.ServerError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, resp)
	default:
		// Unsupported repo type
		slog.ErrorContext(ctx.Request.Context(), "Unsupported repo type", slog.String("repo_type", string(req.RepoType)))
		httpbase.BadRequest(ctx, fmt.Sprintf("Unsupported repo type: %s", req.RepoType))
		return
	}
}

// ValidateYaml godoc
// @Security     ApiKey
// @Summary      Validate yaml, compatible with hf api
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        req body types.ValidateYamlReq true  "validate yaml content"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /validate-yaml [post]
func (h *RepoHandler) ValidateYaml(ctx *gin.Context) {
	var req types.ValidateYamlReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	err := h.c.ValidateYaml(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to validate yaml", slog.Any("error", err), slog.Any("req", req))
		ctx.JSON(http.StatusBadRequest, gin.H{"errors": []string{err.Error()}})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{})
}

// CreateRepoFile godoc
// @Security     ApiKey
// @Summary      Create a new file in repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,datasets,codes,spaces or mcps" Enums(models,datasets,codes,spaces,mcps)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "the new file relative path"
// @Param		 current_user query string false "current user name"
// @Param        req body types.CreateFileReq true  "create file request"
// @Success      200  {object}  types.ResponseWithTotal{data=types.CreateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/raw/{file_path} [post]
func (h *RepoHandler) CreateFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	var req *types.CreateFileReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = userName

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req.Namespace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.CurrentUser = userName

	resp, err := h.c.CreateFile(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug("Create repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, resp)
}

// UpdateRepoFile godoc
// @Security     ApiKey
// @Summary      Update existing file in repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,datasets,codes,spaces or mcps" Enums(models,datasets,codes,spaces,mcps)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "the new file relative path"
// @Param		 current_user query string false "current user name"
// @Param        req body types.UpdateFileReq true  "create file request"
// @Success      200  {object}  types.ResponseWithTotal{data=types.UpdateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/raw/{file_path} [put]
func (h *RepoHandler) UpdateFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateFileReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = userName

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req.Namespace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.CurrentUser = userName

	resp, err := h.c.UpdateFile(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug("Update repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, resp)
}

// DeleteRepoFile godoc
// @Security     ApiKey
// @Summary      Delete existing file in repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,datasets,codes,spaces or mcps" Enums(models,datasets,codes,spaces,mcps)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "the file relative path"
// @Param		 current_user query string false "current user name"
// @Param        req body types.DeleteFileReq true  "delete file request"
// @Success      200  {object}  types.ResponseWithTotal{data=types.DeleteFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/raw/{file_path} [delete]
func (h *RepoHandler) DeleteFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	var req *types.DeleteFileReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.Username = userName

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req.Namespace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.CurrentUser = userName
	req.Content = ""
	req.OriginPath = ""

	resp, err := h.c.DeleteFile(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Delete repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, resp)
}

// GetRepoCommits godoc
// @Security     ApiKey
// @Summary      Get all commits of repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/commits [get]
func (h *RepoHandler) Commits(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	ref := ctx.Query("ref")
	req := &types.GetCommitsReq{
		Namespace:   namespace,
		Name:        name,
		Ref:         ref,
		Per:         per,
		Page:        page,
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	commits, pageOpt, err := h.c.Commits(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo commits", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo commits succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("ref", req.Ref))
	resData := gin.H{
		"commits":    commits,
		"total":      pageOpt.Total,
		"page_count": pageOpt.PageCount,
	}
	httpbase.OK(ctx, resData)
}

// GetRepoLastCommit godoc
// @Security     ApiKey
// @Summary      Get the last commit of repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/last_commit [get]
func (h *RepoHandler) LastCommit(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	ref := ctx.Query("ref")
	req := &types.GetCommitsReq{
		Namespace:   namespace,
		Name:        name,
		Ref:         ref,
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	commit, err := h.c.LastCommit(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo last commit", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo last commit succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commit)
}

// GetRepoFileContent godoc
// @Security     ApiKey
// @Summary      Get the last commit of repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "file path"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=string} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/raw/{file_path} [get]
func (h *RepoHandler) FileRaw(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req := &types.GetFileReq{
		Namespace:   namespace,
		Name:        name,
		Path:        filePath,
		Ref:         ctx.Query("ref"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	raw, err := h.c.FileRaw(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrNotFound) {
			slog.ErrorContext(ctx.Request.Context(), "Repo file not found", slog.String("repo_type", string(req.RepoType)), slog.Any("req", req))
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo file raw", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo file raw succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, raw)
}

// GetRepoFileContent godoc
// @Security     ApiKey or JWT
// @Summary      Get the repo file information like size, content, sha etc
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "file path"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/blob/{file_path} [get]
func (h *RepoHandler) FileInfo(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req := &types.GetFileReq{
		Namespace:   namespace,
		Name:        name,
		Path:        filePath,
		Ref:         ctx.Query("ref"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	file, err := h.c.FileInfo(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo file info", slog.Any("req", req), slog.Any("error", err), slog.Any("req", req))
		if errors.Is(err, errorx.ErrGitFileNotFound) || errors.Is(err, errorx.ErrGitCommitNotFound) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.Debug("Get repo file info succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, file)
}

// DownloadRepoFile godoc
// @Security     ApiKey or JWT
// @Summary      Download a repo file [Depricated: use 'resolve' api instead]
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "file path"
// @Param		 ref query string false "branch or tag"
// @Param		 save_as query string false "file name to save as"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=object} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/download/{file_path} [get]
func (h *RepoHandler) DownloadFile(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req := &types.GetFileReq{
		Namespace:   namespace,
		Name:        name,
		Path:        filePath,
		Ref:         ctx.Query("ref"),
		Lfs:         false,
		SaveAs:      ctx.Query("save_as"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	if ctx.Query("lfs") != "" {
		req.Lfs, err = strconv.ParseBool(ctx.Query("lfs"))
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}

	reader, size, url, err := h.c.DownloadFile(ctx.Request.Context(), req, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	if req.Lfs {
		httpbase.OK(ctx, url)
	} else {
		slog.Debug("Download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		ctx.Header("Content-Length", strconv.FormatInt(size, 10))
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
			httpbase.ServerError(ctx, err)
			return
		}
	}
}

// GetRepoBranches
// @Security     ApiKey
// @Summary      Get the branches of repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Branch} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/branches [get]
func (h *RepoHandler) Branches(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetBranchesReq{
		Namespace:   namespace,
		Name:        name,
		Per:         per,
		Page:        page,
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	branches, err := h.c.Branches(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo branches", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo branches succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, branches)
}

// GetRepoTags
// @Security     ApiKey
// @Summary      Get the tags of repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/tags [get]
func (h *RepoHandler) Tags(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetTagsReq{
		Namespace:   namespace,
		Name:        name,
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	tags, err := h.c.Tags(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo tags", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo tags succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, tags)
}

// UpdateRepoTags
// @Security     ApiKey
// @Summary      update the tags of a certain category
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 current_user query string true "current user name"
// @Param		 category path string true "tag category" Enums(task, license, framework, language, industry)
// @Param		 tags body []string true "tag names in array"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/tags/{category} [post]
func (h *RepoHandler) UpdateTags(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed update tags", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var tags []string
	if err := ctx.ShouldBindJSON(&tags); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal tags: %w", err).Error())
		return
	}
	category := ctx.Param("category")
	repoType := common.RepoTypeFromContext(ctx)
	err = h.c.UpdateTags(ctx.Request.Context(), namespace, name, repoType, category, currentUser, tags)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update tags", slog.String("error", err.Error()), slog.String("category", category), slog.String("repo_type", string(repoType)), slog.String("namespace", namespace), slog.String("name", name))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// GetRepoTree godoc
// @Security     ApiKey
// @Summary      Get repository file tree
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		namespace path string true "repo owner name"
// @Param		name path string true "repo name"
// @Param        path query string false "root dir"
// @Param        ref query string false "branch or tag"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/tree [get]
func (h *RepoHandler) Tree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetFileReq{
		Namespace:   namespace,
		Name:        name,
		Path:        ctx.Query("path"),
		Ref:         ctx.Query("ref"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	tree, err := h.c.Tree(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo file tree", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo file tree succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, tree)
}

// GetTreeV2 godoc
// @Security     ApiKey
// @Summary      Get file tree
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param	 namespace path string true "repo owner name"
// @Param	 name path string true "repo name"
// @Param        path path string false "dir to list"
// @Param        ref path string false "branch or tag"
// @Param        limit query int false "limit of records return"
// @Param        cursor query string false "pagination cursor"
// @Success      200  {object}  types.ResponseWithTotal{data=types.GetRepoFileTreeResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/refs/{ref}/tree/{path} [get]
func (h *RepoHandler) TreeV2(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetTreeRequest{
		Namespace:   namespace,
		Name:        name,
		Path:        ctx.Param("path"),
		Ref:         ctx.Param("ref"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
		Limit:       cast.ToInt(ctx.Query("limit")),
		Cursor:      ctx.Query("cursor"),
	}
	tree, err := h.c.TreeV2(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		} else if errors.Is(err, errorx.ErrGitCommitNotFound) {
			httpbase.NotFoundError(ctx, err)
		} else if errors.Is(err, errorx.ErrGitFileNotFound) {
			// Do nothing, return empty tree
		} else {
			httpbase.ServerError(ctx, err)
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get tree",
			slog.String("repo_type", string(req.RepoType)),
			slog.Any("error", err), slog.Any("req", req),
		)
		return
	}

	httpbase.OK(ctx, tree)
}

// Get Logs Tree godoc
// @Security     ApiKey
// @Summary      Get last commit for file tree
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param	 namespace path string true "repo owner name"
// @Param	 name path string true "repo name"
// @Param        path path string false "dir to list"
// @Param        ref path string false "branch or tag"
// @Param        limit query int false "limit of records return"
// @Param        offset query int false "pagination offset"
// @Success      200  {object}  types.ResponseWithTotal{data=types.LogsTreeResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/refs/{ref}/logs_tree/{path} [get]
func (h *RepoHandler) LogsTree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetLogsTreeRequest{
		Namespace:   namespace,
		Name:        name,
		Path:        ctx.Param("path"),
		Ref:         ctx.Param("ref"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
		Limit:       cast.ToInt(ctx.Query("limit")),
		Offset:      cast.ToInt(ctx.Query("offset")),
	}

	if req.Limit == 0 {
		req.Limit = 25
	}
	tree, err := h.c.LogsTree(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(),
			"Failed to get logs tree",
			slog.String("repo_type", string(req.RepoType)), slog.Any("error", err),
			slog.Any("req", req),
		)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, tree)
}

func (h *RepoHandler) UpdateDownloads(ctx *gin.Context) {
	var req *types.UpdateDownloadsReq
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Namespace = namespace
	req.Name = name
	req.RepoType = common.RepoTypeFromContext(ctx)
	date, err := time.Parse("2006-01-02", req.ReqDate)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Date = date

	err = h.c.UpdateDownloads(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update repo download count", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.Time("date", date), slog.Int64("clone_count", req.CloneCount), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug("Update repo download count succeed", slog.String("repo_type", string(req.RepoType)), slog.String("namespace", namespace), slog.String("name", name), slog.Int64("clone_count", req.CloneCount))
	httpbase.OK(ctx, nil)
}

// IncrDownloads godoc
// @Security     ApiKey
// @Summary      Increase repo download count by 1
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		namespace path string true "repo owner name"
// @Param		name path string true "repo name"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/incr_downloads [put]
func (h *RepoHandler) IncrDownloads(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)

	err = h.c.IncrDownloads(ctx.Request.Context(), repoType, namespace, name)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to increase repo download count", slog.String("repo_type", string(repoType)), slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug("increase repo download count succeed", slog.String("repo_type", string(repoType)), slog.String("namespace", namespace), slog.String("name", name))
	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) UploadFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	form, _ := ctx.MultipartForm()
	fileList := form.File["file"]
	paths := form.Value["file_path"]
	var message, branch string
	if len(form.Value["message"]) > 0 {
		message = form.Value["message"][0]
	}
	if len(form.Value["branch"]) > 0 {
		branch = form.Value["branch"][0]
	}
	var buf bytes.Buffer
	for idx, file := range fileList {
		openedFile, err := file.Open()
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Error opening uploaded file", "error", err)
			httpbase.BadRequest(ctx, err.Error())
			return
		}

		buf.Reset()
		_, err = io.Copy(&buf, openedFile)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Error encodeing uploaded file", "error", err, slog.String("file_name", file.Filename))
			httpbase.BadRequest(ctx, err.Error())
			return
		}
		originalBytes := buf.Bytes()
		base64Content := base64.StdEncoding.EncodeToString(originalBytes)
		openedFile.Close()
		filePath := paths[idx]

		upload := &types.CreateFileReq{
			Username:        userName,
			Namespace:       namespace,
			Name:            name,
			FilePath:        filePath,
			Content:         base64Content,
			RepoType:        common.RepoTypeFromContext(ctx),
			Message:         message,
			Branch:          branch,
			OriginalContent: originalBytes,
			CurrentUser:     userName,
		}
		err = h.c.UploadFile(ctx.Request.Context(), upload)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Failed to upload repo file", slog.String("repo_type", string(upload.RepoType)), slog.Any("error", err), slog.String("file_path", filePath))
			httpbase.ServerError(ctx, err)
			return
		}
		slog.Debug("Upload file succeed", slog.String("repo_type", string(upload.RepoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)), slog.String("file_name", file.Filename))
	}

	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) SDKListFiles(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ref := ctx.Param("ref")
	mappedBranch := ctx.Param("branch_mapped")
	if mappedBranch != "" {
		ref = mappedBranch
	}
	repoType := common.RepoTypeFromContext(ctx)
	expand := ctx.Query("expand")
	if expand == "xetEnabled" {
		resp, err := h.c.IsXnetEnabled(ctx.Request.Context(), repoType, namespace, name, currentUser)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "failed to check if xnetEnabled", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, resp)
		return
	}

	files, err := h.c.SDKListFiles(ctx.Request.Context(), repoType, namespace, name, ref, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrUnauthorized) {
			slog.ErrorContext(ctx.Request.Context(), "permission denied when accessing repo", slog.String("repo_type", string(repoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrNotFound) {
			slog.ErrorContext(ctx.Request.Context(), "repo not found", slog.String("repo_type", string(repoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Error listing repo files", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, files)
}

func (h *RepoHandler) SDKDownload(ctx *gin.Context) {
	h.handleDownload(ctx, false)
}

// DownloadRepoFile godoc
// @Security     ApiKey
// @Summary      Download a rep file
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 file_path path string true "file path"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=string} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/resolve/{file_path} [get]
func (h *RepoHandler) ResolveDownload(ctx *gin.Context) {
	h.handleDownload(ctx, true)
}

func (h *RepoHandler) HeadSDKDownload(ctx *gin.Context) {
	var repoCommit string
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	branch := ctx.Param("branch")
	mappedBranch := ctx.Param("branch_mapped")
	if mappedBranch != "" {
		branch = mappedBranch
	}
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      filePath,
		Ref:       branch,
		Lfs:       false,
		SaveAs:    filepath.Base(filePath),
		RepoType:  common.RepoTypeFromContext(ctx),
	}

	file, commit, err := h.c.HeadDownloadFile(ctx.Request.Context(), req, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrUnauthorized) {
			slog.ErrorContext(ctx.Request.Context(), "permission denied when accessing repo head", slog.String("repo_type", string(req.RepoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.UnauthorizedError(ctx, err)
			return
		}

		if errors.Is(err, errorx.ErrNotFound) {
			slog.ErrorContext(ctx.Request.Context(), "repo not found head", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Failed to download repo file head", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	if commit != nil {
		repoCommit = commit.ID
	}

	slog.Debug("Head download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref), slog.Int64("contentLength", file.Size))
	if file.Lfs && file.XnetEnabled {
		ctx.Header("X-Xet-Hash", file.LfsSHA256)
		ctx.Header("X-Xet-Refresh-Route", h.xetRefreshRoute(req.RepoType, namespace, name, branch))
	}
	ctx.Header("Content-Length", strconv.Itoa(int(file.Size)))
	ctx.Header("X-Repo-Commit", repoCommit)
	ctx.Header("ETag", file.SHA)
	ctx.Status(http.StatusOK)
}

func (h *RepoHandler) xetRefreshRoute(repoType types.RepositoryType, namespace, name, ref string) string {
	if repoType == types.ModelRepo {
		return fmt.Sprintf("%s/hf/%s/%s/xet-write-token/%s", h.config.Model.DownloadEndpoint, namespace, name, ref)
	}
	return fmt.Sprintf("%s/hf/%ss/%s/%s/xet-write-token/%s", h.config.Model.DownloadEndpoint, repoType, namespace, name, ref)

}

func (h *RepoHandler) handleDownload(ctx *gin.Context, isResolve bool) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var (
		namespace     string
		name          string
		branch        string
		reader        io.ReadCloser
		size          int64
		url           string
		contentLength int64
		err           error
	)
	namespace, name, err = common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	if isResolve {
		branch = ctx.Query("ref")
	} else {
		branch = ctx.Param("branch")
	}
	mappedBranch := ctx.Param("branch_mapped")
	if mappedBranch != "" {
		branch = mappedBranch
	}

	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      filePath,
		Ref:       branch,
		Lfs:       false,
		SaveAs:    filepath.Base(filePath),
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	// TODO:move the check into SDKDownloadFile, and can return the file content as we get all the content before check lfs pointer
	lfs, contentLength, err := h.c.IsLfs(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrNotFound) {
			slog.ErrorContext(ctx.Request.Context(), "repo not found", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Filed to lfs information", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	req.Lfs = lfs

	if contentLength > 0 {
		// file content is not empty, download it directly
		reader, size, url, err = h.c.SDKDownloadFile(ctx.Request.Context(), req, currentUser)
		if err != nil {
			if errors.Is(err, errorx.ErrUnauthorized) {
				slog.ErrorContext(ctx.Request.Context(), "permission denied when accessing repo", slog.String("repo_type", string(req.RepoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
				httpbase.UnauthorizedError(ctx, err)
				return
			}

			if errors.Is(err, errorx.ErrNotFound) {
				slog.ErrorContext(ctx.Request.Context(), "repo not found", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
				httpbase.NotFoundError(ctx, err)
				return
			}

			slog.ErrorContext(ctx.Request.Context(), "Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}
	}

	if req.Lfs {
		ctx.Redirect(http.StatusMovedPermanently, url)
	} else {
		slog.Debug("Download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref), slog.Any("Content-Length", size))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		ctx.Header("Content-Length", strconv.FormatInt(size, 10))
		if contentLength > 0 {
			_, err = io.Copy(ctx.Writer, reader)
		}
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}
	}
}

// GetRepoCommitDiff godoc
// @Security     ApiKey
// @Summary      Get commit diff of repository and data field of response need to be decode with base64
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 commit_id path string true "commit id"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.Response{data=types.CommitResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/commit/{commit_id} [get]
func (h *RepoHandler) CommitWithDiff(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	commitID := ctx.Param("commit_id")
	req := &types.GetCommitsReq{
		Namespace:   namespace,
		Name:        name,
		Ref:         commitID,
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	commit, err := h.c.GetCommitWithDiff(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo with commit diff", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		if errors.Is(err, errorx.ErrGitCommitNotFound) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}
	slog.Debug("Get repo commit with diff succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("commit id", req.Ref))
	// client need base64 decode for diff, for example: echo <diff> | base64 -d
	httpbase.OK(ctx, commit)
}

// CreateMirror godoc
// @Security     ApiKey
// @Summary      Create mirror for a existing repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        body body types.CreateMirrorParams true "body"
// @Success      200  {object}  types.Response{data=database.Mirror} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror [post]
func (h *RepoHandler) CreateMirror(ctx *gin.Context) {
	var mirrorReq types.CreateMirrorReq
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBindJSON(&mirrorReq); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	sourceRepoPath, err := getSourceRepoPathFromSourceUrl(mirrorReq.SourceUrl)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	mirrorReq.SourceRepoPath = sourceRepoPath
	mirror, err := h.c.CreateMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create mirror for", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, mirror)
}

// GetMirror godoc
// @Security     ApiKey
// @Summary      Get a mirror
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{data=database.Mirror} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror [get]
func (h *RepoHandler) GetMirror(ctx *gin.Context) {
	var mirrorReq types.GetMirrorReq
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	mirror, err := h.c.GetMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get mirror of", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, mirror)
}

// UpdateMirror godoc
// @Security     ApiKey
// @Summary      Update a mirror for a existing repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        body body types.UpdateMirrorParams true "body"
// @Success      200  {object}  types.Response{data=database.Mirror} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror [put]
func (h *RepoHandler) UpdateMirror(ctx *gin.Context) {
	var mirrorReq types.UpdateMirrorReq
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBindJSON(&mirrorReq); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	sourceRepoPath, err := getSourceRepoPathFromSourceUrl(mirrorReq.SourceUrl)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.SourceRepoPath = sourceRepoPath
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	mirror, err := h.c.UpdateMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update mirror for", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, mirror)
}

// DeleteMirror godoc
// @Security     ApiKey
// @Summary      Delete a mirror
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror [delete]
func (h *RepoHandler) DeleteMirror(ctx *gin.Context) {
	var mirrorReq types.DeleteMirrorReq
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	err = h.c.DeleteMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete mirror of", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// RuntimeFramework godoc
// @Security     ApiKey
// @Summary      [Deprecated: use GET:/{repo_type}/{namespace}/{name}/runtime_framework_v2 instead]
// @Description  List repo runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2, 4) default(1)
// @Success      200  {object}  types.Response{data=[]types.RuntimeFramework} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework [get]
func (h *RepoHandler) RuntimeFrameworkList(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	slog.Debug("list runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.ErrorContext(ctx.Request.Context(), "Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFramework(ctx.Request.Context(), repoType, namespace, name, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// RuntimeFramework godoc
// @Security     ApiKey
// @Summary      List repo runtime framework
// @Description  List repo runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2, 4) default(1)
// @Success      200  {object}  types.Response{data=[]types.RuntimeFrameworkV2} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework_v2 [get]
func (h *RepoHandler) RuntimeFrameworkListV2(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	slog.Debug("list runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.ErrorContext(ctx.Request.Context(), "Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFrameworkV2(ctx.Request.Context(), repoType, namespace, name, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// CreateRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Create runtime framework
// @Description  create runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        body body types.RuntimeFrameworkReq true "body"
// @Success      200  {object}  types.RuntimeFramework "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework [post]
func (h *RepoHandler) RuntimeFrameworkCreate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.RuntimeFrameworkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CurrentUser = currentUser
	frame, err := h.c.CreateRuntimeFramework(ctx.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to create runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, frame)
}

// UpdateRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Update runtime framework
// @Description  Update runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        current_user query string false "current user"
// @Param        body body types.RuntimeFrameworkReq true "body"
// @Success      200  {object}  types.RuntimeFramework "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id} [put]
func (h *RepoHandler) RuntimeFrameworkUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.RuntimeFrameworkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CurrentUser = currentUser
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request url format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	frame, err := h.c.UpdateRuntimeFramework(ctx.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, frame)
}

// DeleteRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Delete a exist RuntimeFramework
// @Description  delete a exist RuntimeFramework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id} [delete]
func (h *RepoHandler) RuntimeFrameworkDelete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	err = h.c.DeleteRuntimeFramework(ctx.Request.Context(), currentUser, id)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeployList godoc
// @Security     ApiKey
// @Summary      List repo deploys
// @Description  List repo deploys
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=[]types.DeployRepo} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run [get]
func (h *RepoHandler) DeployList(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	response, err := h.c.ListDeploy(ctx.Request.Context(), repoType, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list deploy", slog.String("error", err.Error()), slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// DeployDetail  godoc
// @Security     ApiKey
// @Summary      Get repo deploy detail
// @Description  Get repo deploy detail
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.DeployRepo} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id} [get]
func (h *RepoHandler) DeployDetail(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	detailReq := types.DeployActReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
	}
	switch repoType {
	case types.SpaceRepo:
		detailReq.DeployType = types.SpaceType
	case types.ModelRepo:
		detailReq.DeployType = types.InferenceType
	}

	response, err := h.c.DeployDetail(ctx.Request.Context(), detailReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to get deploy detail", slog.Any("error", err), slog.Any("req", detailReq))
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "failed to get deploy detail", slog.Any("error", err), slog.Any("req", detailReq))
			httpbase.ServerError(ctx, err)
		}
		return
	}

	httpbase.OK(ctx, response)
}

// GetInferenceLogs   godoc
// @Security     ApiKey
// @Summary      get deploy instance logs
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        instance path string true "instance"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id}/logs/{instance} [get]
func (h *RepoHandler) DeployInstanceLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	instance := ctx.Param("instance")
	if len(instance) < 1 {
		httpbase.UnauthorizedError(ctx, errors.New("fail to get deploy instance"))
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.InferenceType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Warn("not allowed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}

// FinetuneInstanceLogs      godoc
// @Security     ApiKey
// @Summary      get finetune instance logs
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        instance path string true "instance"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/finetune/{id}/logs/{instance} [get]
func (h *RepoHandler) FinetuneInstanceLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	instance := ctx.Param("instance")
	if len(instance) < 1 {
		httpbase.UnauthorizedError(ctx, errors.New("fail to get deploy instance"))
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.FinetuneType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Warn("not allowed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}

func (h *RepoHandler) testLogs(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		default:
			ctx.SSEvent("Container", "test run log message")
			ctx.Writer.Flush()
		}
		time.Sleep(time.Second * 5)
	}
}

func getSourceRepoPathFromSourceUrl(sourceUrl string) (string, error) {
	parsedURL, err := url.Parse(sourceUrl)
	if err != nil {
		return "", err
	}

	// Remove leading and trailing slashes
	path := strings.Trim(parsedURL.Path, "/")

	// Remove ".git" suffix
	path = strings.TrimSuffix(path, ".git")

	return path, nil
}

// GetDeployStatus   godoc
// @Security     JWT token
// @Summary      get deploy status
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id}/status [get]
func (h *RepoHandler) DeployStatus(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testStatus(ctx)
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	statusReq := types.DeployActReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.InferenceType,
	}

	allow, err := h.c.AllowAccessDeploy(ctx.Request.Context(), statusReq)
	if err != nil {

		if errors.Is(err, errorx.ErrForbidden) {
			slog.Warn("not allowed to get deploy status", slog.Any("error", err), slog.Any("req", statusReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to get deploy status", "error", err, "req", statusReq)
		httpbase.ServerError(ctx, err)
		return
	}

	if !allow {
		slog.ErrorContext(ctx.Request.Context(), "not allowed to query deploy status", "req", statusReq)
		httpbase.ForbiddenError(ctx, err)
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(h.deployStatusCheckInterval)
			// user http request context instead of gin context, so that server knows the life cycle of the request
			status, err := h.c.DeployStatus(ctx.Request.Context(), repoType, namespace, name, deployID)
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "failed to get deploy status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name), slog.Any("deploy_id", deployID))
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)

			}
			ctx.Writer.Flush()
		}
	}
}

// SyncMirror godoc
// @Security     ApiKey
// @Summary      Triggers the mirror synchronization
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror/sync [post]
func (h *RepoHandler) SyncMirror(ctx *gin.Context) {
	repoType := common.RepoTypeFromContext(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	err = h.c.SyncMirror(ctx.Request.Context(), repoType, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "not allowed to sync mirror", slog.Any("error", err), slog.String("repo_type", string(repoType)), slog.String("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Failed to sync mirror for", slog.String("repo_type", string(repoType)), slog.String("path", fmt.Sprintf("%s/%s", namespace, name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) testStatus(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Building")
			ctx.Writer.Flush()
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Running")
			ctx.Writer.Flush()
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Sleeping")
			ctx.Writer.Flush()
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Stopped")
			ctx.Writer.Flush()
		}
	}
}

// DeployUpdate  godoc
// @Security     ApiKey
// @Summary      Update deploy parameters
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Param        body body types.DeployUpdateReq true "deploy setting of inference"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id} [put]
func (h *RepoHandler) DeployUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	allow, err := h.c.AllowReadAccess(ctx.Request.Context(), types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", "error", err, slog.Any("currentUser", currentUser), slog.Any("namespace", name), slog.Any("name", name))
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Warn("user not allowed to update deploy", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user is not authorized to read this repository for update deploy"))
		return
	}

	var req *types.DeployUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err, slog.Any("request.body", ctx.Request.Body))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica != nil && req.MaxReplica != nil {
		err = Validate.Struct(req)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Bad request setting for deploy", slog.Any("req", *req), slog.Any("err", err))
			httpbase.BadRequest(ctx, fmt.Sprintf("Bad request setting for deploy, %v", err))
			return
		}
	}

	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err), slog.Any("id", ctx.Param("id")))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	updateReq := types.DeployActReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.InferenceType,
	}
	err = h.c.DeployUpdate(ctx.Request.Context(), updateReq, req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to update deploy", slog.String("namespace", namespace),
				slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "failed to update deploy", slog.String("namespace", namespace), slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID), slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to update deploy, %w", err))
		return
	}

	httpbase.OK(ctx, nil)
}

// RuntimeFrameworkListWithType godoc
// @Security     ApiKey
// @Summary      List repo runtime framework
// @Description  List repo runtime framework
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Success      200  {object}  types.Response{data=[]types.RuntimeFramework} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/runtime_framework [get]
func (h *RepoHandler) RuntimeFrameworkListWithType(ctx *gin.Context) {
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.ErrorContext(ctx.Request.Context(), "Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFrameworkWithType(ctx.Request.Context(), deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// serverlessDetail  godoc
// @Security     ApiKey
// @Summary      Get repo serverless detail
// @Description  Get repo serverless detail
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.DeployRepo} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id} [get]
func (h *RepoHandler) ServerlessDetail(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	detailReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.ServerlessType,
	}

	response, err := h.c.DeployDetail(ctx.Request.Context(), detailReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy detail", slog.String("namespace", namespace),
				slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID))
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "fail to get serverless deploy detail", slog.String("error", err.Error()), slog.Any("namespace", namespace), slog.Any("name", name), slog.Any("deploy id", deployID))
			httpbase.ServerError(ctx, err)
		}
		return
	}

	httpbase.OK(ctx, response)
}

// GetServerlessLogs   godoc
// @Security     ApiKey
// @Summary      get serverless logs
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        instance path string true "instance"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request. May occur when the since time format is unsupported"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id}/logs/{instance} [get]
func (h *RepoHandler) ServerlessLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	instance := ctx.Param("instance")
	if len(instance) < 1 {
		httpbase.UnauthorizedError(ctx, errors.New("fail to get deploy instance"))
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.ServerlessType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Failed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}

// GetServerlessStatus   godoc
// @Security     JWT token
// @Summary      get serverless status
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id}/status [get]
func (h *RepoHandler) ServerlessStatus(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testStatus(ctx)
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	statusReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.ServerlessType,
	}

	allow, err := h.c.AllowAccessDeploy(ctx.Request.Context(), statusReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy status", slog.Any("error", err), slog.Any("req", statusReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", slog.Any("error", err), slog.Any("req", statusReq))
		httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission, %w", err))
		return
	}

	if !allow {
		slog.Warn("user not allowed to query deploy status", slog.Any("req", statusReq))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to query serverless deploy status"))
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(h.deployStatusCheckInterval)
			// user http request context instead of gin context, so that server knows the life cycle of the request
			status, err := h.c.DeployStatus(ctx.Request.Context(), types.ModelRepo, namespace, name, deployID)
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "failed to get deploy status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name), slog.Any("deploy_id", deployID))
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)

			}
			ctx.Writer.Flush()
		}
	}
}

// ServerlessUpdate  godoc
// @Security     ApiKey
// @Summary      Update serverless parameters
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Param        body body types.DeployUpdateReq true "deploy setting of Serverless"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id} [put]
func (h *RepoHandler) ServerlessUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req *types.DeployUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err, slog.Any("request.body", ctx.Request.Body))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica != nil && req.MaxReplica != nil {
		err = Validate.Struct(req)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Bad request setting for serverless", slog.Any("req", *req), slog.Any("err", err))
			httpbase.BadRequest(ctx, fmt.Sprintf("Bad request setting for serverless, %v", err))
			return
		}
	}

	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err), slog.Any("id", ctx.Param("id")))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	updateReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.ServerlessType,
	}
	err = h.c.DeployUpdate(ctx.Request.Context(), updateReq, req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Debug("user not allowed to update serverless", slog.Any("error", err), slog.Any("req", updateReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to update serverless", slog.Any("error", err), slog.Any("req", updateReq))
		httpbase.ServerError(ctx, fmt.Errorf("failed to update serverless, %w", err))
		return
	}

	httpbase.OK(ctx, nil)
}

// GetRemoteTree godoc
// @Security     ApiKey
// @Summary      Get file tree
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param	     namespace path string true "repo owner name"
// @Param	     name path string true "repo name"
// @Param        path path string false "dir to list"
// @Param        ref path string false "branch or tag"
// @Param        limit query int false "limit of records return"
// @Param        cursor query string false "pagination cursor"
// @Success      200  {object}  types.ResponseWithTotal{data=types.GetRepoFileTreeResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/refs/{ref}/remote_tree/{path} [get]
func (h *RepoHandler) RemoteTree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetTreeRequest{
		Namespace:   namespace,
		Name:        name,
		Path:        ctx.Param("path"),
		Ref:         ctx.Param("ref"),
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
		Limit:       cast.ToInt(ctx.Query("limit")),
		Cursor:      ctx.Query("cursor"),
	}
	tree, err := h.c.RemoteTree(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(),
			"Failed to get remote tree", slog.String("repo_type", string(req.RepoType)),
			slog.Any("error", err), slog.Any("req", req),
		)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, tree)
}

// GetRepoCommitDiff godoc
// @Security     ApiKey
// @Summary      Get commit diff of repository and data field of response need to be decode with base64
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        current_user query string false "current user name"
// @Param        left_commit_id query string true "previous commit id"
// @Param        right_commit_id query string false "current commit id"
// @Success      200  {object}  types.Response{data=[]types.GiteaCallbackPushReq_Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/diff [get]
func (h *RepoHandler) DiffBetweenTwoCommits(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	leftCommitId := ctx.Query("left_commit_id")
	rightCommitId := ctx.Query("right_commit_id")

	if leftCommitId == "" {
		httpbase.BadRequest(ctx, "left_commit_id is required")
		return
	}
	req := types.GetDiffBetweenCommitsReq{
		Namespace:     namespace,
		Name:          name,
		RepoType:      common.RepoTypeFromContext(ctx),
		LeftCommitID:  leftCommitId,
		RightCommitID: rightCommitId,
		CurrentUser:   currentUser,
	}
	diff, err := h.c.DiffBetweenTwoCommits(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(),
			"Failed to get repo diff between two commits",
			slog.String("repo_type", string(req.RepoType)),
			slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug(
		"Get repo commit with diff succeed",
		slog.String("repo_type", string(req.RepoType)),
		slog.String("name", name),
		slog.String("left commit id", leftCommitId),
		slog.String("right commit id", rightCommitId))
	httpbase.OK(ctx, diff)
}

// RepoAllFiles      godoc
// @Security     ApiKey
// @Summary      Get all files of a repo
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=[]types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/all_files [get]
func (h *RepoHandler) AllFiles(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req types.GetAllFilesReq
	req.Namespace = namespace
	req.Name = name
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Limit = 100
	req.Cursor = ctx.Query("cursor")
	detail, err := h.c.AllFiles(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to get repo all files", slog.Any("error", err), slog.Any("req", req))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo all files", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail.Files)
}

// GetRepoRemoteCommitDiff godoc
// @Security     ApiKey
// @Summary      Get diff between local last commit and remote commit
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        current_user query string false "current user name"
// @Param        left_commit_id query string true "previous commit id"
// @Success      200  {object}  types.Response{data=[]types.RemoteDiffs} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/remote_diff [get]
func (h *RepoHandler) RemoteDiff(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	leftCommitId := ctx.Query("left_commit_id")

	if leftCommitId == "" {
		httpbase.BadRequest(ctx, "left_commit_id is required")
		return
	}
	namespace = strings.TrimPrefix(namespace, types.OpenCSGPrefix)
	req := types.GetDiffBetweenCommitsReq{
		Namespace:    namespace,
		Name:         name,
		RepoType:     common.RepoTypeFromContext(ctx),
		LeftCommitID: leftCommitId,
		CurrentUser:  currentUser,
	}
	diff, err := h.c.RemoteDiff(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(),
			"Failed to get repo remote diff",
			slog.String("repo_type", string(req.RepoType)),
			slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug(
		"Get repo remote diff succeed",
		slog.String("repo_type", string(req.RepoType)),
		slog.String("name", name),
		slog.String("left commit id", leftCommitId))
	httpbase.OK(ctx, diff)
}

// RepoPreupload godoc
// @Security     ApiKey
// @Summary      Get upload mode for files
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        revision path string true "revision"
// @Param        current_user query string false "current user name"
// @Param        body body types.PreuploadReq true "body"
// @Success      200  {object}  types.Response{data=types.PreuploadResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/preupload/{revision} [post]
func (h *RepoHandler) Preupload(ctx *gin.Context) {
	var req types.PreuploadReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.Revision = ctx.Param("revision")

	resp, err := h.c.Preupload(ctx.Request.Context(), req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

func (h *RepoHandler) PreuploadHF(ctx *gin.Context) {
	var req types.PreuploadReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.Revision = ctx.Param("revision")

	resp, err := h.c.Preupload(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to preupload", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// CommitFiles godoc
// @Security     ApiKey
// @Summary      Create commit with batch files
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        revision path string true "revision"
// @Param        current_user query string false "current user name"
// @Param        body body types.CommitFilesReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/commit/{revision} [post]
func (h *RepoHandler) CommitFiles(ctx *gin.Context) {
	var req types.CommitFilesReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.Revision = ctx.Param("revision")

	err = h.c.CommitFiles(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to commit files", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) CommitFilesHF(ctx *gin.Context) {
	req, err := h.c.ParseNDJson(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.Revision = ctx.Param("revision")
	if req.Message == "" {
		req.Message = "initial commit"
	}

	err = h.c.CommitFiles(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to commit files", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"commitUrl": fmt.Sprintf("%s/%s", namespace, name),
		"commitOid": "",
	})
}

// ChangePath    godoc
// @Security     ApiKey
// @Summary      Change the namespace of a repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        body body types.ChangePathReq true "deploy setting of inference"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /admin/{repo_type}/{namespace}/{name}/change_path [post]
func (h *RepoHandler) ChangePath(ctx *gin.Context) {
	var (
		req types.ChangePathReq
		err error
	)
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.RepoType, err = common.RepoTypeFromString(ctx.Param("repo_type"))
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid repo type", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = validator.ValidateRepoPath(req.NewPath)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid new path", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.c.ChangePath(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrBadRequest) {
			slog.ErrorContext(ctx.Request.Context(), "invalid request", slog.Any("error", err))
			httpbase.BadRequest(ctx, err.Error())
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "failed to commit files", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetRepos godoc
// @Security     ApiKey
// @Summary      Get repo paths with search query
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user name"
// @Param        search query string true "search query"
// @Param        type query string true "repository type query" enums(model, dataset, code, space, mcpserver)
// @Success      200  {object}  types.Response{data=[]string} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /admin/repos [get]
func (h *RepoHandler) GetRepos(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	search := ctx.Query("search")
	repositoryType := ctx.Query("type")
	repoType := types.RepositoryType(repositoryType)
	if repoType == types.UnknownRepo {
		httpbase.BadRequest(ctx, "Unknown repository type")
		return
	}

	repos, err := h.c.GetRepos(ctx.Request.Context(), search, currentUser, repoType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(),
			"Failed to get repos",
			slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug(
		"Get repos succeed",
		slog.String("search", search))
	httpbase.OK(ctx, repos)
}

// GetInferenceLogsByVersion   godoc
// @Security     ApiKey
// @Summary      get serverless logs by version (commitid)
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        commit_id path string true "commit_id"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/serverless/{id}/versions/{commit_id} [get]
func (h *RepoHandler) ServerlessVersionLogs(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.NotFoundError(ctx, err)
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, "Invalid deploy ID format")
		return
	}
	commitID := ctx.Param("commit_id")
	instance := ctx.Query("instance_name")
	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.ServerlessType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
		CommitID:     commitID,
	}

	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Failed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			time.Sleep(time.Second * 1)
		}
	}
}
