package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/logcollector/component"
)

type LogCollectorHandler struct {
	collector component.LogFactory
}

func NewLogCollectorHandler(collector component.LogFactory) (*LogCollectorHandler, error) {
	return &LogCollectorHandler{collector: collector}, nil
}

// @Summary Get LogCollector service health status and statistics
// @Description Returns the current health status
// @Tags LogCollector
// @Accept json
// @Produce json
// @Success 200 {object} nil
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /logcollector/health [get]

func (clh *LogCollectorHandler) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, nil)
}

// @Summary Get LogCollector service statistics
// @Description Returns the current statistics of the LogCollector service.
// @Tags LogCollector
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "LogCollector service statistics"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /logcollector/health [get]

func (clh *LogCollectorHandler) GetStats(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, clh.collector.GetStats())
}
