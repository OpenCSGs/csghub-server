package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
	"opencsg.com/starhub-server/component"
)

func NewModelHandler(config *config.Config) (*ModelHandler, error) {
	uc, err := component.NewModelComponent(config)
	if err != nil {
		return nil, err
	}
	return &ModelHandler{
		c: uc,
	}, nil
}

type ModelHandler struct {
	c *component.ModelComponent
}

func (h *ModelHandler) Index(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	models, total, err := h.c.Index(ctx, per, page)
	if err != nil {
		slog.Error("Failed to create user", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create public models succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  models,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *ModelHandler) Create(ctx *gin.Context) {
	var req *types.CreateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	model, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create model", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create model succeed", slog.String("model", model.Name))
	httpbase.OK(ctx, model)
}

func (h *ModelHandler) Update(ctx *gin.Context) {
	var req *types.UpdateModelReq
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

	model, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update model", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Update model succeed", slog.String("model", model.Name))
	httpbase.OK(ctx, model)
}

func (h *ModelHandler) Delete(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	err = h.c.Delete(ctx, namespace, name)
	if err != nil {
		slog.Error("Failed to delete model", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Delete model succeed", slog.String("model", name))
	httpbase.OK(ctx, nil)
}

func (h *ModelHandler) Detail(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	detail, err := h.c.Detail(ctx, namespace, name)
	if err != nil {
		slog.Error("Failed to get model detail", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model detail succeed", slog.String("model", name))
	httpbase.OK(ctx, detail)
}

func (h *ModelHandler) CreateFile(ctx *gin.Context) {
	var req *types.CreateFileReq
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
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = ctx.Param("file_path")

	err = h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create model file", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create model file succeed", slog.String("model", name))
	httpbase.OK(ctx, nil)
}

func (h *ModelHandler) UpdateFile(ctx *gin.Context) {
	var req *types.UpdateFileReq
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

	req.NameSpace = namespace
	req.Name = name
	req.FilePath = ctx.Param("file_path")

	err = h.c.UpdateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to update model file", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Update model file succeed", slog.String("model", name))
	httpbase.OK(ctx, nil)
}

func (h *ModelHandler) Commits(ctx *gin.Context) {
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
		slog.Error("Failed to get model commits", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model commits succeed", slog.String("model", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commits)
}

func (h *ModelHandler) LastCommit(ctx *gin.Context) {
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
		slog.Error("Failed to get model last commit", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model last commit succeed", slog.String("model", name), slog.String("ref", req.Ref))
	httpbase.OK(ctx, commit)
}

func (h *ModelHandler) FileRaw(ctx *gin.Context) {
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
		slog.Error("Failed to get model file raw", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model file raw succeed", slog.String("model", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, raw)
}

func (h *ModelHandler) Branches(ctx *gin.Context) {
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
		slog.Error("Failed to get model branches", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model branches succeed", slog.String("model", name))
	httpbase.OK(ctx, branches)
}

func (h *ModelHandler) Tags(ctx *gin.Context) {
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
		slog.Error("Failed to get model tags", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model tags succeed", slog.String("model", name))
	httpbase.OK(ctx, tags)
}

func (h *ModelHandler) Tree(ctx *gin.Context) {
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
	tree, err := h.c.Tree(ctx, req)
	if err != nil {
		slog.Error("Failed to get model file tree", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get model file tree succeed", slog.String("model", name), slog.String("path", req.Path), slog.String("ref", req.Ref))
	httpbase.OK(ctx, tree)
}
