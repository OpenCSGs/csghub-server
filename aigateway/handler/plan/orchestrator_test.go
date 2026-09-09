package plan

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

// --- Test doubles ---

type stubExtractor struct {
	meta *types.RequestMetadata
	err  error
}

func (s *stubExtractor) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	return s.meta, s.err
}

type stubHandler struct {
	executed       bool
	execErr        error
	planErrorSeen  bool
	planErrorErr   error
	executeMeta    *types.RequestMetadata
	executePlan    *types.RequestPlan
	planErrorMeta  *types.RequestMetadata
	planErrorPlan  *types.RequestPlan
}

func (s *stubHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	s.executed = true
	s.executeMeta = meta
	s.executePlan = p
	return s.execErr
}

func (s *stubHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	s.planErrorSeen = true
	s.planErrorErr = err
	s.planErrorMeta = meta
	s.planErrorPlan = p
}

type stubPlanner struct {
	plan *types.RequestPlan
	err  error
}

func (s *stubPlanner) Plan(ctx context.Context, meta *types.RequestMetadata) (*types.RequestPlan, error) {
	return s.plan, s.err
}

// --- Tests ---

func TestOrchestrator_ExtractError_ShortCircuits(t *testing.T) {
	orch := NewOrchestrator(&stubPlanner{plan: &types.RequestPlan{}})
	extractor := &stubExtractor{err: errors.New("bad request")}
	handler := &stubHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	orch.Dispatch(c, extractor, handler)

	assert.False(t, handler.executed)
	assert.False(t, handler.planErrorSeen)
}

func TestOrchestrator_PlanError_CallsHandlePlanError(t *testing.T) {
	plan := &types.RequestPlan{ErrorCode: types.PlanErrInsufficientBalance}
	orch := NewOrchestrator(&stubPlanner{plan: plan, err: errors.New("insufficient balance")})
	extractor := &stubExtractor{meta: &types.RequestMetadata{Model: "test"}}
	handler := &stubHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	orch.Dispatch(c, extractor, handler)

	assert.False(t, handler.executed)
	assert.True(t, handler.planErrorSeen)
	assert.Equal(t, plan, handler.planErrorPlan)
	assert.NotNil(t, handler.planErrorMeta)
}

func TestOrchestrator_Success_CallsExecute(t *testing.T) {
	plan := &types.RequestPlan{ModelTarget: &types.ModelTarget{ModelName: "test"}}
	orch := NewOrchestrator(&stubPlanner{plan: plan})
	extractor := &stubExtractor{meta: &types.RequestMetadata{Model: "test"}}
	handler := &stubHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	orch.Dispatch(c, extractor, handler)

	assert.True(t, handler.executed)
	assert.False(t, handler.planErrorSeen)
	assert.Equal(t, plan, handler.executePlan)
	assert.NotNil(t, handler.executeMeta)
}

func TestOrchestrator_ExecuteError_DoesNotCallHandlePlanError(t *testing.T) {
	// Execute errors are logged but not routed to HandlePlanError —
	// the protocol handler should handle its own execute errors inline.
	plan := &types.RequestPlan{ModelTarget: &types.ModelTarget{ModelName: "test"}}
	orch := NewOrchestrator(&stubPlanner{plan: plan})
	extractor := &stubExtractor{meta: &types.RequestMetadata{Model: "test"}}
	handler := &stubHandler{execErr: errors.New("proxy failed")}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	orch.Dispatch(c, extractor, handler)

	assert.True(t, handler.executed)
	assert.False(t, handler.planErrorSeen)
}

func TestOrchestrator_PartialPlan_PassedToHandlePlanError(t *testing.T) {
	// When Plan returns an error, the partially populated plan (e.g. with
	// ModelTarget set) must be passed to HandlePlanError so the handler
	// can attribute metrics.
	partialPlan := &types.RequestPlan{
		ModelTarget: &types.ModelTarget{ModelName: "resolved-model"},
		ErrorCode:   types.PlanErrInsufficientBalance,
	}
	orch := NewOrchestrator(&stubPlanner{plan: partialPlan, err: errors.New("insufficient balance")})
	extractor := &stubExtractor{meta: &types.RequestMetadata{Model: "test"}}
	handler := &stubHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	orch.Dispatch(c, extractor, handler)

	require.True(t, handler.planErrorSeen)
	assert.Equal(t, partialPlan, handler.planErrorPlan)
	assert.NotNil(t, handler.planErrorPlan.ModelTarget)
}
