package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
)

func XOpenCSGHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		s3Internal := c.GetHeader("X-OPENCSG-S3-Internal")
		if s3Internal == "true" {
			c.Set("X-OPENCSG-S3-Internal", true)
			ctx := context.WithValue(c.Request.Context(), "X-OPENCSG-S3-Internal", true)
			c.Request = c.Request.WithContext(ctx)
		}
	}
}
