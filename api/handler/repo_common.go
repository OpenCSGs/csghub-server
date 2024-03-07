package handler

import (
	"bytes"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewRepoHanlder(config *config.Config) (*RepoHanlder, error) {
	tc, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return &RepoHanlder{
		c: tc,
	}, nil
}

type RepoHanlder struct {
	c *component.RepoComponent
}

// CreateCodeFile godoc
// @Security     ApiKey
// @Summary      Create code file
// @Description  create code file
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        body body types.CreateFileReq true "body"
// @Success      200  {object}  types.Response{data=types.CreateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/raw/{file_path} [post]
func (h *RepoHanlder) CreateFile(ctx *gin.Context) {
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
	// req.RepoType = common.RepoTypeFromContext(ctx)
	req.RepoType = common.RepoTypeFromContext(ctx)

	resp, err = h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create file", slog.Any("error", err), slog.String("file_path", filePath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create file succeed", slog.String("file_path", filePath))
	httpbase.OK(ctx, resp)
}

// UpdateCodeFile godoc
// @Security     ApiKey
// @Summary      Update code file
// @Description  update code file
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        body body types.UpdateFileReq true "body"
// @Success      200  {object}  types.Response{data=types.UpdateFileResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/raw/{file_path} [put]
func (h *RepoHanlder) UpdateFile(ctx *gin.Context) {
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
	req.RepoType = common.RepoTypeFromContext(ctx)

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

// GetCodeCommits godoc
// @Security     ApiKey
// @Summary      Get code commits
// @Description  get code commits
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=[]types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/commits [get]
func (h *RepoHanlder) Commits(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	commits, err := h.c.Commits(ctx, req)
	if err != nil {
		slog.Error("Failed to get code commits", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code commits succeed", slog.String("code", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commits)
}

// GetCodeLastCommit godoc
// @Security     ApiKey
// @Summary      Get code last commit
// @Description  get code last commit
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/last_commit [get]
func (h *RepoHanlder) LastCommit(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	commit, err := h.c.LastCommit(ctx, req)
	if err != nil {
		slog.Error("Failed to get code last commit", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code last commit succeed", slog.String("code", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commit)
}

// GetCodeFileRaw godoc
// @Security     ApiKey
// @Summary      Get code file raw
// @Description  get code file raw
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/raw/{file_path} [get]
func (h *RepoHanlder) FileRaw(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	raw, err := h.c.FileRaw(ctx, req)
	if err != nil {
		slog.Error("Failed to get code file raw", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code file raw succeed", slog.String("code", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, raw)
}

// GetCodeFileInfo godoc
// @Security     ApiKey
// @Summary      Get code file info
// @Description  get code file info
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=types.Commit} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/blob/{file_path} [get]
func (h *RepoHanlder) FileInfo(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	file, err := h.c.FileInfo(ctx, req)
	if err != nil {
		slog.Error("Failed to get code file info", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code file info succeed", slog.String("code", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, file)
}

// DownloadCodeFile godoc
// @Security     ApiKey
// @Summary      Download code file
// @Description  download code file
// @Tags         Code
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
// @Router       /codes/{namespace}/{name}/download/{file_path} [get]
func (h *RepoHanlder) DownloadFile(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
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
		slog.Error("Failed to download code file", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if req.Lfs {
		httpbase.OK(ctx, url)
	} else {
		slog.Info("Download code file succeed", slog.String("code", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
		fileName := path.Base(req.Path)
		ctx.Header("Content-Type", "application/octet-stream")
		ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download code file", slog.Any("error", err))
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	}
}

// GetCodeBranches godoc
// @Security     ApiKey
// @Summary      Get code branches
// @Description  get code branches
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Success      200  {object}  types.Response{data=[]types.Branch} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/branches [get]
func (h *RepoHanlder) Branches(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	branches, err := h.c.Branches(ctx, req)
	if err != nil {
		slog.Error("Failed to get code branches", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code branches succeed", slog.String("code", name))
	httpbase.OK(ctx, branches)
}

// GetCodeTags godoc
// @Security     ApiKey
// @Summary      Get code tags
// @Description  get code tags
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Success      200  {object}  types.Response{data=[]types.Branch} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/tags [get]
func (h *RepoHanlder) Tags(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req := &types.GetTagsReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	tags, err := h.c.Tags(ctx, req)
	if err != nil {
		slog.Error("Failed to get code tags", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code tags succeed", slog.String("code", name))
	httpbase.OK(ctx, tags)
}

// GetCodeFileTree godoc
// @Security     ApiKey
// @Summary      Get code file tree
// @Description  get code file tree
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        ref query string false "ref"
// @Success      200  {object}  types.Response{data=[]types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/tree [get]
func (h *RepoHanlder) Tree(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	tree, err := h.c.Tree(ctx, req)
	if err != nil {
		slog.Error("Failed to get code file tree", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get code file tree succeed", slog.String("code", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, tree)
}

// UpdateCodeDownloads godoc
// @Security     ApiKey
// @Summary      Update code downloads
// @Description  update code downloads
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateDownloadsReq true "body"
// @Success      200  {object}  types.Response{data=[]types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name}/update_downloads [post]
func (h *RepoHanlder) UpdateDownloads(ctx *gin.Context) {
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
		slog.Error("Failed to update code download count", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.Time("date", date), slog.Int64("clone_count", req.CloneCount))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Update code download count succeed", slog.String("namespace", namespace), slog.String("name", name), slog.Int64("clone_count", req.CloneCount))
	httpbase.OK(ctx, nil)
}

// UploadCodeFile godoc
// @Security     ApiKey
// @Summary      Create code file
// @Description  upload code file to create or update a file in code repository
// @Tags         Code
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
// @Router       /codes/{namespace}/{name}/upload_file [post]
func (h *RepoHanlder) UploadFile(ctx *gin.Context) {
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
		slog.Info("Error encodeing uploaded file", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filePath := ctx.PostForm("file_path")
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath
	req.Content = buf.String()
	req.RepoType = common.RepoTypeFromContext(ctx)

	err = h.c.UploadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create code file", slog.Any("error", err), slog.String("file_path", filePath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create file succeed", slog.String("file_path", filePath))
	httpbase.OK(ctx, nil)
}

func (h *RepoHanlder) SDKListFiles(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	files, err := h.c.SDKListFiles(ctx, common.RepoTypeFromContext(ctx), namespace, name)
	if err != nil {
		slog.Error("Error listing code files", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, files)
}

func (h *RepoHanlder) SDKDownload(ctx *gin.Context) {
	h.handleDownload(ctx, false)
}

// DownloadCodeFile godoc
// @Security     ApiKey
// @Summary      Download code file
// @Description  download code file
// @Tags         Code
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
// @Router       /codes/{namespace}/{name}/resolve/{file_path} [get]
func (h *RepoHanlder) ResolveDownload(ctx *gin.Context) {
	h.handleDownload(ctx, true)
}

func (h *RepoHanlder) HeadSDKDownload(ctx *gin.Context) {
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

	file, err := h.c.HeadDownloadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to download code file", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Head download code file succeed", slog.String("code", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	ctx.Header("Content-Length", strconv.Itoa(file.Size))
	ctx.Header("X-Repo-Commit", file.SHA)
	ctx.Header("ETag", file.SHA)
	ctx.Status(http.StatusOK)
}

func (h *RepoHanlder) handleDownload(ctx *gin.Context, isResolve bool) {
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
	lfs, err := h.c.IsLfs(ctx, req)
	if err != nil {
		slog.Error("Filed to lfs information", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	req.Lfs = lfs
	reader, url, err := h.c.SDKDownloadFile(ctx, req)
	if err != nil {
		slog.Error("Failed to download code file", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if req.Lfs {
		ctx.Redirect(http.StatusMovedPermanently, url)
	} else {
		slog.Info("Download code file succeed", slog.String("code", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
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
