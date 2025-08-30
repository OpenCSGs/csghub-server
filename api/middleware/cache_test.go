package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
)

func TestCacheStrategyTrendingRepos(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		user          string
		sortParam     string
		searchParam   string
		expectedCache bool
	}{
		{
			name:          "Anonymous user, trending sort, no search",
			user:          "",
			sortParam:     "trending",
			searchParam:   "",
			expectedCache: true,
		},
		{
			name:          "Logged-in user, trending sort, no search",
			user:          "testuser",
			sortParam:     "trending",
			searchParam:   "",
			expectedCache: false,
		},
		{
			name:          "Anonymous user, non-trending sort, no search",
			user:          "",
			sortParam:     "popular",
			searchParam:   "",
			expectedCache: false,
		},
		{
			name:          "Anonymous user, trending sort, with search",
			user:          "",
			sortParam:     "trending",
			searchParam:   "testquery",
			expectedCache: false,
		},
		{
			name:          "Anonymous user, no sort, no search",
			user:          "",
			sortParam:     "",
			searchParam:   "",
			expectedCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a memory store for testing
			store := persist.NewMemoryStore(time.Minute)

			// Create a test router with cache middleware
			router := gin.New()

			// Counter to track how many times the handler is called
			callCount := 0
			testHandler := gin.HandlerFunc(func(c *gin.Context) {
				callCount++
				c.JSON(200, gin.H{"message": "test", "call": callCount})
			})

			// Apply cache middleware with our strategy
			cacheMiddleware := cache.Cache(store, 2*time.Minute, CacheStrategyTrendingRepos())
			router.GET("/test", func(c *gin.Context) {
				// Set user context if provided
				if tt.user != "" {
					c.Set(httpbase.CurrentUserCtxVar, tt.user)
				}
				// Call next to continue to cache middleware
				c.Next()
			}, cacheMiddleware, testHandler)

			// Build the request URL
			url := "/test"
			if tt.sortParam != "" || tt.searchParam != "" {
				url += "?sort=" + tt.sortParam + "&search=" + tt.searchParam
			}

			// First request
			req1, _ := http.NewRequest(http.MethodGet, url, nil)
			w1 := httptest.NewRecorder()
			router.ServeHTTP(w1, req1)

			assert.Equal(t, 200, w1.Code)
			initialCallCount := callCount

			if tt.expectedCache {
				// For cached responses, make a second identical request
				// The handler should NOT be called again if caching is working
				req2, _ := http.NewRequest(http.MethodGet, url, nil)
				w2 := httptest.NewRecorder()
				router.ServeHTTP(w2, req2)

				assert.Equal(t, 200, w2.Code)
				// Call count should remain the same for cached response
				assert.Equal(t, initialCallCount, callCount, "Handler should not be called again for cached response")
			} else {
				// For non-cached responses, make a second identical request
				// The handler SHOULD be called again
				req2, _ := http.NewRequest(http.MethodGet, url, nil)
				w2 := httptest.NewRecorder()
				router.ServeHTTP(w2, req2)

				assert.Equal(t, 200, w2.Code)
				// Call count should increase for non-cached response
				assert.Greater(t, callCount, initialCallCount, "Handler should be called again for non-cached response")
			}
		})
	}
}

// Test the cache strategy function directly by replicating its logic
func TestCacheStrategyLogic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		user          string
		sortParam     string
		searchParam   string
		expectedCache bool
		expectedKey   string
	}{
		{
			name:          "Anonymous user, trending sort, no search",
			user:          "",
			sortParam:     "trending",
			searchParam:   "",
			expectedCache: true,
			expectedKey:   "/test?sort=trending&search=",
		},
		{
			name:          "Logged-in user, trending sort, no search",
			user:          "testuser",
			sortParam:     "trending",
			searchParam:   "",
			expectedCache: false,
			expectedKey:   "",
		},
		{
			name:          "Anonymous user, non-trending sort, no search",
			user:          "",
			sortParam:     "popular",
			searchParam:   "",
			expectedCache: false,
			expectedKey:   "",
		},
		{
			name:          "Anonymous user, trending sort, with search",
			user:          "",
			sortParam:     "trending",
			searchParam:   "testquery",
			expectedCache: false,
			expectedKey:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Simulate request with query parameters
			url := "/test?sort=" + tt.sortParam + "&search=" + tt.searchParam
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req.RequestURI = url // Manually set RequestURI since http.NewRequest doesn't set it
			c.Request = req

			// Set the current user in context if provided
			if tt.user != "" {
				c.Set(httpbase.CurrentUserCtxVar, tt.user)
			}

			// Test the cache strategy logic directly
			shouldCache, strategy := getCacheStrategyTrendingReposByRequest(c)

			assert.Equal(t, tt.expectedCache, shouldCache)
			if tt.expectedCache {
				assert.Equal(t, tt.expectedKey, strategy.CacheKey)
				assert.Equal(t, 2*time.Minute, strategy.CacheDuration)
			} else {
				assert.Empty(t, strategy.CacheKey)
				assert.Zero(t, strategy.CacheDuration)
			}
		})
	}
}
