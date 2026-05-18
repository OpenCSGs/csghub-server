package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseFloatRangeFromContext parses optional min/max float query parameters
// and validates that min <= max when both are provided.
func parseFloatRangeFromContext(ctx *gin.Context, minKey, maxKey string) (*float64, *float64, error) {
	minValue, err := parseOptionalFloatQuery(ctx, minKey)
	if err != nil {
		return nil, nil, err
	}
	maxValue, err := parseOptionalFloatQuery(ctx, maxKey)
	if err != nil {
		return nil, nil, err
	}
	if minValue != nil && maxValue != nil && *minValue > *maxValue {
		return nil, nil, fmt.Errorf("%s must be less than or equal to %s", minKey, maxKey)
	}
	return minValue, maxValue, nil
}

// parseOptionalFloatQuery parses an optional float query parameter.
// Returns nil if the parameter is not provided or empty.
func parseOptionalFloatQuery(ctx *gin.Context, key string) (*float64, error) {
	raw := ctx.Query(key)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid number", key)
	}
	if value < 0 {
		return nil, fmt.Errorf("%s must be greater than or equal to 0", key)
	}
	return &value, nil
}

// parseInt64RangeFromContext parses optional min/max int64 query parameters
// and validates that min <= max when both are provided.
func parseInt64RangeFromContext(ctx *gin.Context, minKey, maxKey string) (*int64, *int64, error) {
	minValue, err := parseOptionalInt64Query(ctx, minKey)
	if err != nil {
		return nil, nil, err
	}
	maxValue, err := parseOptionalInt64Query(ctx, maxKey)
	if err != nil {
		return nil, nil, err
	}
	if minValue != nil && maxValue != nil && *minValue > *maxValue {
		return nil, nil, fmt.Errorf("%s must be less than or equal to %s", minKey, maxKey)
	}
	return minValue, maxValue, nil
}

// parseOptionalInt64Query parses an optional int64 query parameter.
// Returns nil if the parameter is not provided or empty.
func parseOptionalInt64Query(ctx *gin.Context, key string) (*int64, error) {
	raw := ctx.Query(key)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid integer", key)
	}
	if value < 0 {
		return nil, fmt.Errorf("%s must be greater than or equal to 0", key)
	}
	return &value, nil
}
