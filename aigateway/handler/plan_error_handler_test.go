package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

func newPlanErrorContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)
	return c, w
}

func TestHandleOpenAIPlanError_UnknownDoesNotLeakInternalMessage(t *testing.T) {
	c, w := newPlanErrorContext(t)
	meta := &types.RequestMetadata{Model: "test-model"}
	// PlanErrUnknown with an internal-looking error string.
	p := &types.RequestPlan{ErrorCode: types.PlanErrUnknown}
	internalErr := errors.New("database connection string postgres://user:secret@db:5432")

	handleOpenAIPlanError(c, meta, p, internalErr, "")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var body struct {
		Error types.Error `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Error.Message != "an internal error occurred while processing the request" {
		t.Errorf("expected generic internal message, got %q", body.Error.Message)
	}
	for _, sub := range []string{"postgres", "secret", "database"} {
		if strings.Contains(body.Error.Message, sub) {
			t.Errorf("internal error details leaked into response: %q", body.Error.Message)
		}
	}
}

func TestHandleOpenAIPlanError_NilPlanDoesNotLeakInternalMessage(t *testing.T) {
	c, w := newPlanErrorContext(t)
	meta := &types.RequestMetadata{Model: "test-model"}
	internalErr := errors.New("internal: connection refused to 10.0.0.5:6379")

	// p == nil falls through to the fallback path.
	handleOpenAIPlanError(c, meta, nil, internalErr, "")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	var body struct {
		Error types.Error `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	for _, sub := range []string{"10.0.0.5", "6379", "connection refused"} {
		if strings.Contains(body.Error.Message, sub) {
			t.Errorf("internal error details leaked into response: %q", body.Error.Message)
		}
	}
}

func TestHandleOpenAIPlanError_ModelNotFound(t *testing.T) {
	c, w := newPlanErrorContext(t)
	meta := &types.RequestMetadata{Model: "test-model"}
	p := &types.RequestPlan{ErrorCode: types.PlanErrModelNotFound}

	handleOpenAIPlanError(c, meta, p, errors.New("model not found"), "")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHandleOpenAIPlanError_InsufficientBalanceWithEmptyFrontendURL(t *testing.T) {
	c, w := newPlanErrorContext(t)
	meta := &types.RequestMetadata{Model: "test-model"}
	p := &types.RequestPlan{ErrorCode: types.PlanErrInsufficientBalance}

	handleOpenAIPlanError(c, meta, p, errors.New("insufficient balance"), "")

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, w.Code)
	}
	// Empty frontendURL produces a relative recharge link, not a generic message.
	if !strings.Contains(w.Body.String(), "/settings/recharge-payment") {
		t.Errorf("expected relative recharge link in body, got %q", w.Body.String())
	}
}
