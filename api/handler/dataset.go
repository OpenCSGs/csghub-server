package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
	"opencsg.com/starhub-server/component"
)

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
	var req *types.CreateFileReq
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get namespace from context", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if err = ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	filePath := ctx.Param("file_path")
	req.NameSpace = namespace
	req.Name = name
	req.FilePath = filePath

	err = h.c.CreateFile(ctx, req)
	if err != nil {
		slog.Error("Failed to create file", slog.Any("error", err), slog.String("file_path", filePath))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create file succeed", slog.String("file_path", filePath))
	ctx.JSON(http.StatusOK, nil)
}
