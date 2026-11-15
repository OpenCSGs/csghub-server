package jev

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// The systemone protocol renders the per-user rate limit rejection as 429
// rate_limit_error; the code field carries the SCOPE (global vs model), so
// clients can tell the gateway-global fallback from a model's own limit.
func TestHandlePlanError_UserRateLimitExceeded(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	h := &Handler{}
	rateLimitErr := &types.UserRateLimitExceededError{
		Scope:             types.UserRateLimitScopeGlobal,
		Dimension:         types.UserRateLimitDimensionRPM,
		RetryAfterSeconds: 14,
	}

	h.HandlePlanError(c, &types.RequestMetadata{}, &types.RequestPlan{
		ErrorCode: types.PlanErrUserRateLimitExceeded,
	}, rateLimitErr)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != strconv.FormatInt(14, 10) {
		t.Errorf("expected Retry-After 14, got %q", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, "user_g_rate_limit_exceeded") {
		t.Errorf("expected user_g_rate_limit_exceeded code, got %q", body)
	}
	if !strings.Contains(body, "rate_limit_error") {
		t.Errorf("expected rate_limit_error type, got %q", body)
	}
}

// A model-scope rejection carries the model-scope code.
func TestHandlePlanError_UserRateLimitExceeded_ModelScope(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	h := &Handler{}
	rateLimitErr := &types.UserRateLimitExceededError{
		Scope:             types.UserRateLimitScopeModel,
		Dimension:         types.UserRateLimitDimensionTPM,
		ModelID:           "test-model",
		RetryAfterSeconds: 9,
	}

	h.HandlePlanError(c, &types.RequestMetadata{}, &types.RequestPlan{
		ErrorCode: types.PlanErrUserRateLimitExceeded,
	}, rateLimitErr)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
	if !strings.Contains(w.Body.String(), "user_rate_limit_exceeded") {
		t.Errorf("expected user_rate_limit_exceeded code, got %q", w.Body.String())
	}
}
