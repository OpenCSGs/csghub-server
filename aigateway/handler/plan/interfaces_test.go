package plan

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// TestInterfaceCompileAssertions verifies that the plan package interfaces
// are satisfied by minimal stubs.  This catches accidental signature drift
// early — if an interface method changes, the stubs here fail to compile.
func TestInterfaceCompileAssertions(t *testing.T) {
	var _ MetadataExtractor = (*stubExtractor2)(nil)
	var _ Planner = (*stubPlanner2)(nil)
	var _ ProtocolHandler = (*stubHandler2)(nil)
	var _ OrchestratorInterface = (*Orchestrator)(nil)
}

type stubExtractor2 struct{}

func (s *stubExtractor2) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	return nil, nil
}

type stubPlanner2 struct{}

func (s *stubPlanner2) Plan(ctx context.Context, meta *types.RequestMetadata) (*types.RequestPlan, error) {
	return nil, nil
}

type stubHandler2 struct{}

func (s *stubHandler2) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	return nil
}

func (s *stubHandler2) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
}
