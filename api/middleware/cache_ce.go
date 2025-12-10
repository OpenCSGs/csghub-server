//go:build !saas

package middleware

import (
	cache "github.com/chenyahui/gin-cache"
	"github.com/gin-gonic/gin"
)

func getCacheStrategyRepoInfoByRequest(c *gin.Context) (bool, cache.Strategy) {
	return false, cache.Strategy{}
}
