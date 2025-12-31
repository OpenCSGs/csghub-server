package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	bldprometheus "opencsg.com/csghub-server/builder/prometheus"
)

// WebhookMetrics returns a middleware that collects metrics for webhook requests
func WebhookMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Increment the total webhook requests counter
		if bldprometheus.WebhookRequestsTotal != nil {
			bldprometheus.WebhookRequestsTotal.Inc()
		}

		// Process the request
		c.Next()

		// Record the duration
		duration := time.Since(startTime).Seconds()
		if bldprometheus.WebhookRequestDuration != nil {
			bldprometheus.WebhookRequestDuration.WithLabelValues(
				c.Request.Method,
				c.FullPath(),
				string(rune(c.Writer.Status())),
			).Observe(duration)
		}
	}
}
