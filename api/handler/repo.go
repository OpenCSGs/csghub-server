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
	"strconv"
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

func (h *RepoHandler) CreateFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
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
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = common.RepoTypeFromContext(ctx)

	resp, err := h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, resp)
}

func (h *RepoHandler) UpdateFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
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
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath
	req.RepoType = common.RepoTypeFromContext(ctx)

	resp, err := h.c.UpdateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to update repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Update repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name))
	httpbase.OK(ctx, resp)
}

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
		slog.Error("Failed to get repo commits", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo commits succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commits)
}

func (h *RepoHandler) LastCommit(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
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
		slog.Error("Failed to get repo last commit", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo last commit succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commit)
}

func (h *RepoHandler) FileRaw(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
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

func (h *RepoHandler) FileInfo(ctx *gin.Context) {
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
		RepoType:  common.RepoTypeFromContext(ctx),
	}
	file, err := h.c.FileInfo(ctx, req)
	if err != nil {
		slog.Error("Failed to get repo file info", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get repo file info succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, file)
}

func (h *RepoHandler) DownloadFile(ctx *gin.Context) {
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
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}
	}
}

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
	req := &types.GetBranchesReq{
		Namespace: namespace,
		Name:      name,
		Per:       per,
		Page:      page,
		RepoType:  common.RepoTypeFromContext(ctx),
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

func (h *RepoHandler) Tags(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := &types.GetTagsReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  common.RepoTypeFromContext(ctx),
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

func (h *RepoHandler) Tree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
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

func (h *RepoHandler) UploadFile(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
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
		w := base64.NewEncoder(base64.StdEncoding, &buf)
		_, err = io.Copy(w, openedFile)
		w.Close()
		openedFile.Close()
		if err != nil {
			slog.Info("Error encodeing uploaded file", "error", err, slog.String("file_name", file.Filename))
			httpbase.BadRequest(ctx, err.Error())
			return
		}
		filePath := paths[idx]

		upload := &types.CreateFileReq{
			Username:  userName,
			NameSpace: namespace,
			Name:      name,
			FilePath:  filePath,
			Content:   buf.String(),
			RepoType:  common.RepoTypeFromContext(ctx),
			Message:   message,
			Branch:    branch,
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	files, err := h.c.SDKListFiles(ctx, common.RepoTypeFromContext(ctx), namespace, name)
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

func (h *RepoHandler) ResolveDownload(ctx *gin.Context) {
	h.handleDownload(ctx, true)
}

func (h *RepoHandler) HeadSDKDownload(ctx *gin.Context) {
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

	slog.Info("Head download repo file succeed", slog.String("repo_type", string(req.RepoType)), slog.String("name", name), slog.String("path", req.Path), slog.String("ref", req.Ref),
		slog.Int64("contentLength", file.Size))
	ctx.Header("Content-Length", strconv.Itoa(int(file.Size)))
	ctx.Header("X-Repo-Commit", file.SHA)
	ctx.Header("ETag", file.SHA)
	ctx.Status(http.StatusOK)
}

func (h *RepoHandler) handleDownload(ctx *gin.Context, isResolve bool) {
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
	reader, url, err := h.c.SDKDownloadFile(ctx, req)
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
		_, err = io.Copy(ctx.Writer, reader)
		if err != nil {
			slog.Error("Failed to download repo file", slog.String("repo_type", string(req.RepoType)), slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}
	}
}
