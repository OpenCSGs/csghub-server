//go:build !ee && !saas

package handler

import "github.com/gin-gonic/gin"

type requestAnalyticsContext struct {
	DistinctID    string
	SessionID     string
	CorrelationID string
	TemplateID    string
}

// analyticsContextFromRequest is intentionally a no-op in CE builds.
func analyticsContextFromRequest(*gin.Context) requestAnalyticsContext {
	return requestAnalyticsContext{}
}
