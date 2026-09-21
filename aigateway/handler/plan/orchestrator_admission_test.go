package plan

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

type stubAdmissionReleaser struct {
	released int
	lastPlan *types.RequestPlan
}

func (r *stubAdmissionReleaser) ReleaseAdmission(c *gin.Context, p *types.RequestPlan) {
	r.released++
	r.lastPlan = p
}

func admittedPlan() *types.RequestPlan {
	return &types.RequestPlan{
		Admission: &types.AdmissionDecision{
			Action: types.AdmissionAdmit,
			Lease:  &types.AdmissionLease{ModelID: "m", UpstreamID: 1, Token: "t"},
		},
	}
}

func TestOrchestrator_ReleasesAdmissionLeaseAfterExecute(t *testing.T) {
	releaser := &stubAdmissionReleaser{}
	orch := NewOrchestrator(&stubPlanner{plan: admittedPlan()}, nil, releaser)

	c, _ := gin.CreateTestContext(nil)
	orch.Dispatch(c, &stubExtractor{meta: &types.RequestMetadata{}}, &stubHandler{})

	require.Equal(t, 1, releaser.released)
	require.NotNil(t, releaser.lastPlan)
	require.NotNil(t, releaser.lastPlan.Admission.Lease)
}

func TestOrchestrator_NoReleaseWithoutLease(t *testing.T) {
	releaser := &stubAdmissionReleaser{}
	orch := NewOrchestrator(&stubPlanner{plan: &types.RequestPlan{}}, nil, releaser)

	c, _ := gin.CreateTestContext(nil)
	orch.Dispatch(c, &stubExtractor{meta: &types.RequestMetadata{}}, &stubHandler{})

	// The safety net is registered unconditionally and reads the lease at
	// defer execution time; for a plan without a lease it observes nothing
	// to release (the real releaser is a no-op on a nil lease).
	require.Equal(t, 1, releaser.released)
	require.NotNil(t, releaser.lastPlan)
	assert.Nil(t, releaser.lastPlan.Admission)
}

func TestOrchestrator_ReleasesLeaseAcquiredDuringFallback(t *testing.T) {
	// A plan that left the Plan phase WITHOUT a lease (fail-open or
	// policy-less preferred upstream) may still acquire one during the
	// attempt loop (availability fallback). The safety net must release it:
	// otherwise the renewer would keep the lease alive forever and the
	// concurrency slot would leak.
	releaser := &stubAdmissionReleaser{}
	handler := &stubHandler{}
	orch := NewOrchestrator(&stubPlanner{plan: &types.RequestPlan{}}, nil, releaser)

	handler.executePlan = &types.RequestPlan{} // mutated in place by Execute below
	handler.executeOverride = func(p *types.RequestPlan) {
		p.Admission = &types.AdmissionDecision{
			Action: types.AdmissionAdmit,
			Lease:  &types.AdmissionLease{ModelID: "m", UpstreamID: 2, Token: "fallback"},
		}
	}

	c, _ := gin.CreateTestContext(nil)
	orch.Dispatch(c, &stubExtractor{meta: &types.RequestMetadata{}}, handler)

	require.Equal(t, 1, releaser.released)
	require.NotNil(t, releaser.lastPlan)
	require.NotNil(t, releaser.lastPlan.Admission)
	require.Equal(t, "fallback", releaser.lastPlan.Admission.Lease.Token)
}

func TestOrchestrator_ReleasesAdmissionLeaseOnExecutePanic(t *testing.T) {
	releaser := &stubAdmissionReleaser{}
	handler := &stubHandler{panicInExecute: true}
	orch := NewOrchestrator(&stubPlanner{plan: admittedPlan()}, nil, releaser)

	c, _ := gin.CreateTestContext(nil)
	require.Panics(t, func() {
		orch.Dispatch(c, &stubExtractor{meta: &types.RequestMetadata{}}, handler)
	})

	// The deferred safety net runs during the panic unwind: the lease must
	// be released even though Execute never returned normally.
	require.Equal(t, 1, releaser.released)
	require.NotNil(t, releaser.lastPlan)
	require.NotNil(t, releaser.lastPlan.Admission.Lease)
}

func TestOrchestrator_ReleasesLeaseOnPlanRejection(t *testing.T) {
	releaser := &stubAdmissionReleaser{}
	denied := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action: types.AdmissionReject,
		Reason: types.AdmissionReasonRPMExceeded,
	}}
	plan := &types.RequestPlan{ErrorCode: types.PlanErrCapacityExceeded}
	orch := NewOrchestrator(&stubPlanner{plan: plan, err: denied}, nil, releaser)

	c, _ := gin.CreateTestContext(nil)
	orch.Dispatch(c, &stubExtractor{meta: &types.RequestMetadata{}}, &stubHandler{})

	// The safety net always runs and reads the lease at execution time; for
	// a rejected decision (no lease) it observes nothing to release.
	assert.Equal(t, 1, releaser.released)
	assert.Nil(t, releaser.lastPlan.Admission)

	// When the planner acquired a lease before rejecting (re-selection
	// routing failure), the safety net must still release it.
	orch = NewOrchestrator(&stubPlanner{plan: admittedPlan(), err: errors.New("routing failed")}, nil, releaser)
	orch.Dispatch(c, &stubExtractor{meta: &types.RequestMetadata{}}, &stubHandler{})
	assert.Equal(t, 2, releaser.released)
	require.NotNil(t, releaser.lastPlan.Admission.Lease)
}
