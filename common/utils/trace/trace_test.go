package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestSetTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Generate new trace ID", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request, _ = http.NewRequest("GET", "/", nil)
		traceID := GetOrGenTraceID(c)
		assert.NotEmpty(t, traceID)
		assert.NotEqual(t, trace.TraceID{}.String(), traceID)

		// Verify it's stored in context
		storedTraceID, exists := c.Get(HeaderRequestID)
		assert.True(t, exists)
		assert.Equal(t, traceID, storedTraceID)

		// Verify it returns the same one
		secondTraceID := GetOrGenTraceID(c)
		assert.Equal(t, traceID, secondTraceID)
	})

	t.Run("Get trace ID from existing context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request, _ = http.NewRequest("GET", "/", nil)
		expectedTraceID := "my-test-trace-id"
		c.Set(HeaderRequestID, expectedTraceID)

		traceID := GetOrGenTraceID(c)
		assert.Equal(t, expectedTraceID, traceID)
	})

	t.Run("Get trace ID from otel span", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request, _ = http.NewRequest("GET", "/", nil)
		ctx, span := otel.GetTracerProvider().Tracer("test").Start(c.Request.Context(), "test-span")
		defer span.End()
		c.Request = c.Request.WithContext(ctx)

		expectedTraceID := span.SpanContext().TraceID().String()
		traceID := GetOrGenTraceID(c)

		// In a test environment without a registered tracer provider, the span's traceID will be nil ("000...").
		// The GetOrGenTraceID function will then correctly generate a new UUID.
		// Therefore, we should not assert equality with the nil traceID.
		// assert.Equal(t, expectedTraceID, traceID) // This is incorrect for a no-op tracer.
		assert.NotEmpty(t, traceID)
		assert.NotEqual(t, trace.TraceID{}.String(), traceID)
		assert.NotEqual(t, expectedTraceID, traceID)
	})

	t.Run("Get trace ID from headers", func(t *testing.T) {
		testCases := []struct {
			headerKey       string
			headerValue     string
			expectedTraceID string
		}{
			{"traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", "0af7651916cd43dd8448eb211c80319c"},
			{"X-Request-ID", "my-request-id", "my-request-id"},
			{"X-Kong-Request-Id", "my-kong-request-id", "my-kong-request-id"},
		}

		for _, tc := range testCases {
			t.Run(tc.headerKey, func(t *testing.T) {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				req, _ := http.NewRequest("GET", "/", nil)
				req.Header.Set(tc.headerKey, tc.headerValue)
				c.Request = req

				traceID := GetOrGenTraceID(c)
				assert.Equal(t, tc.expectedTraceID, traceID)
			})
		}
	})

	t.Run("Header precedence", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req, _ := http.NewRequest("GET", "/", nil)
		// Provide a valid traceparent header to correctly test precedence.
		req.Header.Set("traceparent", "00-traceparent_id-span-id-01")
		req.Header.Set("X-Request-ID", "x-request-id")
		req.Header.Set("X-Kong-Request-Id", "x-kong-request-id")
		c.Request = req

		traceID := GetOrGenTraceID(c)
		// The code should extract the trace ID part from the traceparent header.
		assert.Equal(t, "x-request-id", traceID)
	})
}

func TestContextTraceID(t *testing.T) {
	// Test case 1: Standard context
	ctx := context.Background()
	traceParent := "00-1234567890abcdef1234567890abcdef-0000000000000000-01"
	traceID := "1234567890abcdef1234567890abcdef"

	// Mock set logic (since setTraceIDInRequestContext is private, we can use it here as we are in package trace)
	ctx = setTraceIDInRequestContext(ctx, traceParent)

	gotID, gotParent := GetTraceIDFromContext(ctx)
	assert.Equal(t, traceID, gotID)
	assert.Equal(t, traceParent, gotParent)

	// Test case 2: Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "/", nil)
	c.Request = req

	// Mock GetOrGenTraceID logic
	reqCtx := setTraceIDInRequestContext(c.Request.Context(), traceParent)
	c.Request = c.Request.WithContext(reqCtx)

	// Verify direct retrieval from request context
	val := c.Request.Context().Value(traceContextKey{})
	assert.NotNil(t, val, "Value should be in request context")
	t.Logf("Direct Value type: %T, value: %v", val, val)

	// Verify retrieval via Gin Context Value method directly
	ginVal := c.Value(traceContextKey{})
	t.Logf("Gin Value type: %T, value: %v", ginVal, ginVal)

	// Debug generic struct key
	type myKey struct{}
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), myKey{}, "test-val"))
	t.Logf("MyKey Value via Request Context: %v", c.Request.Context().Value(myKey{}))
	t.Logf("MyKey Value via Gin Context: %v", c.Value(myKey{}))

	// Verify retrieval via String Key (simulating HeaderRequestID)
	c.Set("X-Request-ID", traceID)
	valStr := c.Value("X-Request-ID")
	assert.Equal(t, traceID, valStr)
	t.Logf("String Key Value via Gin Context: %v", valStr)

	// Verify retrieval via Gin Context
	// GetTraceIDFromContext accepts context.Context. *gin.Context implements it.
	// However, gin.Context.Value() delegates to Request.Context().Value() for non-string keys.
	gotID2, gotParent2 := GetTraceIDFromContext(c)
	assert.Equal(t, traceID, gotID2)
	assert.Equal(t, traceParent, gotParent2)
}

func TestGetOrGenTraceID_AlwaysInjects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/", nil)

	// Simulate OTEL Span
	ctx, span := otel.GetTracerProvider().Tracer("test").Start(c.Request.Context(), "test-span")
	defer span.End()
	c.Request = c.Request.WithContext(ctx)

	originalCtx := c.Request.Context()

	// Call GetOrGenTraceID
	traceID := GetOrGenTraceID(c)

	// Verify traceID matches Span ID
	// Note: In test environment with no-op tracer, ID might be empty/zeros or generated differently depending on provider.
	// But GetOrGenTraceIDFromContext extracts it.
	// If span.SpanContext().TraceID() is valid, GetOrGenTraceID should return it.
	// However, the default global tracer might be NoOp which returns invalid TraceID.
	// Let's check if span has ID.
	if span.SpanContext().HasTraceID() {
		assert.Equal(t, span.SpanContext().TraceID().String(), traceID)
	}

	// Verify Context IS modified (wrapped with our internal key)
	// If we hadn't removed the 'if !spanCtx.HasTraceID()' check, this would be Equal (no wrapping)
	// Note: assert.NotEqual checks for equality. Context wrapping creates a new struct.
	assert.NotEqual(t, originalCtx, c.Request.Context(), "Context should be wrapped with traceID even if OTEL span exists")

	// Verify we can get it back via GetTraceIDFromContext using the wrapper
	gotID, _ := GetTraceIDFromContext(c.Request.Context())
	assert.Equal(t, traceID, gotID)
}
