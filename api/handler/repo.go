package handler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
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
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/component/validator"
)

// escapeCommitMessageForOutput returns a copy with an HTML-escaped commit message for output only.
func escapeCommitMessageForOutput(commit types.Commit) types.Commit {
	commit.Message = html.EscapeString(commit.Message)
	return commit
}

// escapeCommitResponseMessageForOutput returns a shallow copy with an HTML-escaped commit message for output only.
func escapeCommitResponseMessageForOutput(commit *types.CommitResponse) *types.CommitResponse {
	if commit == nil {
		return nil
	}
	escapedCommit := *commit
	if commit.Commit != nil {
		commitCopy := escapeCommitMessageForOutput(*commit.Commit)
		escapedCommit.Commit = &commitCopy
	}
	return &escapedCommit
}

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
	mirror, err := component.NewMirrorComponent(config)
	if err != nil {
		return nil, err
	}
	return &RepoHandler{
		c:                         uc,
		m:                         m,
		d:                         d,
		mirror:                    mirror,
		temporal:                  temporal.GetClient(),
		deployStatusCheckInterval: time.Duration(config.Model.DeployStatusCheckInterval) * time.Second,
		config:                    config,
	}, nil
}

type RepoHandler struct {
	c                         component.RepoComponent
	m                         component.ModelComponent
	d                         component.DatasetComponent
	mirror                    component.MirrorComponent
	temporal                  temporal.Client
	deployStatusCheckInterval time.Duration
	config                    *config.Config
}

// ScanIndustryTags godoc
// @Security     ApiKey
// @Summary      Trigger repository industry tag scan
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "repo type" Enums(dataset,model)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /admin/{repo_type}/{namespace}/{name}/industry_tags/scan [post]
func (h *RepoHandler) ScanIndustryTags(ctx *gin.Context) {
	repoType, err := parseIndustryScanRepoType(ctx.Param("repo_type"))
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
		ID:        fmt.Sprintf("repo-industry-scan-%s-%s-%s", repoType, namespace, name),
	}

	_, err = h.temporal.ExecuteWorkflow(ctx.Request.Context(), workflowOptions, workflow.ScanRepoIndustryTagsWorkflow, types.ScanRepoIndustryTagsReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  repoType,
	})
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to trigger repo industry tag scan", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.String("repo_type", string(repoType)))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

