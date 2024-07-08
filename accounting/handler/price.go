package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func NewPriceHandler() (*PriceHandler, error) {
	return &PriceHandler{
		apc: component.NewAccountingPrice(),
	}, nil
}

type PriceHandler struct {
	apc *component.AccountingPriceComponent
}

func (ph *PriceHandler) QueryPricesBySKUTypeAndResourceID(ctx *gin.Context) {
	skyStr := ctx.Query("sku_type")
	skuInt, err := strconv.Atoi(skyStr)
	if err != nil {
		slog.Error("Bad request sku type format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	resID := ctx.Query("resource_id")
	req := types.ACCT_PRICE_REQ{
		SKUType:    types.SKUType(skuInt),
		ResourceID: resID,
		Per:        per,
		Page:       page,
	}
	var (
		prices []database.AccountPrice
		total  int
	)
	if len(resID) < 1 {
		prices, total, err = ph.apc.ListPricesBySKUType(ctx, req)
	} else {
		prices, total, err = ph.apc.ListPricesBySKUTypeAndResourceID(ctx, req)
	}
	if err != nil {
		slog.Error("fail to query prices by sku type and resource id", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  prices,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}

func (ph *PriceHandler) GetPriceByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		slog.Error("Bad request id format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("Bad request id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	price, err := ph.apc.GetPriceByID(ctx, int64(id))
	if err != nil {
		slog.Error("fail to query price by id", slog.Any("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, price)
}

func (ph *PriceHandler) PriceCreate(ctx *gin.Context) {
	var req types.ACCT_PRICE
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad price request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	price, err := ph.apc.CreatePrice(ctx, req)
	if err != nil {
		slog.Error("fail to add price", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, price)
}

func (ph *PriceHandler) PriceUpdate(ctx *gin.Context) {
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		slog.Error("Bad request id format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("Bad request id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req types.ACCT_PRICE
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad price request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	price, err := ph.apc.UpdatePrice(ctx, req, int64(id))
	if err != nil {
		slog.Error("fail to update price", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, price)
}

func (ph *PriceHandler) PriceDelete(ctx *gin.Context) {
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		slog.Error("Bad request id format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("Bad request id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = ph.apc.DeletePrice(ctx, int64(id))
	if err != nil {
		slog.Error("fail to delete price by id", slog.Any("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
