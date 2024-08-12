package handler

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewHFDatasetHandler(config *config.Config) (*HFDatasetHandler, error) {
	c, err := component.NewHFDatasetComponent(config)
	if err != nil {
		return nil, err
	}
	return &HFDatasetHandler{
		dc: c,
	}, nil
}

type HFDatasetHandler struct {
	dc *component.HFDatasetComponent
}

func (h *HFDatasetHandler) DatasetPathsInfo(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format for dataset path info", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ref := ctx.Param("ref")
	body, err := ctx.GetRawData()
	// body: paths=test-00000-of-00001.parquet&expand=True
	if err != nil {
		httpbase.BadRequest(ctx, "failed to read request body for dataset path")
		return
	}
	slog.Debug("Received body for dataset paths info", slog.Any("body", string(body)))

	currentUser := httpbase.GetCurrentUser(ctx)
	params, err := url.ParseQuery(string(body))
	if err != nil {
		slog.Error("error parsing query body", slog.Any("body", string(body)), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	var req types.PathReq
	req.Path = params.Get("paths")
	req.Expand = params.Get("expand") == "True"
	req.Namespace = namespace
	req.Name = name
	req.Ref = ref
	req.CurrentUser = currentUser
	slog.Debug("Received req for dataset paths info", slog.Any("req", req))

	res, err := h.dc.GetPathsInfo(ctx, req)
	if err != nil {
		slog.Error("fail to get dataset paths info", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
	}
	ctx.PureJSON(http.StatusOK, res)
}

func (h *HFDatasetHandler) DatasetTree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format for dataset tree", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ref := ctx.Param("ref")
	currentUser := httpbase.GetCurrentUser(ctx)
	pathInRepo := ctx.Param("path_in_repo")
	slog.Debug("get path_in_repo in repo", slog.Any("pathInRepo", pathInRepo))

	var req types.PathReq
	req.Namespace = namespace
	req.Name = name
	req.Ref = ref
	req.Path = pathInRepo
	req.CurrentUser = currentUser
	slog.Debug("received req for tree", slog.Any("req", req))

	tree, err := h.dc.GetDatasetTree(ctx, req)
	if err != nil {
		slog.Error("fail to get dataset tree", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
	}
	ctx.PureJSON(http.StatusOK, tree)
}

func (h *HFDatasetHandler) HandleHFYaml(ctx *gin.Context) {
	// request file: /api/datasets/haihui/ds1/resolve/7e6d54428819cb0f004fcc04e77a003a42c219b1/.huggingface.yaml
	// HF return: {"error":"Sorry, we can't find the page you are looking for."}
	response := types.HFErrorRes{
		Error: "Sorry, we can't find the page you are looking for.",
	}
	ctx.PureJSON(http.StatusOK, response)
}
