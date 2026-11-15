package anthropic

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// The messages protocol renders the per-user rate limit rejection as 429
// rate_limit_error with a Retry-After hint at the window rollover.
func TestHandlePlanError_UserRateLimitExceeded(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	h := &Handler{}
	rateLimitErr := &types.UserRateLimitExceededError{
		Scope:             types.UserRateLimitScopeModel,
		Dimension:         types.UserRateLimitDimensionTPM,
		ModelID:           "test-model",
		RetryAfterSeconds: 37,
	}

	h.HandlePlanError(c, &types.RequestMetadata{}, &types.RequestPlan{
		ErrorCode: types.PlanErrUserRateLimitExceeded,
	}, rateLimitErr)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != strconv.FormatInt(37, 10) {
		t.Errorf("expected Retry-After 37, got %q", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, "rate_limit_error") {
		t.Errorf("expected rate_limit_error type, got %q", body)
	}
	if !strings.Contains(body, "user TPM rate limit exceeded on model scope") {
		t.Errorf("expected scope+dimension message, got %q", body)
	}
}

// Wrapped errors are still attributed: errors.As must see through fmt.Errorf.
func TestHandlePlanError_UserRateLimitExceeded_Wrapped(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	h := &Handler{}
	err := errors.Join(&types.UserRateLimitExceededError{
		Scope:             types.UserRateLimitScopeGlobal,
		Dimension:         types.UserRateLimitDimensionRPM,
		RetryAfterSeconds: 21,
	})

	h.HandlePlanError(c, &types.RequestMetadata{}, &types.RequestPlan{
		ErrorCode: types.PlanErrUserRateLimitExceeded,
	}, err)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != strconv.FormatInt(21, 10) {
		t.Errorf("expected Retry-After 21, got %q", got)
	}
}
