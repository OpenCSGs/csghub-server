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
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewRepoHandler(config *config.Config) (*RepoHandler, error) {
	uc, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return &RepoHandler{
		c: uc,
	}, nil
}

type RepoHandler struct {
	c *component.RepoComponent
}

// CreateRepoFile godoc
// @Security     ApiKey
// @Summary      Create a new file in repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
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
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req *types.CreateFileReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = userName

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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

	resp, err := h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, resp)
}

// UpdateRepoFile godoc
// @Security     ApiKey
// @Summary      Update existing file in repository
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,dataset,codes or spaces" Enums(models,datasets,codes,spaces)
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
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req *types.UpdateFileReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = userName

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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

	resp, err := h.c.UpdateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to update repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Update repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
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
	commits, pageOpt, err := h.c.Commits(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo commits", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo commits succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("ref", req.Ref))
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
		slog.Error("Bad request format", "error", err)
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
	commit, err := h.c.LastCommit(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo last commit", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo last commit succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("ref", req.Ref))
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
		slog.Error("Bad request format", "error", err)
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
	raw, err := h.c.FileRaw(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo file raw", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo file raw succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
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
		slog.Error("Bad request format", "error", err)
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
	file, err := h.c.FileInfo(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo file info", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo file info succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
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
		slog.Error("Bad request format", "error", err)
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
			slog.Error("Bad request format", "error", err)
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}
	reader, size, url, err := h.c.DownloadFile(ctx, req, currentUser)
	if err != nil {
		slog.Error("Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if req.Lfs {
		httpbase.OK(ctx, url)
	} else {
		slog.Info("Download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		ctx.Header("Content-Length", strconv.FormatInt(size, 10))
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
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
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetBranchesReq{
		Namespace:   namespace,
		Name:        name,
		Per:         per,
		Page:        page,
		RepoType:    common.RepoTypeFromContext(ctx),
		CurrentUser: currentUser,
	}
	branches, err := h.c.Branches(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo branches", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo branches succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
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
		slog.Error("Bad request format", "error", err)
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
	tags, err := h.c.Tags(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo tags", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo tags succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, httpbase.ErrorNeedLogin)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed update tags", "error", err)
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
	err = h.c.UpdateTags(ctx, namespace, name, repoType, category, currentUser, tags)
	if err != nil {
		slog.Error("Failed to update tags", slog.String("error", err.Error()), slog.String("category", category), slog.String("repo_type", string(repoType)),
			slog.String("namespace", namespace), slog.String("name", name))
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
		slog.Error("Bad request format", "error", err)
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
	tree, err := h.c.Tree(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo file tree", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo file tree succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, tree)
}

func (h *RepoHandler) UpdateDownloads(ctx *gin.Context) {
	var req *types.UpdateDownloadsReq
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

	req.Namespace = namespace
	req.Name = name
	req.RepoType = common.RepoTypeFromContext(ctx)
	date, err := time.Parse("2006-01-02", req.ReqDate)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Date = date

	err = h.c.UpdateDownloads(ctx, req)
	if err != nil {
		slog.Error("Failed to update repo download count", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.Time("date", date), slog.Int64("clone_count", req.CloneCount))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Update repo download count succeed", slog.String("repo_type", string(req.RepoType)), slog.String("namespace", namespace), slog.String("name", name), slog.Int64("clone_count", req.CloneCount))
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
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)

	err = h.c.IncrDownloads(ctx, repoType, namespace, name)
	if err != nil {
		slog.Error("Failed to increase repo download count", slog.String("repo_type", string(repoType)), slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("increase repo download count succeed", slog.String("repo_type", string(repoType)), slog.String("namespace", namespace), slog.String("name", name))
	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) UploadFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get namespace from context", "error", err)
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
			slog.Error("Error opening uploaded file", "error", err)
			httpbase.BadRequest(ctx, err.Error())
			return
		}

		buf.Reset()
		_, err = io.Copy(&buf, openedFile)
		if err != nil {
			slog.Info("Error encodeing uploaded file", "error", err, slog.String("file_name", file.Filename))
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
		err = h.c.UploadFile(ctx, upload)
		if err != nil {
			slog.Error("Failed to upload repo file", slog.String("repo_type", string(upload.RepoType)), slog.Any("error", err), slog.String("file_path", filePath))
			httpbase.ServerError(ctx, err)
			return
		}
		slog.Info("Upload file succeed", slog.String("repo_type", string(upload.RepoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)),
			slog.String("file_name", file.Filename))
	}

	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) SDKListFiles(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	files, err := h.c.SDKListFiles(ctx, common.RepoTypeFromContext(ctx), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			slog.Error("permission denied when accessing repo", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		if errors.Is(err, component.ErrNotFound) {
			slog.Error("repo not found", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.Error("Error listing repo files", "error", err)
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
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	branch := ctx.Param("branch")
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      filePath,
		Ref:       branch,
		Lfs:       false,
		SaveAs:    filepath.Base(filePath),
		RepoType:  common.RepoTypeFromContext(ctx),
	}

	file, err := h.c.HeadDownloadFile(ctx, req, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			slog.Error("permission denied when accessing repo head", slog.String("repo_type", string(req.RepoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.UnauthorizedError(ctx, err)
			return
		}

		if errors.Is(err, component.ErrNotFound) {
			slog.Error("repo not found head", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}

		slog.Error("Failed to download repo file head", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Head download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref),
		slog.Int64("contentLength", file.Size))
	ctx.Header("Content-Length", strconv.Itoa(int(file.Size)))
	ctx.Header("X-Repo-Commit", file.SHA)
	ctx.Header("ETag", file.SHA)
	ctx.Status(http.StatusOK)
}

func (h *RepoHandler) handleDownload(ctx *gin.Context, isResolve bool) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var branch string
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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
	lfs, err := h.c.IsLfs(ctx, req)
	if err != nil {
		if errors.Is(err, component.ErrNotFound) {
			slog.Error("repo not found", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}

		slog.Error("Filed to lfs information", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	req.Lfs = lfs
	reader, size, url, err := h.c.SDKDownloadFile(ctx, req, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			slog.Error("permission denied when accessing repo", slog.String("repo_type", string(req.RepoType)), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.UnauthorizedError(ctx, err)
			return
		}

		if errors.Is(err, component.ErrNotFound) {
			slog.Error("repo not found", slog.String("repo_type", string(common.RepoTypeFromContext(ctx))), slog.Any("path", fmt.Sprintf("%s/%s", namespace, name)))
			httpbase.NotFoundError(ctx, err)
			return
		}

		slog.Error("Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if req.Lfs {
		ctx.Redirect(http.StatusMovedPermanently, url)
	} else {
		slog.Info("Download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		ctx.Header("Content-Length", strconv.FormatInt(size, 10))
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
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
		slog.Error("Bad request format", "error", err)
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
	commit, err := h.c.GetCommitWithDiff(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo with commit diff", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get repo commit with diff succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("commit id", req.Ref))
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBindJSON(&mirrorReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	sourceRepoPath, err := getSourceRepoPathFromSourceUrl(mirrorReq.SourceUrl)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	mirrorReq.SourceRepoPath = sourceRepoPath
	mirror, err := h.c.CreateMirror(ctx, mirrorReq)
	if err != nil {
		slog.Error("Failed to create mirror for", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, mirror)
}

// MirrorFromSaas godoc
// @Security     ApiKey
// @Summary      Mirror repo from OpenCSG Saas(only on-premises)
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,datasets,codes or spaces" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "repo owner name"
// @Param        name path string true "repo name"
// @Success      200  {object}  types.Response{data=database.Mirror} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/mirror_from_saas [post]
func (h *RepoHandler) MirrorFromSaas(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	if !strings.HasPrefix(namespace, types.OpenCSGPrefix) {
		httpbase.BadRequest(ctx, "Repo could not be mirrored")
		return
	}
	err = h.c.MirrorFromSaas(ctx, namespace, name, currentUser, repoType)
	if err != nil {
		slog.Error("Failed to create mirror for", slog.String("repo_type", string(repoType)), slog.String("path", fmt.Sprintf("%s/%s", namespace, name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	mirror, err := h.c.GetMirror(ctx, mirrorReq)
	if err != nil {
		slog.Error("Failed to get mirror of", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBindJSON(&mirrorReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	sourceRepoPath, err := getSourceRepoPathFromSourceUrl(mirrorReq.SourceUrl)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.SourceRepoPath = sourceRepoPath
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	mirror, err := h.c.UpdateMirror(ctx, mirrorReq)
	if err != nil {
		slog.Error("Failed to update mirror for", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mirrorReq.Namespace = namespace
	mirrorReq.Name = name
	mirrorReq.RepoType = common.RepoTypeFromContext(ctx)
	mirrorReq.CurrentUser = currentUser
	err = h.c.DeleteMirror(ctx, mirrorReq)
	if err != nil {
		slog.Error("Failed to delete mirror of", slog.String("repo_type", string(mirrorReq.RepoType)), slog.String("path", fmt.Sprintf("%s/%s", mirrorReq.Namespace, mirrorReq.Name)), "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// RuntimeFramework godoc
// @Security     ApiKey
// @Summary      List repo runtime framework
// @Description  List repo runtime framework
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework [get]
func (h *RepoHandler) RuntimeFrameworkList(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
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
	slog.Debug("list runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.Error("Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFramework(ctx, repoType, namespace, name, deployType)
	if err != nil {
		slog.Error("fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// CreateRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Create runtime framework
// @Description  create runtime framework
// @Tags         Repository
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
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework [post]
func (h *RepoHandler) RuntimeFrameworkCreate(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	slog.Debug("create runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	var req types.RuntimeFrameworkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	frame, err := h.c.CreateRuntimeFramework(ctx, &req)
	if err != nil {
		slog.Error("Failed to create runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, frame)
}

// UpdateRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Update runtime framework
// @Description  Update runtime framework
// @Tags         Repository
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
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework/{id} [put]
func (h *RepoHandler) RuntimeFrameworkUpdate(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	slog.Debug("update runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	var req types.RuntimeFrameworkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request url format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	frame, err := h.c.UpdateRuntimeFramework(ctx, id, &req)
	if err != nil {
		slog.Error("Failed to update runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, frame)
}

// DeleteRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Delete a exist RuntimeFramework
// @Description  delete a exist RuntimeFramework
// @Tags         Repository
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
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework/{id} [delete]
func (h *RepoHandler) RuntimeFrameworkDelete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.c.DeleteRuntimeFramework(ctx, id)
	if err != nil {
		slog.Error("Failed to delete runtime framework", slog.Any("error", err))
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
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run [get]
func (h *RepoHandler) DeployList(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	response, err := h.c.ListDeploy(ctx, repoType, namespace, name, currentUser)
	if err != nil {
		slog.Error("fail to list deploy", slog.String("error", err.Error()), slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
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
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id} [get]
func (h *RepoHandler) DeployDetail(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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
	if repoType == types.SpaceRepo {
		detailReq.DeployType = types.SpaceType
	} else if repoType == types.ModelRepo {
		detailReq.DeployType = types.InferenceType
	}

	response, err := h.c.DeployDetail(ctx, detailReq)
	if err != nil {
		slog.Error("fail to deploy detail", slog.String("error", err.Error()), slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name), slog.Any("deploy id", deployID))
		var pErr *types.PermissionError
		if errors.As(err, &pErr) {
			httpbase.UnauthorizedError(ctx, err)
		} else {
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
		slog.Error("failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		var pErr *types.PermissionError
		if errors.As(err, &pErr) {
			httpbase.UnauthorizedError(ctx, err)
		} else {
			slog.Error("Failed to get deploy instance logs", slog.Any("logReq", logReq), slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Info("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
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

	// Remove ".git" suffix if present
	if strings.HasSuffix(path, ".git") {
		path = strings.TrimSuffix(path, ".git")
	}

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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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

	allow, err := h.c.AllowAccessDeploy(ctx, statusReq)
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		var pErr *types.PermissionError
		if errors.As(err, &pErr) {
			httpbase.UnauthorizedError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	if !allow {
		slog.Info("user not allowed to query deploy status", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser), slog.Any("deploy_id", deployID))
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
			slog.Info("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(time.Second * 5)
			// user http request context instead of gin context, so that server knows the life cycle of the request
			_, status, instances, err := h.c.DeployStatus(ctx.Request.Context(), repoType, namespace, name, deployID)
			if err != nil {
				slog.Error("failed to get deploy status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name), slog.Any("deploy_id", deployID))
				ctx.SSEvent("error", err.Error())
			} else {
				eventData := &types.ModelStatusEventData{
					Status:  status,
					Details: instances,
				}
				ctx.SSEvent("status", eventData)

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
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	err = h.c.SyncMirror(ctx, repoType, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to sync mirror for", slog.String("repo_type", string(repoType)), slog.String("path", fmt.Sprintf("%s/%s", namespace, name)), "error", err)
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
			slog.Info("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, err := h.c.AllowAdminAccess(ctx, types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to check user permission", "error", err, slog.Any("currentUser", currentUser), slog.Any("namespace", name), slog.Any("name", name))
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}
	if !allow {
		slog.Info("user not allowed to update deploy", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.UnauthorizedError(ctx, errors.New("user not allowed to update deploy"))
		return
	}

	var req *types.DeployUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err, slog.Any("request.body", ctx.Request.Body))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica != nil && req.MaxReplica != nil {
		err = Validate.Struct(req)
		if err != nil {
			slog.Error("Bad request setting for deploy", slog.Any("req", *req), slog.Any("err", err))
			httpbase.BadRequest(ctx, fmt.Sprintf("Bad request setting for deploy, %v", err))
			return
		}
	}

	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err), slog.Any("id", ctx.Param("id")))
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
	err = h.c.DeployUpdate(ctx, updateReq, req)
	if err != nil {
		slog.Error("failed to update deploy", slog.String("namespace", namespace), slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID), slog.Any("error", err))
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
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/runtime_framework [get]
func (h *RepoHandler) RuntimeFrameworkListWithType(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
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

	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.Error("Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFrameworkWithType(ctx, deployType)
	if err != nil {
		slog.Error("fail to list runtime framework", slog.String("error", err.Error()))
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
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id} [get]
func (h *RepoHandler) ServerlessDetail(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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

	response, err := h.c.DeployDetail(ctx, detailReq)
	if err != nil {
		slog.Error("fail to serverless detail", slog.String("error", err.Error()), slog.Any("namespace", namespace), slog.Any("name", name), slog.Any("deploy id", deployID))
		httpbase.ServerError(ctx, err)
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
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id}/logs/{instance} [get]
func (h *RepoHandler) ServerlessLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		var pErr *types.PermissionError
		if errors.As(err, &pErr) {
			httpbase.UnauthorizedError(ctx, err)
		} else {
			slog.Error("Failed to get deploy instance logs", slog.Any("logReq", logReq), slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Info("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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

	allow, err := h.c.AllowAccessDeploy(ctx, statusReq)
	if err != nil {
		slog.Error("failed to check user permission", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission, %w", err))
		return
	}

	if !allow {
		slog.Info("user not allowed to query deploy status", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser), slog.Any("deploy_id", deployID))
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
			slog.Info("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(time.Second * 5)
			// user http request context instead of gin context, so that server knows the life cycle of the request
			_, status, instances, err := h.c.DeployStatus(ctx.Request.Context(), types.ModelRepo, namespace, name, deployID)
			if err != nil {
				slog.Error("failed to get deploy status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name), slog.Any("deploy_id", deployID))
				ctx.SSEvent("error", err.Error())
			} else {
				eventData := &types.ModelStatusEventData{
					Status:  status,
					Details: instances,
				}
				ctx.SSEvent("status", eventData)

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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req *types.DeployUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err, slog.Any("request.body", ctx.Request.Body))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica != nil && req.MaxReplica != nil {
		err = Validate.Struct(req)
		if err != nil {
			slog.Error("Bad request setting for serverless", slog.Any("req", *req), slog.Any("err", err))
			httpbase.BadRequest(ctx, fmt.Sprintf("Bad request setting for serverless, %v", err))
			return
		}
	}

	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err), slog.Any("id", ctx.Param("id")))
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
	err = h.c.DeployUpdate(ctx, updateReq, req)
	if err != nil {
		slog.Error("failed to update serverless", slog.String("namespace", namespace), slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID), slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to update serverless, %w", err))
		return
	}

	httpbase.OK(ctx, nil)
}
