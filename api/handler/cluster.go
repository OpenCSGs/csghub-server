package handler

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	code "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewClusterHandler(config *config.Config) (*ClusterHandler, error) {
	ncc, err := component.NewClusterComponent(config)
	if err != nil {
		return nil, err
	}
	return &ClusterHandler{
		c: ncc,
	}, nil
}

type ClusterHandler struct {
	c component.ClusterComponent
}

const (
	deployTimeLayout     = "2006-01-02 15:04:05"
	deployDateOnlyLayout = "2006-01-02"
)

// Getclusters   godoc
// @Security     ApiKey
// @Summary      Get cluster list
// @Description  Get cluster list
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster [get]
func (h *ClusterHandler) Index(ctx *gin.Context) {
	clusters, err := h.c.Index(ctx.Request.Context())
	if err != nil {
		slog.Error("Failed to get cluster list", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, clusters)
}

// GetClusterById   godoc
// @Security     ApiKey
// @Summary      Get cluster by id
// @Description  Get cluster by id
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/{id} [get]
func (h *ClusterHandler) GetClusterById(ctx *gin.Context) {
	id := ctx.Param("id")
	cluster, err := h.c.GetClusterWithResourceByID(ctx.Request.Context(), id)
	if err != nil {
		slog.Error("Failed to get cluster", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, cluster)
}

// GetClusterUsage   godoc
// @Security     ApiKey
// @Summary      Get all cluster usage
// @Description  Get all cluster usage
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/usage [get]
func (h *ClusterHandler) GetClusterUsage(ctx *gin.Context) {
	usages, err := h.c.GetClusterUsages(ctx.Request.Context())
	if err != nil {
		slog.Error("Failed to get cluster usage", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, usages)
}

// GetClusterDeploys  godoc
// @Security     ApiKey
// @Summary      Get cluster deploys
// @Description  Get cluster deploys
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Param        per query int false "per" default(50)
// @Param        page query int false "page index" default(1)
// @Param        status query string false "status" default(all) Enums(all, running, stopped, deployfailed)
// @Param        search query string false "search" default("")
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/deploys [get]
func (h *ClusterHandler) GetDeploys(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var req types.DeployReq
	req.DeployTypes = []int{types.SpaceType, types.InferenceType, types.FinetuneType}
	req.Page = page
	req.PageSize = per
	status := ctx.Query("status")
	switch status {
	case "running":
		req.Status = []int{code.Running}
	case "stopped":
		req.Status = []int{code.Stopped}
	case "deployfailed":
		req.Status = []int{code.DeployFailed}
	}
	req.Query = ctx.Query("search")
	deploys, total, err := h.c.GetDeploys(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get cluster deploys", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OKWithTotal(ctx, deploys, total)
}

// GetClusterDeploysReport  godoc
// @Security     ApiKey
// @Summary      Export cluster deploys as CSV
// @Description  Export all cluster deploys (Excel-readable CSV) with streaming output
// @Tags         Cluster
// @Produce      text/csv
// @Param        status query string false "status" default(all) Enums(all, running, stopped, deployfailed)
// @Param        search query string false "search" default("")
// @Param        start_time query string false "filter deploys created after or at this time"
// @Param        end_time query string false "filter deploys created before or at this time"
// @Success      200  {string}  string "CSV file"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/deploys_report [get]
func (h *ClusterHandler) GetDeploysReport(ctx *gin.Context) {
	var req types.DeployReq
	req.DeployTypes = []int{types.SpaceType, types.InferenceType, types.FinetuneType}
	req.Page = 1
	req.PageSize = 10
	status := ctx.Query("status")
	switch status {
	case "running":
		req.Status = []int{code.Running}
	case "stopped":
		req.Status = []int{code.Stopped}
	case "deployfailed":
		req.Status = []int{code.DeployFailed}
	}
	req.Query = ctx.Query("search")
	if err := bindDeployDateRange(ctx, &req); err != nil {
		slog.Error("Invalid date range for deploy report", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filename := "deploys_report.csv"
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Status(http.StatusOK)

	writer := csv.NewWriter(ctx.Writer)
	defer writer.Flush()

	_ = writer.Write([]string{
		"ClusterID",
		"ClusterRegion",
		"DeployName",
		"Username",
		"Resource",
		"CreateTime",
		"Status",
		"TotalTimeInMin",
		"TotalFeeInCents",
	})
	writer.Flush()

	totalProcessed := 0

	for {
		deploys, total, err := h.c.GetDeploys(ctx.Request.Context(), req)
		if err != nil {
			slog.Error("Failed to get cluster deploys", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
			return
		}

		for _, d := range deploys {
			_ = writer.Write([]string{
				d.ClusterID,
				d.ClusterRegion,
				d.DeployName,
				d.User.Username,
				d.Resource,
				d.CreateTime.Local().Format(deployTimeLayout),
				d.Status,
				strconv.Itoa(d.TotalTimeInMin),
				strconv.Itoa(d.TotalFeeInCents),
			})
			totalProcessed++
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			slog.Error("Failed to write csv", slog.Any("error", err))
			return
		}

		if totalProcessed >= total || len(deploys) == 0 {
			break
		}
		req.Page++
	}

	slog.Info("CSV export completed", slog.Int("total_processed", totalProcessed))
}

func (h *ClusterHandler) Update(ctx *gin.Context) {
	var req types.ClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ClusterID = ctx.Param("id")
	result, err := h.c.Update(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to update cluster info", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, result)
}

func bindDeployDateRange(ctx *gin.Context, req *types.DeployReq) error {
	startTime := ctx.Query("start_time")
	endTime := ctx.Query("end_time")
	if startTime == "" && endTime == "" {
		return nil
	}
	if startTime == "" || endTime == "" {
		return fmt.Errorf("start_time and end_time must be provided together")
	}
	parsedStart, err := parseDeployQueryTime(startTime, false)
	if err != nil {
		return err
	}
	parsedEnd, err := parseDeployQueryTime(endTime, true)
	if err != nil {
		return err
	}
	req.StartTime = &parsedStart
	req.EndTime = &parsedEnd
	return nil
}

func parseDeployQueryTime(value string, isEnd bool) (time.Time, error) {
	layouts := []string{deployTimeLayout, deployDateOnlyLayout}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.UTC)
		if err != nil {
			continue
		}
		if layout == deployDateOnlyLayout {
			if isEnd {
				parsed = parsed.Add(24*time.Hour - time.Nanosecond)
			}
			return parsed, nil
		}
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid datetime format, use '%s' or '%s'", deployTimeLayout, deployDateOnlyLayout)
}
