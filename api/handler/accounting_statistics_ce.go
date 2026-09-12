//go:build !ee && !saas

package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
)

func (ah *AccountingHandler) QueryStatisticsByUserID(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}

func (ah *AccountingHandler) QueryStatisticsDetailByUserID(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}

func (ah *AccountingHandler) QueryStatisticsSummary(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}
