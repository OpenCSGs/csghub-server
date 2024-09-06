package handler

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/golang/gddo/httputil"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewGitHTTPHandler(config *config.Config) (*GitHTTPHandler, error) {
	uc, err := component.NewGitHTTPComponent(config)
	if err != nil {
		return nil, err
	}
	return &GitHTTPHandler{
		c: uc,
	}, nil
}

type GitHTTPHandler struct {
	c *component.GitHTTPComponent
}

func (h *GitHTTPHandler) InfoRefs(ctx *gin.Context) {
	rpc := ctx.Query("service")
	if !(rpc == "git-upload-pack" || rpc == "git-receive-pack") {
		httpbase.NotFoundError(ctx, errors.New("service not found"))
	}
	ctx.Header("Content-Type", fmt.Sprintf("application/x-%s-advertisement", rpc))
	ctx.Header("Cache-Control", "no-cache")

	gitProtocol := ctx.GetHeader("Git-Protocol")

	offers := []string{"gzip", "identity"}
	encoding := httputil.NegotiateContentEncoding(ctx.Request, offers)
	req := types.InfoRefsReq{
		Namespace:   ctx.GetString("namespace"),
		Name:        ctx.GetString("name"),
		RepoType:    types.RepositoryType(ctx.GetString("repo_type")),
		Rpc:         rpc,
		GitProtocol: gitProtocol,
		CurrentUser: httpbase.GetCurrentUser(ctx),
	}
	reader, err := h.c.InfoRefs(ctx, req)
	if err != nil {
		if err == component.ErrUnauthorized {
			ctx.Header("WWW-Authenticate", "Basic realm=opencsg-git")
			ctx.PureJSON(http.StatusUnauthorized, nil)
			return
		}

		if err == component.ErrForbidden {
			ctx.PureJSON(http.StatusForbidden, gin.H{
				"error": "You do not have permission to access this repository.",
			})
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}

	if encoding == "gzip" {
		gzWriter := gzip.NewWriter(ctx.Writer)
		defer gzWriter.Close()

		gzRespWriter := &gzipResponseWriter{
			ResponseWriter: ctx.Writer,
			writer:         gzWriter,
		}
		ctx.Writer = gzRespWriter
		ctx.Header("Content-Encoding", "gzip")
	}

	if _, err = io.Copy(ctx.Writer, reader); err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
}

// func (h *GitHTTPHandler) HEAD(ctx *gin.Context) {
// 	httpbase.OK(ctx, nil)
// }

func (h *GitHTTPHandler) GitUploadPack(ctx *gin.Context) {
	gitProtocol := ctx.GetHeader("Git-Protocol")
	req := types.GitUploadPackReq{
		Namespace:   ctx.GetString("namespace"),
		Name:        ctx.GetString("name"),
		RepoType:    types.RepositoryType(ctx.GetString("repo_type")),
		GitProtocol: gitProtocol,
		Request:     ctx.Request,
		Writer:      ctx.Writer,
		CurrentUser: httpbase.GetCurrentUser(ctx),
	}
	action := getService(ctx.Request)

	ctx.Header("Content-Type", fmt.Sprintf("application/x-%s-result", action))
	ctx.Header("Cache-Control", "no-cache")

	err := h.c.GitUploadPack(ctx, req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
}

func (h *GitHTTPHandler) GitReceivePack(ctx *gin.Context) {
	gitProtocol := ctx.GetHeader("Git-Protocol")
	req := types.GitUploadPackReq{
		Namespace:   ctx.GetString("namespace"),
		Name:        ctx.GetString("name"),
		RepoType:    types.RepositoryType(ctx.GetString("repo_type")),
		GitProtocol: gitProtocol,
		Request:     ctx.Request,
		Writer:      ctx.Writer,
		CurrentUser: httpbase.GetCurrentUser(ctx),
	}
	action := getService(ctx.Request)

	ctx.Header("Content-Type", fmt.Sprintf("application/x-%s-result", action))
	ctx.Header("Cache-Control", "no-cache")

	err := h.c.GitReceivePack(ctx, req)
	if err != nil {
		if err == component.ErrUnauthorized {
			ctx.Header("WWW-Authenticate", "Basic realm=opencsg-git")
			ctx.PureJSON(http.StatusUnauthorized, nil)
			return
		}

		if err == component.ErrForbidden {
			ctx.PureJSON(http.StatusForbidden, gin.H{
				"error": "You do not have permission to access this repository.",
			})
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
}

func (h *GitHTTPHandler) LfsBatch(ctx *gin.Context) {
	var batchRequest types.BatchRequest
	if err := ctx.ShouldBindJSON(&batchRequest); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	batchRequest.CurrentUser = httpbase.GetCurrentUser(ctx)
	batchRequest.Authorization = ctx.Request.Header.Get("Authorization")
	batchRequest.Namespace = ctx.GetString("namespace")
	batchRequest.Name = ctx.GetString("name")
	batchRequest.RepoType = types.RepositoryType(ctx.GetString("repo_type"))

	var isUpload bool
	if batchRequest.Operation == "upload" {
		isUpload = true
	} else if batchRequest.Operation == "download" {
		isUpload = false
	} else {
		slog.Error("Invalid lfs batch operation", slog.String("operation", batchRequest.Operation))
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid lfs batch operation: %s", batchRequest.Operation))
		return
	}

	objectResponse, err := h.c.BuildObjectResponse(ctx, batchRequest, isUpload)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			ctx.Header("WWW-Authenticate", "Basic realm=opencsg-git")
			ctx.PureJSON(http.StatusUnauthorized, nil)
			return
		}

		if errors.Is(err, component.ErrForbidden) {
			ctx.PureJSON(http.StatusForbidden, gin.H{
				"error": "You do not have permission to access this repository.",
			})
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.Header("Content-Type", types.LfsMediaType)
	ctx.PureJSON(http.StatusOK, objectResponse)
}

func (h *GitHTTPHandler) LfsUpload(ctx *gin.Context) {
	var err error
	var uploadRequest types.UploadRequest
	uploadRequest.Oid = ctx.Param("oid")
	uploadRequest.Size, err = strconv.ParseInt(ctx.Param("size"), 10, 64)
	if err != nil {
		slog.Error("Invalid lfs file size", slog.String("size", ctx.Param("size")))
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid lfs file size: %s", ctx.Param("size")))
		return
	}
	uploadRequest.Namespace = ctx.GetString("namespace")
	uploadRequest.Name = ctx.GetString("name")
	uploadRequest.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	uploadRequest.CurrentUser = httpbase.GetCurrentUser(ctx)

	err = h.c.LfsUpload(ctx, ctx.Request.Body, uploadRequest)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.Header("Content-Type", types.LfsMediaType)
}

func (h *GitHTTPHandler) LfsDownload(ctx *gin.Context) {
	var err error
	var downloadRequest types.DownloadRequest
	downloadRequest.Oid = ctx.Param("oid")
	downloadRequest.Size, err = strconv.ParseInt(ctx.Param("size"), 10, 64)
	if err != nil {
		slog.Error("Invalid lfs file size", slog.String("size", ctx.Param("size")))
		httpbase.BadRequest(ctx, fmt.Sprintf("Invalid lfs file size: %s", ctx.Param("size")))
		return
	}
	downloadRequest.Namespace = ctx.GetString("namespace")
	downloadRequest.Name = ctx.GetString("name")
	downloadRequest.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	downloadRequest.CurrentUser = httpbase.GetCurrentUser(ctx)
	downloadRequest.SaveAs = ctx.Query("save_as")

	url, err := h.c.LfsDownload(ctx, downloadRequest)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.Redirect(http.StatusFound, url.String())
}

func (h *GitHTTPHandler) LfsVerify(ctx *gin.Context) {
	var (
		pointer       types.Pointer
		verifyRequest types.VerifyRequest
	)
	if err := ctx.ShouldBindJSON(&pointer); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	verifyRequest.Namespace = ctx.GetString("namespace")
	verifyRequest.Name = ctx.GetString("name")
	verifyRequest.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	verifyRequest.CurrentUser = httpbase.GetCurrentUser(ctx)

	err := h.c.LfsVerify(ctx, verifyRequest, pointer)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ctx.PureJSON(http.StatusOK, nil)
}

func (h *GitHTTPHandler) ListLocks(ctx *gin.Context) {
	var (
		req types.ListLFSLockReq
		err error
	)

	req.Namespace = ctx.GetString("namespace")
	req.Name = ctx.GetString("name")
	req.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	req.Path = ctx.Query("path")
	id := ctx.Query("id")
	if id != "" {
		req.ID, err = strconv.ParseInt(id, 10, 64)
		if err != nil {
			slog.Error("Bad request format", "error", err)
			ctx.PureJSON(http.StatusBadRequest, types.LFSLockError{
				Message: "unable to delete lock : Invalid request",
			})
			return
		}
	}
	cursor, _ := strconv.Atoi(ctx.Query("cursor"))
	if cursor < 0 {
		cursor = 0
	}
	req.Cursor = cursor

	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if limit < 0 {
		limit = 0
	}
	req.Limit = limit

	res, err := h.c.ListLocks(ctx, req)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			ctx.Header("WWW-Authenticate", "Basic realm=opencsg-git")
			ctx.PureJSON(http.StatusUnauthorized, types.LFSLockError{
				Message: "You must have access to read locks",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, types.LFSLockError{
			Message: "unable to list locks : Internal Server Error",
		})
		return
	}
	ctx.PureJSON(http.StatusOK, res)
}

func (h *GitHTTPHandler) CreateLock(ctx *gin.Context) {
	var req types.LfsLockReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusBadRequest, types.LFSLockError{
			Message: "unable to delete lock : Invalid request",
		})
		return
	}

	req.Namespace = ctx.GetString("namespace")
	req.Name = ctx.GetString("name")
	req.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	lock, err := h.c.CreateLock(ctx, req)
	if err != nil {
		if errors.Is(err, component.ErrAlreadyExists) {
			ctx.PureJSON(http.StatusConflict, types.LFSLockError{
				Lock: &types.LFSLock{
					ID:       strconv.FormatInt(lock.ID, 10),
					Path:     lock.Path,
					LockedAt: lock.CreatedAt,
					Owner: &types.LFSLockOwner{
						Name: lock.User.Username,
					},
				},
				Message: "already created lock",
			})
			return
		}

		if errors.Is(err, component.ErrUnauthorized) {
			ctx.Header("WWW-Authenticate", "Basic realm=opencsg-git")
			ctx.PureJSON(http.StatusUnauthorized, types.LFSLockError{
				Message: "You must have push access to create locks",
			})
			return
		}
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusInternalServerError, types.LFSLockError{
			Message: "unable to delete lock : Internal Server Error",
		})
		return
	}
	ctx.PureJSON(http.StatusOK, &types.LFSLockResponse{
		Lock: &types.LFSLock{
			ID:       strconv.FormatInt(lock.ID, 10),
			Path:     lock.Path,
			LockedAt: lock.CreatedAt,
			Owner: &types.LFSLockOwner{
				Name: lock.User.Username,
			},
		},
	})
}

func (h *GitHTTPHandler) VerifyLock(ctx *gin.Context) {
	var req types.VerifyLFSLockReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusBadRequest, types.LFSLockError{
			Message: "unable to delete lock : Invalid request",
		})
		return
	}

	req.Namespace = ctx.GetString("namespace")
	req.Name = ctx.GetString("name")
	req.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	cursor, _ := strconv.Atoi(ctx.PostForm("cursor"))
	if cursor < 0 {
		cursor = 0
	}
	req.Cursor = cursor

	limit, _ := strconv.Atoi(ctx.PostForm("limit"))
	if limit < 0 {
		limit = 0
	}
	req.Limit = limit

	res, err := h.c.VerifyLock(ctx, req)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusInternalServerError, types.LFSLockError{
			Message: "unable to delete lock : Internal Server Error",
		})
		return
	}
	ctx.PureJSON(http.StatusOK, res)
}

