package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewCodeHandler(config *config.Config) (*CodeHandler, error) {
	tc, err := component.NewCodeComponent(config)
	if err != nil {
		return nil, err
	}
	return &CodeHandler{
		c: tc,
	}, nil
}

type CodeHandler struct {
	c *component.CodeComponent
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

// CreateCode   godoc
// @Security     ApiKey
// @Summary      Create a new code
// @Description  create a new code
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        body body types.CreateCodeReq true "body"
// @Success      200  {object}  types.Response{data=types.Code} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes [post]
func (h *CodeHandler) Create(ctx *gin.Context) {
	var req *types.CreateCodeReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	code, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create code succeed", slog.String("code", code.Name))
	respData := gin.H{
		"data": code,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetVisiableCodes godoc
// @Security     ApiKey
// @Summary      Get Visiable codes for current user
// @Description  get visiable codes for current user
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param        current_user query string true "current user"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Code,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes [get]
func (h *CodeHandler) Index(ctx *gin.Context) {
	tagReqs := parseTagReqs(ctx)
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
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	codes, total, err := h.c.Index(ctx, username, search, sort, tagReqs, per, page)
	if err != nil {
		slog.Error("Failed to get codes", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public codes succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  codes,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// UpdateCode   godoc
// @Security     ApiKey
// @Summary      Update a exists code
// @Description  update a exists code
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.UpdateCodeReq true "body"
// @Success      200  {object}  types.Response{data=database.Code} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name} [put]
func (h *CodeHandler) Update(ctx *gin.Context) {
	var req *types.UpdateCodeReq
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

	code, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update code succeed", slog.String("code", code.Name))
	httpbase.OK(ctx, code)
}

// DeleteCode   godoc
// @Security     ApiKey
// @Summary      Delete a exists code
// @Description  delete a exists code
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name} [delete]
func (h *CodeHandler) Delete(ctx *gin.Context) {
	username, exists := ctx.Get("currentUser")
	if !exists {
		slog.Info("username not found in gin context")
		httpbase.BadRequest(ctx, "user not found, please login first")
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, _ := h.c.AllowAdminAccess(ctx, namespace, name, username.(string))
	if !allow {
		httpbase.UnauthorizedError(ctx, errors.New("user not allowed to delete space"))
		return
	}
	err = h.c.Delete(ctx, namespace, name, username.(string))
	if err != nil {
		slog.Error("Failed to delete code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete code succeed", slog.String("code", name))
	httpbase.OK(ctx, nil)
}

// GetCode      godoc
// @Security     ApiKey
// @Summary      Get code detail
// @Description  get code detail
// @Tags         Code
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{data=types.Code} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /codes/{namespace}/{name} [get]
func (h *CodeHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := ctx.Query("current_user")
	detail, err := h.c.Show(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to get code", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get code succeed", slog.String("code", name))
	httpbase.OK(ctx, detail)
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