func parseIndustryScanRepoType(raw string) (types.RepositoryType, error) {
	switch types.RepositoryType(raw) {
	case types.DatasetRepo, types.ModelRepo:
		return types.RepositoryType(raw), nil
	default:
		return "", fmt.Errorf("unsupported repo type: %s", raw)
	}
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
	stdCtx := common.GinContextToStdContext(ctx)
	switch req.RepoType {
	case types.ModelRepo:
		modelReq := &types.CreateModelReq{
			CreateRepoReq: *req,
		}
		resp, err := h.m.Create(stdCtx, modelReq)
		if err != nil {
			if errors.Is(err, errorx.ErrRepoAlreadyExist) || errors.Is(err, errorx.ErrSpaceNameAlreadyExist) || strings.Contains(err.Error(), "duplicate key") {
				resp := &types.Model{
					URL: fmt.Sprintf("%s/%s", req.Namespace, req.Name),
				}
				ctx.JSON(http.StatusConflict, resp)
				return
			}
			slog.ErrorContext(stdCtx, "Failed to create model repo", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
			httpbase.ServerError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, resp)
	case types.DatasetRepo:
		datasetReq := &types.CreateDatasetReq{
			CreateRepoReq: *req,
		}
		resp, err := h.d.Create(stdCtx, datasetReq)
		if err != nil {
			if errors.Is(err, errorx.ErrRepoAlreadyExist) || errors.Is(err, errorx.ErrSpaceNameAlreadyExist) || strings.Contains(err.Error(), "duplicate key") {
				resp := &types.Dataset{
					URL: fmt.Sprintf("%s/%s", req.Namespace, req.Name),
				}
				ctx.JSON(http.StatusConflict, resp)
				return
			}
			slog.ErrorContext(stdCtx, "Failed to create dataset repo", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.Any("req", req))
			httpbase.ServerError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, resp)
	default:
		// Unsupported repo type
		slog.ErrorContext(stdCtx, "Unsupported repo type", slog.String("repo_type", string(req.RepoType)))
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
	escapedCommits := make([]types.Commit, 0, len(commits))
	for _, commit := range commits {
		escapedCommits = append(escapedCommits, escapeCommitMessageForOutput(commit))
	}
	resData := gin.H{
		"commits":    escapedCommits,
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
// @Success      202  {object}  httpbase.R{data=types.GetRepoFileTreeResp} "Repository synchronization is in progress; data may be omitted when remote file metadata is unavailable"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      409  {object}  httpbase.R "Repository synchronization failed (MIRROR-ERR-2) or was canceled (MIRROR-ERR-4)"
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
		if isRootTreeGitNotFound(req, err) && h.respondMirrorTreeStatus(ctx, req) {
			return
		}
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

// isRootTreeGitNotFound identifies an unavailable root tree without changing nested-path errors.
func isRootTreeGitNotFound(req *types.GetTreeRequest, err error) bool {
	return strings.Trim(req.Path, "/") == "" &&
		(errors.Is(err, errorx.ErrGitFileNotFound) || errors.Is(err, errorx.ErrGitCommitNotFound))
}

// respondMirrorTreeStatus translates an unavailable root tree only when the current repo phase explains it.
func (h *RepoHandler) respondMirrorTreeStatus(ctx *gin.Context, req *types.GetTreeRequest) bool {
	status, err := h.mirror.MirrorFromSaasStatus(ctx.Request.Context(), types.MirrorFromSaasStatusReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		RepoType:    req.RepoType,
		CurrentUser: req.CurrentUser,
	})
	if err != nil {
		slog.DebugContext(ctx.Request.Context(), "Mirror status does not explain unavailable root tree", "error", err)
		return false
	}
	errContext := errorx.Ctx().Set("task_id", status.TaskID).Set("status", status.Status)
	if !status.RepoReady && !status.Terminal {
		remoteTree, remoteErr := h.c.RemoteTree(ctx.Request.Context(), req)
		if remoteErr != nil {
			slog.DebugContext(ctx.Request.Context(), "Failed to get remote tree while repository is syncing", "error", remoteErr)
		}
		ctx.Header("Retry-After", "2")
		httpbase.AcceptedWithExt(ctx, errorx.MirrorRepoSyncing(
			errors.New("repository synchronization is in progress"), errContext,
		), remoteTree)
		return true
	}
	if !status.RepoReady && status.Terminal {
		if status.FailureReason == types.MirrorSyncFailureCanceled {
			errContext.Set("failure_reason", status.FailureReason)
			httpbase.ConflictError(ctx, errorx.MirrorRepoSyncCanceled(
				errors.New("repository synchronization was canceled"), errContext,
			))
			return true
		}
		httpbase.ConflictError(ctx, errorx.MirrorRepoSyncFailed(
			errors.New("repository synchronization failed"), errContext,
		))
		return true
	}
	return false
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
		action := "read"
		ctx.Header("X-Xet-Hash", file.LfsSHA256)
		ctx.Header("X-Xet-Refresh-Route", h.xetRefreshRoute(req.RepoType, namespace, name, branch, action))
	}
	ctx.Header("Content-Length", strconv.Itoa(int(file.Size)))
	ctx.Header("X-Repo-Commit", repoCommit)
	ctx.Header("ETag", file.SHA)
	ctx.Status(http.StatusOK)
}

func (h *RepoHandler) xetRefreshRoute(repoType types.RepositoryType, namespace, name, ref, action string) string {
	if repoType == types.ModelRepo {
		return fmt.Sprintf("%s/hf/%s/%s/xet-%s-token/%s", h.config.Model.DownloadEndpoint, namespace, name, action, ref)
	}
	return fmt.Sprintf("%s/hf/%ss/%s/%s/xet-%s-token/%s", h.config.Model.DownloadEndpoint, repoType, namespace, name, action, ref)
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
	httpbase.OK(ctx, escapeCommitResponseMessageForOutput(commit))
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
// @Failure      409  {object}  httpbase.R "Target repository already has a different mirror source (MIRROR-ERR-0)"
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
	mirror, err := h.mirror.CreateMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create mirror for", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		if errors.Is(err, errorx.ErrMirrorSourceRepoAuthInvalid) || errors.Is(err, errorx.ErrBadRequest) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrMirrorSourceConflict) {
			httpbase.ConflictError(ctx, err)
			return
		}
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
	mirror, err := h.mirror.GetMirror(ctx.Request.Context(), mirrorReq)
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
	mirror, err := h.mirror.UpdateMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update mirror for", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		if errors.Is(err, errorx.ErrMirrorSourceRepoAuthInvalid) || errors.Is(err, errorx.ErrBadRequest) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
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
	err = h.mirror.DeleteMirror(ctx.Request.Context(), mirrorReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete mirror of", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
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

// SyncMirror godoc
// @Security     ApiKey
// @Summary      Triggers the mirror synchronization
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        body body types.SyncMirrorParams false "scheduling options"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror/sync [post]
func (h *RepoHandler) SyncMirror(ctx *gin.Context) {
	var params types.SyncMirrorParams
	if ctx.Request.Body != nil && ctx.Request.Body != http.NoBody {
		if err := ctx.ShouldBindJSON(&params); err != nil && !errors.Is(err, io.EOF) {
			slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}
	repoType := common.RepoTypeFromContext(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	err = h.mirror.SyncMirror(ctx.Request.Context(), types.SyncMirrorReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		Urgent:      params.Urgent,
	})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		if errors.Is(err, errorx.ErrBadRequest) || errors.Is(err, errorx.ErrChangePathBlocked) || errors.Is(err, errorx.ErrTargetNamespaceNotFound) {
			slog.ErrorContext(ctx.Request.Context(), "invalid request", slog.Any("error", err))
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "failed to change path", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// TransferOwnership godoc
// @Security     ApiKey
// @Summary      Transfer repository ownership to another namespace
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        namespace path string true "current namespace"
// @Param        name path string true "repository name"
// @Param        body body types.TransferRepoReq true "transfer request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/transfer [post]
func (h *RepoHandler) TransferOwnership(ctx *gin.Context) {
	var req types.TransferRepoReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "invalid request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.RepoType = common.RepoTypeFromContext(ctx)

	err = h.c.TransferOwnership(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrNoSourceTransferPermission) ||
			errors.Is(err, errorx.ErrNoTargetTransferPermission) {
			slog.ErrorContext(ctx.Request.Context(), "forbidden to transfer ownership", slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrTransferSameNamespace) ||
			errors.Is(err, errorx.ErrTransferTargetExists) ||
			errors.Is(err, errorx.ErrTransferNotSupported) ||
			errors.Is(err, errorx.ErrBadRequest) ||
			errors.Is(err, errorx.ErrChangePathBlocked) {
			slog.ErrorContext(ctx.Request.Context(), "bad request for transfer ownership", slog.Any("error", err))
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "forbidden to transfer ownership", slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "failed to transfer ownership", slog.Any("error", err))
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

// GetRepoSizeByBranch godoc
// @Security     ApiKey
// @Summary      Get the repository size for a specific branch
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes,spaces or mcps" Enums(models,datasets,codes,spaces,mcps)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        branch path string true "branch name"
// @Param        current_user query string false "current user name"
// @Success      200  {object}  types.Response{data=int64} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/size/{branch} [get]
func (h *RepoHandler) GetRepoSizeByBranch(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	branch := ctx.Param("branch")
	repoType := common.RepoTypeFromContext(ctx)

	resp, err := h.c.GetRepoSizeByBranch(ctx.Request.Context(), repoType, namespace, name, branch, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get repo size", slog.String("repo_type", string(repoType)), slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.String("branch", branch))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Get repo size succeed", slog.String("repo_type", string(repoType)), slog.String("namespace", namespace), slog.String("name", name), slog.String("branch", branch), slog.Any("total_size", resp.TotalSize), slog.Any("last_commit_size", resp.LastCommitSize))
	httpbase.OK(ctx, resp)
}

// BatchGetRepoExtra godoc
// @Security     ApiKey
// @Summary      Batch get extra information for multiple repositories
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user name"
// @Param        body body types.BatchRepoExtraReq true "request body"
// @Success      200  {object}  types.Response{data=[]types.RepoExtraItem} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /repos/extra [post]
func (h *RepoHandler) BatchGetRepoExtra(ctx *gin.Context) {
	var req types.BatchRepoExtraReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	if len(req.RepoIDs) > h.config.MaxRepoBatchNum {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("too many repository ids, max %d, got %d", h.config.MaxRepoBatchNum, len(req.RepoIDs)),
			errorx.Ctx().Set("max", h.config.MaxRepoBatchNum).Set("got", len(req.RepoIDs)),
		))
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	extras, err := h.c.BatchGetRepoExtra(ctx.Request.Context(), req.RepoIDs, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to batch get repo extras", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Debug("Batch get repo extras succeed", slog.Int("count", len(extras)))
	httpbase.OK(ctx, extras)
}

// DownloadCodeZip godoc
// @Summary      Download code repository as zip archive
// @Description  Download code repository as zip archive
// @Tags         Code
// @Produce      application/zip
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Param        ref path string true "branch or tag name"
// @Success      200  {file}  file "OK"
// @Router       /codes/{namespace}/{name}/download_archive/refs/{ref}/ [get]
func (h *RepoHandler) DownloadCodeZip(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	ref := strings.TrimPrefix(ctx.Param("ref"), "/")

	zipData, err := h.c.DownloadCodeZip(ctx.Request.Context(), types.DownloadCodeZipReq{
		Namespace: namespace,
		Name:      name,
		Revision:  ref,
	}, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to download code zip", slog.String("namespace", namespace), slog.String("name", name), slog.Any("error", err))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrRepoNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrRepoNoDefaultBranch) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}

	safeName := strings.ReplaceAll(name, "/", "-")
	safeRef := strings.ReplaceAll(ref, "/", "-")
	filename := fmt.Sprintf("%s-%s.zip", safeName, safeRef)
	if ref == "" {
		filename = fmt.Sprintf("%s.zip", safeName)
	}
	ctx.Header("Content-Type", "application/zip")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	ctx.Data(http.StatusOK, "application/zip", zipData)
}
