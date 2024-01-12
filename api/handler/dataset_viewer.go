package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type DatasetViewerHandler struct {
	c *component.DatasetViewerComponent
}

func NewDatasetViewerHandler(cfg *config.Config) (*DatasetViewerHandler, error) {
	dvc, err := component.NewDatasetViewerComponent(cfg)
	if err != nil {
		return nil, err
	}

	return &DatasetViewerHandler{
		c: dvc,
	}, nil
}

func (h *DatasetViewerHandler) View(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := new(component.ViewParquetFileReq)
	req.Namespace = namespace
	req.RepoName = name
	req.Path = ctx.Param("file_path")
	pcount := ctx.Query("count")
	req.RowCount, _ = strconv.Atoi(pcount)
	resp, err := h.c.ViewParquetFile(ctx, req)
	if err != nil {
		slog.Error("Failed to view parquet file", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}
