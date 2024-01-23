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

// GetDatasetDemoData godoc
// @Security     ApiKey
// @Summary      Get the demo data of the dataset
// @Description  get the demo data of the dataset
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Parsm        name path string true "name"
// @Param        file_path path string true "file_path"
// @Param        count query int true "count"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name}/viewer/{file_path} [get]
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
