package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bldprometheus "opencsg.com/csghub-server/builder/prometheus"
)

func TestWebhookMetrics(t *testing.T) {
	// Initialize metrics for testing
	bldprometheus.InitMetrics()

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		setupHandler   func(*gin.RouterGroup)
	}{
		{
			name:           "Successful POST request",
			method:         "POST",
			path:           "/webhook/runner",
			expectedStatus: http.StatusOK,
			setupHandler: func(rg *gin.RouterGroup) {
				rg.POST("/runner", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"message": "webhook received"})
				})
			},
		},
		{
			name:           "Failed request with 500 status",
			method:         "POST",
			path:           "/webhook/runner",
			expectedStatus: http.StatusInternalServerError,
			setupHandler: func(rg *gin.RouterGroup) {
				rg.POST("/runner", func(c *gin.Context) {
					c.AbortWithStatus(http.StatusInternalServerError)
				})
			},
		},
		{
			name:           "GET request (should still work)",
			method:         "GET",
			path:           "/webhook/runner",
			expectedStatus: http.StatusOK,
			setupHandler: func(rg *gin.RouterGroup) {
				rg.GET("/runner", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"message": "ok"})
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new router with webhook metrics middleware
			router := gin.New()
			router.Use(WebhookMetrics())

			webhookGroup := router.Group("/webhook")

			tt.setupHandler(webhookGroup)

			// Get initial metric values
			initialRequestsCount := getMetricValue(t, bldprometheus.WebhookRequestsTotal)
			// Create and execute request
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response status
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Verify metrics were incremented
			finalRequestsCount := getMetricValue(t, bldprometheus.WebhookRequestsTotal)

			assert.Greater(t, finalRequestsCount, initialRequestsCount, "WebhookRequestsTotal should be incremented")
		})
	}
}

func TestWebhookMetricsWithNilMetrics(t *testing.T) {
	// Test behavior when metrics are nil (before InitMetrics)
	// Store original values
	originalRequestsTotal := bldprometheus.WebhookRequestsTotal
	originalRequestDuration := bldprometheus.WebhookRequestDuration

	// Set metrics to nil
	defer func() {
		bldprometheus.WebhookRequestsTotal = originalRequestsTotal
		bldprometheus.WebhookRequestDuration = originalRequestDuration
	}()

	bldprometheus.WebhookRequestsTotal = nil
	bldprometheus.WebhookRequestDuration = nil

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(WebhookMetrics())

	router.POST("/webhook/runner", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "webhook received"})
	})

	req, _ := http.NewRequest("POST", "/webhook/runner", nil)
	w := httptest.NewRecorder()

	// Should not panic when metrics are nil
	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusOK, w.Code)
}

// Helper function to get metric value
func getMetricValue(t *testing.T, counter prometheus.Counter) float64 {
	if counter == nil {
		return 0
	}

	dto := &dto.Metric{}
	err := counter.Write(dto)
	require.NoError(t, err)
	return dto.Counter.GetValue()
}
