package middleware

import (
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
)

func CacheStrategyTrendingRepos() cache.Option {
	return cache.WithCacheStrategyByRequest(getCacheStrategyTrendingReposByRequest)
}

func getCacheStrategyTrendingReposByRequest(c *gin.Context) (bool, cache.Strategy) {
	// only cache anonymous users access the trending repositories
	if httpbase.GetCurrentUser(c) != "" {
		return false, cache.Strategy{}
	}

	sort := c.Query("sort")
	if sort != "trending" {
		return false, cache.Strategy{}
	}

	search := c.Query("search")
	if search != "" {
		return false, cache.Strategy{}
	}

	return true, cache.Strategy{
		CacheKey:      c.Request.RequestURI,
		CacheDuration: 2 * time.Minute,
	}
}

func CacheRepoInfo() cache.Option {
	return cache.WithCacheStrategyByRequest(getCacheStrategyRepoInfoByRequest)
}
