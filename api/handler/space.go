package handler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewSpaceHandler(config *config.Config) (*SpaceHandler, error) {
	sc, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, err
	}
	rp, err := proxy.NewReverseProxy(config.Space.K8SEndpoint)
	if err != nil {
		// log error and continue
		slog.Error("failed to create space reverse proxy", slog.String("K8sEndpoint", config.Space.K8SEndpoint),
			slog.Any("error", err))
	}
	return &SpaceHandler{
		c:      sc,
		rproxy: rp,
	}, nil
}

type SpaceHandler struct {
	c      *component.SpaceComponent
	rproxy *proxy.ReverseProxy
}

// GetAllSpaces   godoc
// @Security     ApiKey
// @Summary      Get spaces visible to current user
// @Description  get spaces visible to current user
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)data
// @Param        page query int false "per page" default(1)
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Space,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [get]
func (h *SpaceHandler) Index(ctx *gin.Context) {
	username := ctx.Query("current_user")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	search, sort := getFilterFromContext(ctx)
	if !slices.Contains[[]string](Sorts, sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		httpbase.BadRequest(ctx, msg)
		return
	}
	spaces, total, err := h.c.Index(ctx, username, search, sort, per, page)
	if err != nil {
		slog.Error("Failed to get spaces", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public spaces succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  spaces,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *SpaceHandler) Get(ctx *gin.Context) {
}

// CreateSpace   godoc
// @Security     ApiKey
// @Summary      Create a new space
// @Description  create a new space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        body body types.CreateSpaceReq true "body"
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [post]
func (h *SpaceHandler) Create(ctx *gin.Context) {
	var req types.CreateSpaceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	space, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create space", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create space succeed", slog.String("space", space.Name))
	httpbase.OK(ctx, space)
}

func (h *SpaceHandler) Update(ctx *gin.Context) {
	type updateSpaceReq struct {
		Username string `json:"username" example:"creator_user_name"`
		License  string `json:"license" example:"MIT"`
		Private  bool   `json:"private"`
	}
}

// DeleteSpace   godoc
// @Security     ApiKey
// @Summary      Delete a exists space
// @Description  delete a exists space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name} [delete]
func (h *SpaceHandler) Delete(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	currentUser := ctx.Query("current_user")
	err = h.c.Delete(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to delete space", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Delete space succeed", slog.String("space", name))
	httpbase.OK(ctx, nil)
}

// CallSpaceApi   godoc
// @Security     JWT token
// @Summary      Call space api
// @Description  call space api
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        api_name path string true "api_name"
// @Param        body body string false "body"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/api/{api_name} [post]
func (h *SpaceHandler) Proxy(ctx *gin.Context) {
	username, exists := ctx.Get("currentUser")
	if !exists {
		slog.Info("username not found in gin context")
		httpbase.BadRequest(ctx, "user not found, please login first")
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, err := h.c.AllowCallApi(ctx, namespace, name, username.(string))
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if allow {
		apiname := ctx.Param("api_name")
		slog.Info("proxy space request", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username),
			slog.String("api_name", apiname))
		h.rproxy.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Info("user not allowed to call sapce api", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username))
	}
}

// CreateSpaceFile godoc
// @Security     ApiKey
// @Summary      Create space file
// @Description  create space file
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        body body types.CreateFileReq true "body"
// @Success      200  {object}  types.Response{data=types.CreateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/raw/{file_path} [post]
func (h *SpaceHandler) CreateFile(ctx *gin.Context) {
	var (
		req  *types.CreateFileReq
		resp *types.CreateFileResp
	)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = types.SpaceRepo

	resp, err = h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create file", slog.Any("error", err), slog.String("file_path", filePath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create file succeed", slog.String("file_path", filePath))
	httpbase.OK(ctx, resp)
}

// UpdateSpaceFile godoc
// @Security     ApiKey
// @Summary      Update code file
// @Description  update code file
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        body body types.UpdateFileReq true "body"
// @Success      200  {object}  types.Response{data=types.UpdateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/raw/{file_path} [put]
func (h *SpaceHandler) UpdateFile(ctx *gin.Context) {
	var (
		req  *types.UpdateFileReq
		resp *types.UpdateFileResp
	)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = types.SpaceRepo

	resp, err = h.c.UpdateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to update file", slog.Any("error", err), slog.String("file_path", filePath),
			slog.String("origin_path", req.OriginPath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Update file succeed", slog.String("file_path", filePath),
		slog.String("origin_path", req.OriginPath))
	httpbase.OK(ctx, resp)
}

