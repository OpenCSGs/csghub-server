//go:build ee || saas

package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/api/httpbase"
)

const (
	correlationIDHeader       = "X-Correlation-ID"
	postHogDistinctIDHeader   = "X-PostHog-Distinct-Id"
	postHogSessionIDHeader    = "X-PostHog-Session-Id"
	analyticsTemplateIDHeader = "X-Analytics-Template-ID"
)

type requestAnalyticsContext struct {
	DistinctID    string
	SessionID     string
	CorrelationID string
	TemplateID    string
}

func analyticsContextFromRequest(ctx *gin.Context) requestAnalyticsContext {
	correlationID := strings.TrimSpace(ctx.GetHeader(correlationIDHeader))
	if _, err := uuid.Parse(correlationID); err != nil {
		correlationID = ""
	}

	distinctID := strings.TrimSpace(ctx.GetHeader(postHogDistinctIDHeader))
	if authenticatedUserUUID := strings.TrimSpace(httpbase.GetCurrentUserUUID(ctx)); authenticatedUserUUID != "" {
		distinctID = authenticatedUserUUID
	}

	return requestAnalyticsContext{
		DistinctID:    distinctID,
		SessionID:     strings.TrimSpace(ctx.GetHeader(postHogSessionIDHeader)),
		CorrelationID: correlationID,
		TemplateID:    strings.TrimSpace(ctx.GetHeader(analyticsTemplateIDHeader)),
	}
}
