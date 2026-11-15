package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
)

// queryBool parses a boolean query parameter. An absent or empty parameter is
// treated as false. An invalid value is logged as an error and treated as
// false.
func queryBool(ctx *gin.Context, key string) bool {
	q := ctx.Query(key)
	if q == "" {
		return false
	}
	v, err := strconv.ParseBool(q)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), fmt.Sprintf("bad %s params", key), slog.Any(key, q), slog.Any("error", err))
		return false
	}
	return v
}
