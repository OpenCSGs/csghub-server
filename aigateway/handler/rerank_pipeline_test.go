package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
)

func TestRerankPipeline_Extract_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(`{"model":"test-model","query":"hello","documents":["doc1","doc2"]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(httpbase.CurrentUserCtxVar, "testuser")
	c.Set(httpbase.CurrentNamespaceUUIDVar, "ns-123")
	c.Set(httpbase.AccessTokenCtxVar, "apikey-xxx")

	meta, err := h.Extract(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Model != "test-model" {
		t.Errorf("expected model 'test-model', got '%s'", meta.Model)
	}
	if meta.Task != "rerank" {
		t.Errorf("expected task 'rerank', got '%s'", meta.Task)
	}
	if meta.TenantID != "ns-123" {
		t.Errorf("expected tenant 'ns-123', got '%s'", meta.TenantID)
	}
	if meta.Streaming {
		t.Error("expected streaming=false")
	}
	req, ok := meta.ParsedBody.(*types.RerankRequest)
	if !ok {
		t.Fatalf("expected ParsedBody to be *types.RerankRequest")
	}
	if req.Query != "hello" {
		t.Errorf("expected query 'hello', got '%s'", req.Query)
	}
}

func TestRerankPipeline_Extract_MissingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(`{"query":"hello","documents":["doc1"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := h.Extract(c)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestRerankPipeline_Extract_MissingQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(`{"model":"test","documents":["doc1"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := h.Extract(c)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestRerankPipeline_Extract_MissingDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(`{"model":"test","query":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := h.Extract(c)
	if err == nil {
		t.Fatal("expected error for missing documents")
	}
}

func TestRerankPipeline_HandlePlanError_ModelNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)

	meta := &types.RequestMetadata{Model: "unknown-model"}
	p := &types.RequestPlan{ErrorCode: types.PlanErrModelNotFound}

	h.HandlePlanError(c, meta, p, errRerankModelNotFound)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestRerankPipeline_HandlePlanError_InsufficientBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)

	meta := &types.RequestMetadata{Model: "test-model"}
	p := &types.RequestPlan{ErrorCode: types.PlanErrInsufficientBalance}

	h.HandlePlanError(c, meta, p, errRerankInsufficientBalance)

	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected status %d, got %d", http.StatusPaymentRequired, w.Code)
	}
}

func TestRerankPipeline_HandlePlanError_Sensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &rerankPipelineHandler{handler: &OpenAIHandlerImpl{}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)

	meta := &types.RequestMetadata{Model: "test-model"}
	p := &types.RequestPlan{
		ErrorCode: types.PlanErrSensitive,
		Safety:    &types.SafetyDecision{IsSensitive: true, Message: "blocked by policy"},
	}

	h.HandlePlanError(c, meta, p, errRerankSensitive)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

var (
	errRerankModelNotFound     = &testErr{msg: "model not found"}
	errRerankInsufficientBalance = &testErr{msg: "insufficient balance"}
	errRerankSensitive         = &testErr{msg: "content blocked"}
)

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
