package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
	"opencsg.com/starhub-server/component"
)

var Sorts = []string{"trending", "recently_update", "most_download", "most_favorite"}

func NewDatasetHandler(config *config.Config) (*DatasetHandler, error) {
	tc, err := component.NewDatasetComponent(config)
	if err != nil {
		return nil, err
	}
	return &DatasetHandler{
		c: tc,
	}, nil
}

type DatasetHandler struct {
	c *component.DatasetComponent
}

func (h *DatasetHandler) CreateFile(ctx *gin.Context) {
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
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath

	resp, err = h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create file", slog.Any("error", err), slog.String("file_path", filePath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create file succeed", slog.String("file_path", filePath))
	httpbase.OK(ctx, resp)
}

func (h *DatasetHandler) UpdateFile(ctx *gin.Context) {
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
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath

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

func (h *DatasetHandler) Create(ctx *gin.Context) {
	var req *types.CreateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	dataset, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create dataset", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create dataset succeed", slog.String("dataset", dataset.Name))
	respData := gin.H{
		"data": dataset,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *DatasetHandler) Index(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	search, sort, tag := getFilterFromContext(ctx)
	if !slices.Contains[[]string](Sorts, sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	datasets, total, err := h.c.Index(ctx, search, sort, tag, per, page)
	if err != nil {
		slog.Error("Failed to create user", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create public datasets succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  datasets,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *DatasetHandler) Update(ctx *gin.Context) {
	var req *types.UpdateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Namespace = namespace
	req.OriginName = name

	dataset, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update dataset", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Update dataset succeed", slog.String("dataset", dataset.Name))
	httpbase.OK(ctx, dataset)
}

func (h *DatasetHandler) Delete(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	err = h.c.Delete(ctx, namespace, name)
	if err != nil {
		slog.Error("Failed to delete dataset", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Delete dataset succeed", slog.String("dataset", name))
	httpbase.OK(ctx, nil)
}

func (h *DatasetHandler) Detail(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	detail, err := h.c.Detail(ctx, namespace, name)
	if err != nil {
		slog.Error("Failed to get dataset detail", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Get dataset detail succeed", slog.String("dataset", name))
	httpbase.OK(ctx, detail)
}

func (h *DatasetHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := ctx.Query("current_user")
	detail, err := h.c.Show(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to get dataset detail", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset detail succeed", slog.String("dataset", name))
	httpbase.OK(ctx, detail)
}

func (h *DatasetHandler) Commits(ctx *gin.Context) {
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
	}
	commits, err := h.c.Commits(ctx, req)
	if err != nil {
		slog.Error("Failed to get dataset commits", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset commits succeed", slog.String("dataset", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commits)
}

func (h *DatasetHandler) LastCommit(ctx *gin.Context) {
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
	}
	commit, err := h.c.LastCommit(ctx, req)
	if err != nil {
		slog.Error("Failed to get dataset last commit", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset last commit succeed", slog.String("dataset", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commit)
}

func (h *DatasetHandler) FileRaw(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req := &types.GetFileReq{
		Namespace: namespace,
		Name:      name,
		Path:      ctx.Param("file_path"),
		Ref:       ctx.Query("ref"),
	}
	raw, err := h.c.FileRaw(ctx, req)
	if err != nil {
		slog.Error("Failed to get dataset file raw", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset file raw succeed", slog.String("dataset", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, raw)
}

func (h *DatasetHandler) Branches(ctx *gin.Context) {
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
	}
	branches, err := h.c.Branches(ctx, req)
	if err != nil {
		slog.Error("Failed to get dataset branches", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset branches succeed", slog.String("dataset", name))
	httpbase.OK(ctx, branches)
}

func (h *DatasetHandler) Tags(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req := &types.GetTagsReq{
		Namespace: namespace,
		Name:      name,
	}
	tags, err := h.c.Tags(ctx, req)
	if err != nil {
		slog.Error("Failed to get dataset tags", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset tags succeed", slog.String("dataset", name))
	httpbase.OK(ctx, tags)
}

func (h *DatasetHandler) Tree(ctx *gin.Context) {
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
	}
	tree, err := h.c.Tree(ctx, req)
	if err != nil {
		slog.Error("Failed to get dataset file tree", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get dataset file tree succeed", slog.String("dataset", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, tree)
}

func getFilterFromContext(ctx *gin.Context) (searchKey, sort, tag string) {
	searchKey = ctx.Query("search")
	sort = ctx.Query("sort")
	tag = ctx.Query("tag")
	if sort == "" {
		sort = "recently_update"
	}
	return
}