// GetSpaceCommits godoc
// @Security     ApiKey
// @Summary      Get space commits
// @Description  get space commits
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=[]types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/commits [get]
func (h *SpaceHandler) Commits(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ref := ctx.Query("ref")
	req := &types.GetCommitsReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		Per:       per,
		Page:      page,
		RepoType:  types.SpaceRepo,
	}
	commits, err := h.c.Commits(ctx, req)
	if err != nil {
		slog.Error("Failed to get space commits", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space commits succeed", slog.String("space", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commits)
}

// GetSpaceLastCommit godoc
// @Security     ApiKey
// @Summary      Get space last commit
// @Description  get space last commit
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/last_commit [get]
func (h *SpaceHandler) LastCommit(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	ref := ctx.Query("ref")
	req := &types.GetCommitsReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  types.SpaceRepo,
	}
	commit, err := h.c.LastCommit(ctx, req)
	if err != nil {
		slog.Error("Failed to get space last commit", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space last commit succeed", slog.String("space", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commit)
}

// GetSpaceFileRaw godoc
// @Security     ApiKey
// @Summary      Get space file raw
// @Description  get space file raw
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/raw/{file_path} [get]
func (h *SpaceHandler) FileRaw(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      filePath,
		Ref:       ctx.Query("ref"),
		RepoType:  types.SpaceRepo,
	}
	raw, err := h.c.FileRaw(ctx, req)
	if err != nil {
		slog.Error("Failed to get space file raw", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space file raw succeed", slog.String("space", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, raw)
}

// GetSpaceFileInfo godoc
// @Security     ApiKey
// @Summary      Get space file info
// @Description  get space file info
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/blob/{file_path} [get]
func (h *SpaceHandler) FileInfo(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      filePath,
		Ref:       ctx.Query("ref"),
		RepoType:  types.SpaceRepo,
	}
	file, err := h.c.FileInfo(ctx, req)
	if err != nil {
		slog.Error("Failed to get space file info", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space file info succeed", slog.String("space", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, file)
}

// DownloadSpaceFile godoc
// @Security     ApiKey
// @Summary      Download space file
// @Description  download space file
// @Tags         Space
// @Accept       json
// @Produce      json
// @Produce      octet-stream
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        lfs query bool false "lfs"
// @Param        ref query string false "ref"
// @Param        save_as query string false "name of download file"
// @Success      200  {object}  types.Response{data=string} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/download/{file_path} [get]
func (h *SpaceHandler) DownloadFile(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      filePath,
		Ref:       ctx.Query("ref"),
		Lfs:       false,
		SaveAs:    ctx.Query("save_as"),
		RepoType:  types.SpaceRepo,
	}
	if ctx.Query("lfs") != "" {
		req.Lfs, err = strconv.ParseBool(ctx.Query("lfs"))
		if err != nil {
			slog.Error("Bad request format", "error", err)
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}
	reader, url, err := h.c.DownloadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to download space file", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if req.Lfs {
		httpbase.OK(ctx, url)
	} else {
		slog.Info("Download space file succeed", slog.String("space", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download space file", slog.Any("error", err))
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	}
}

// GetSpaceBranches godoc
// @Security     ApiKey
// @Summary      Get space branches
// @Description  get space branches
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Success      200  {object}  types.Response{data=[]types.Branch} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/branches [get]
func (h *SpaceHandler) Branches(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req := &types.GetBranchesReq{
		Namespace: namespace,
		Name:      name,
		Per:       per,
		Page:      page,
		RepoType:  types.SpaceRepo,
	}
	branches, err := h.c.Branches(ctx, req)
	if err != nil {
		slog.Error("Failed to get space branches", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space branches succeed", slog.String("space", name))
	httpbase.OK(ctx, branches)
}

// GetSpaceTags godoc
// @Security     ApiKey
// @Summary      Get space tags
// @Description  get space tags
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success      200  {object}  types.Response{data=[]types.Branch} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/tags [get]
func (h *SpaceHandler) Tags(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req := &types.GetTagsReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  types.SpaceRepo,
	}
	tags, err := h.c.Tags(ctx, req)
	if err != nil {
		slog.Error("Failed to get space tags", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space tags succeed", slog.String("space", name))
	httpbase.OK(ctx, tags)
}

// GetSpaceFileTree godoc
// @Security     ApiKey
// @Summary      Get space file tree
// @Description  get space file tree
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=[]types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/tree [get]
func (h *SpaceHandler) Tree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      ctx.Query("path"),
		Ref:       ctx.Query("ref"),
		RepoType:  types.SpaceRepo,
	}
	tree, err := h.c.Tree(ctx, req)
	if err != nil {
		slog.Error("Failed to get space file tree", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get space file tree succeed", slog.String("space", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, tree)
}

// UpdateSpaceDownloads godoc
// @Security     ApiKey
// @Summary      Update space downloads
// @Description  update space downloads
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateDownloadsReq true "body"
// @Success      200  {object}  types.Response{data=[]types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/update_downloads [post]
func (h *SpaceHandler) UpdateDownloads(ctx *gin.Context) {
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
	req.RepoType = types.SpaceRepo
	date, err := time.Parse("2006-01-02", req.ReqDate)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Date = date

	err = h.c.UpdateDownloads(ctx, req)
	if err != nil {
		slog.Error("Failed to update space download count", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.Time("date", date), slog.Int64("clone_count", req.CloneCount))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Update space download count succeed", slog.String("namespace", namespace), slog.String("name", name), slog.Int64("clone_count", req.CloneCount))
	httpbase.OK(ctx, nil)
}

// UploadSpaceFile godoc
// @Security     ApiKey
// @Summary      Create space file
// @Description  upload space file to create or update a file in space repository
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path formData string true "file_path"
// @Param        file formData file true "file"
// @Param        email formData string true "email"
// @Param        message formData string true "message"
// @Param        branch formData string false "branch"
// @Param        username formData string true "username"
// @Success      200  {object}  types.Response{data=types.CreateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/upload_file [post]
func (h *SpaceHandler) UploadFile(ctx *gin.Context) {
	var req *types.CreateFileReq

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err = ctx.ShouldBind(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		slog.Error("Error opening uploaded file", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	defer openedFile.Close()
	var buf bytes.Buffer
	w := base64.NewEncoder(base64.StdEncoding, &buf)
	_, err = io.Copy(w, openedFile)
	w.Close()
	if err != nil {
		slog.Info("Error enspaceing uploaded file", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filePath := ctx.PostForm("file_path")
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath
	req.Content = buf.String()
	req.RepoType = types.SpaceRepo

	err = h.c.UploadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create space file", slog.Any("error", err), slog.String("file_path", filePath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create file succeed", slog.String("file_path", filePath))
	httpbase.OK(ctx, nil)
}

func (h *SpaceHandler) SDKListFiles(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	files, err := h.c.SDKListFiles(ctx, types.SpaceRepo, namespace, name)
	if err != nil {
		slog.Error("Error listing space files", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, files)
}

func (h *SpaceHandler) SDKDownload(ctx *gin.Context) {
	h.handleDownload(ctx, false)
}

// DownloadSpaceFile godoc
// @Security     ApiKey
// @Summary      Download space file
// @Description  download space file
// @Tags         Space
// @Accept       json
// @Produce      json
// @Produce      octet-stream
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=string} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/resolve/{file_path} [get]
func (h *SpaceHandler) ResolveDownload(ctx *gin.Context) {
	h.handleDownload(ctx, true)
}

func (h *SpaceHandler) HeadSDKDownload(ctx *gin.Context) {
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
		RepoType:  types.SpaceRepo,
	}

	file, err := h.c.HeadDownloadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to download space file", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Head download space file succeed", slog.String("space", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	ctx.Header("Content-Length", strconv.Itoa(file.Size))
	ctx.Header("X-Repo-Commit", file.SHA)
	ctx.Header("ETag", file.SHA)
	ctx.Status(http.StatusOK)
}

func (h *SpaceHandler) handleDownload(ctx *gin.Context, isResolve bool) {
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
		RepoType:  types.SpaceRepo,
	}
	lfs, err := h.c.IsLfs(ctx, req)
	if err != nil {
		slog.Error("Filed to lfs information", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	req.Lfs = lfs
	reader, url, err := h.c.SDKDownloadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to download space file", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if req.Lfs {
		ctx.Redirect(http.StatusMovedPermanently, url)
	} else {
		slog.Info("Download space file succeed", slog.String("space", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download model file", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}
	}
}
