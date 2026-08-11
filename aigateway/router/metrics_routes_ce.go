//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

// newMetricsMiddleware returns a no-op handler when the metrics feature is not
// compiled in (ce build).  This allows the router to unconditionally include
// the middleware in route chains without conditionally registering routes.
//
// Returns a no-op cleanup function alongside the handler to match the EE/SaaS
// signature.
func newMetricsMiddleware(_ *config.Config) (gin.HandlerFunc, func()) {
	return func(c *gin.Context) { c.Next() }, func() {}
}