func (h *GitHTTPHandler) UnLock(ctx *gin.Context) {
	var (
		req  types.UnlockLFSReq
		err  error
		lock *database.LfsLock
	)

	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusInternalServerError, types.LFSLockError{
			Message: "unable to delete lock : Bad request format",
		})
		return
	}
	req.ID, err = strconv.ParseInt(ctx.Param("lid"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusBadRequest, types.LFSLockError{
			Message: "unable to delete lock : Invalid id",
		})
		return
	}

	req.Namespace = ctx.GetString("namespace")
	req.Name = ctx.GetString("name")
	req.RepoType = types.RepositoryType(ctx.GetString("repo_type"))
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	lock, err = h.c.UnLock(ctx, req)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			ctx.Header("WWW-Authenticate", "Basic realm=opencsg-git")
			ctx.PureJSON(http.StatusUnauthorized, types.LFSLockError{
				Message: "You must have push access to create locks",
			})
			return
		}
		if errors.Is(err, component.ErrNotFound) {
			ctx.PureJSON(http.StatusNotFound, types.LFSLockError{
				Message: "unable to delete lock : not found",
			})
			return
		}
		if errors.Is(err, component.ErrPermissionDenied) {
			ctx.PureJSON(http.StatusMethodNotAllowed, types.LFSLockError{
				Message: "unable to delete lock : this lock is not created by you, try --force if you still want unlock it",
			})
			return
		}
		slog.Error("Bad request format", "error", err)
		ctx.PureJSON(http.StatusInternalServerError, types.LFSLockError{
			Message: "unable to delete lock : Internal Server Error",
		})
		return
	}
	ctx.PureJSON(http.StatusOK, types.LFSLockResponse{
		Lock: &types.LFSLock{
			ID:       strconv.FormatInt(lock.ID, 10),
			Path:     lock.Path,
			LockedAt: lock.CreatedAt,
			Owner: &types.LFSLockOwner{
				Name: lock.User.Username,
			},
		},
	})
}

func getService(r *http.Request) string {
	if r.Method == "GET" {
		return r.URL.Query().Get("service")
	}
	return filepath.Base(r.URL.Path)
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	return g.writer.Write(data)
}

func (g *gzipResponseWriter) WriteString(s string) (int, error) {
	return g.writer.Write([]byte(s))
}
