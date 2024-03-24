package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
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
	rp, err := proxy.NewReverseProxy(config.Space.RootDomain)
	if err != nil {
		// log error and continue
		slog.Error("failed to create space reverse proxy", slog.String("K8sEndpoint", config.Space.RunnerEndpoint),
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

// ShowSpaceDetail   godoc
// @Security     ApiKey
// @Summary      show space detail
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name} [get]
func (h *SpaceHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := ctx.Query("current_user")
	detail, err := h.c.Show(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to get space detail", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get space succeed", slog.String("space", name))
	httpbase.OK(ctx, detail)
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
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create space succeed", slog.String("space", space.Name))
	httpbase.OK(ctx, space)
}

// UpdateSpace   godoc
// @Security     ApiKey
// @Summary      Update a exists space
// @Description  update a exists space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateSpaceReq true "body"
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name} [put]
func (h *SpaceHandler) Update(ctx *gin.Context) {
	var req *types.UpdateSpaceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name

	space, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update space succeed", slog.String("space", space.Name))
	httpbase.OK(ctx, space)
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
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := ctx.Query("current_user")
	err = h.c.Delete(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to delete space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
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

// RunSpace   godoc
// @Security     JWT token
// @Summary      run space app
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/run [post]
func (h *SpaceHandler) Run(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	deployID, err := h.c.Deploy(ctx, namespace, name)
	if err != nil {
		slog.Error("failed to deploy space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("failed to deploy space"))
		return
	}

	slog.Info("deploy space sucess", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))
	httpbase.OK(ctx, nil)
}

// StopSpace   godoc
// @Security     JWT token
// @Summary      stop space app
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/stop [post]
func (h *SpaceHandler) Stop(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.c.Stop(ctx, namespace, name)
	if err != nil {
		slog.Error("failed to stop space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("failed to stop space"))
		return
	}

	slog.Info("stop space sucess", slog.String("namespace", namespace),
		slog.String("name", name))
	httpbase.OK(ctx, nil)
}

// GetSpaceStatus   godoc
// @Security     JWT token
// @Summary      get space status
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/status [get]
func (h *SpaceHandler) Status(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.status(ctx)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var allow bool
	username, exists := ctx.Get("currentUser")
	if exists {
		allow, err = h.c.AllowReadAccess(ctx, namespace, name, username.(string))
	} else {
		allow, err = h.c.AllowReadAccess(ctx, namespace, name, "")
	}

	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Info("user not allowed to query sapce status", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username))
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 创建一个通知channel以监测客户端是否已经断开连接
	closeNotify := ctx.Writer.CloseNotify()

	for {
		select {
		case <-closeNotify:
			return
		default:
			time.Sleep(time.Second * 5)
			_, status, err := h.c.Status(ctx, namespace, name)
			if err != nil {
				slog.Error("failed to get space status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name))
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)
			}
			ctx.Writer.Flush()
		}
	}
}

func (h *SpaceHandler) status(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 创建一个通知channel以监测客户端是否已经断开连接
	closeNotify := ctx.Writer.CloseNotify()

	for {
		select {
		case <-closeNotify:
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

// GetSpaceLogs   godoc
// @Security     JWT token
// @Summary      get space logs
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/logs [get]
func (h *SpaceHandler) Logs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var allow bool
	username, exists := ctx.Get("currentUser")
	if exists {
		allow, err = h.c.AllowReadAccess(ctx, namespace, name, username.(string))
	} else {
		allow, err = h.c.AllowReadAccess(ctx, namespace, name, "")
	}

	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Info("user not allowed to query sapce status", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username))
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logs, err := h.c.Logs(ctx, namespace, name)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	defer logs.Close()
	closeNotify := ctx.Writer.CloseNotify()

	type logContent struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}

	buildLogChan := logs.BuildLog()
	runLogChan := logs.RunLog()
	for {
		select {
		case <-closeNotify:
			// logs.Close()
			return
		case data := <-buildLogChan:
			ctx.SSEvent("Build", string(data))
			ctx.Writer.Flush()
		case data := <-runLogChan:
			ctx.SSEvent("Container", string(data))
			ctx.Writer.Flush()
		}
	}
}

func (h *SpaceHandler) testLogs(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	// watch client connection
	closeNotify := ctx.Writer.CloseNotify()

	for {
		select {
		case <-closeNotify:
			return
		default:
			ctx.SSEvent("Build", "test build log message")
			ctx.SSEvent("Container", "test run log message")
			ctx.Writer.Flush()
		}
		time.Sleep(time.Second * 5)
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
