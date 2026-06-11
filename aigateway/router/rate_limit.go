package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func aigatewayRateLimitHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"code":    "rate_limit_exceeded",
			"message": "Rate limit exceeded. Please wait and retry.",
			"type":    "rate_limit_error",
		},
	})
}
