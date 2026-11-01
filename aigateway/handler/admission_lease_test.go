package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func admissionAttemptTarget(upstreamID int64) *resolvedModelTarget {
	return &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Upstreams: []commontypes.UpstreamConfig{{ID: upstreamID, URL: "https://u.example.com", Enabled: true}},
		},
		Upstream:  commontypes.UpstreamConfig{ID: upstreamID, URL: "https://u.example.com", Enabled: true},
		Target:    "https://u.example.com",
		ModelName: "test-model",
	}
}

func TestEnsureAdmissionForAttempt_ReusesSameUpstreamLease(t *testing.T) {
	tester, _, _ := setupTest(t)
	// No mock expectations: reusing the lease must not touch the component.
	p := &types.RequestPlan{Admission: &types.AdmissionDecision{
		Action: types.AdmissionAdmit,
		Lease:  &types.AdmissionLease{ModelID: "test-model", UpstreamID: 1, Token: "t1", ReservedTokens: 1064},
	}}

	err := ensureAdmissionForAttempt(context.Background(), tester.handler, p, admissionAttemptTarget(1))
	require.NoError(t, err)
	require.NotNil(t, p.Admission.Lease)
	require.Equal(t, "t1", p.Admission.Lease.Token)
}

func TestEnsureAdmissionForAttempt_RotatesLeaseOnFallback(t *testing.T) {
	tester, _, _ := setupTest(t)
	mockOpenAI := tester.mocks.openAIComp

	newLease := &types.AdmissionLease{ModelID: "test-model", UpstreamID: 2, Token: "t2", ReservedTokens: 1064}
	mockOpenAI.EXPECT().
		AcquireCapacityAdmission(mock.Anything, mock.Anything, int64(2), int64(1064)).
		Return(&types.AdmissionDecision{
			Action:             types.AdmissionAdmit,
			Lease:              newLease,
			SelectedUpstreamID: 2,
		}).
		Once()
	mockOpenAI.EXPECT().
		FinalizeCapacityAdmission(mock.Anything, mock.Anything, mock.Anything).
		Once()

	p := &types.RequestPlan{Admission: &types.AdmissionDecision{
		Action: types.AdmissionAdmit,
		Lease:  &types.AdmissionLease{ModelID: "test-model", UpstreamID: 1, Token: "t1", ReservedTokens: 1064},
	}}

	err := ensureAdmissionForAttempt(context.Background(), tester.handler, p, admissionAttemptTarget(2))
	require.NoError(t, err)
	require.Same(t, newLease, p.Admission.Lease)
}

func TestEnsureAdmissionForAttempt_RejectAbortsAndReleasesPrevious(t *testing.T) {
	tester, _, _ := setupTest(t)
	mockOpenAI := tester.mocks.openAIComp

	rejected := &types.AdmissionDecision{
		Action:            types.AdmissionReject,
		Reason:            types.AdmissionReasonConcurrencyExceeded,
		RetryAfterSeconds: 5,
	}
	mockOpenAI.EXPECT().
		AcquireCapacityAdmission(mock.Anything, mock.Anything, int64(2), int64(1064)).
		Return(rejected).
		Once()
	mockOpenAI.EXPECT().
		FinalizeCapacityAdmission(mock.Anything, mock.Anything, mock.Anything).
		Once()

	p := &types.RequestPlan{Admission: &types.AdmissionDecision{
		Action: types.AdmissionAdmit,
		Lease:  &types.AdmissionLease{ModelID: "test-model", UpstreamID: 1, Token: "t1", ReservedTokens: 1064},
	}}

	err := ensureAdmissionForAttempt(context.Background(), tester.handler, p, admissionAttemptTarget(2))
	require.Error(t, err)
	require.True(t, types.IsAdmissionDenied(err))
	require.Nil(t, p.Admission.Lease, "the previous lease is released and cleared on rejection")
}

func TestEnsureAdmissionForAttempt_UnprotectedFallbackProceeds(t *testing.T) {
	tester, _, _ := setupTest(t)
	mockOpenAI := tester.mocks.openAIComp

	mockOpenAI.EXPECT().
		AcquireCapacityAdmission(mock.Anything, mock.Anything, int64(2), int64(1064)).
		Return(nil).
		Once()
	mockOpenAI.EXPECT().
		FinalizeCapacityAdmission(mock.Anything, mock.Anything, mock.Anything).
		Once()

	p := &types.RequestPlan{Admission: &types.AdmissionDecision{
		Action: types.AdmissionAdmit,
		Lease:  &types.AdmissionLease{ModelID: "test-model", UpstreamID: 1, Token: "t1", ReservedTokens: 1064},
	}}

	err := ensureAdmissionForAttempt(context.Background(), tester.handler, p, admissionAttemptTarget(2))
	require.NoError(t, err)
	require.Nil(t, p.Admission.Lease)
}

func TestEnsureAdmissionForAttempt_NilPlanIsNoop(t *testing.T) {
	tester, _, _ := setupTest(t)
	err := ensureAdmissionForAttempt(context.Background(), tester.handler, nil, admissionAttemptTarget(1))
	require.NoError(t, err)
}

func TestHandleAdmissionDenied_Renders429WithRetryAfter(t *testing.T) {
	tester, c, w := setupTest(t)

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action:            types.AdmissionReject,
		Reason:            types.AdmissionReasonTPMExceeded,
		RetryAfterSeconds: 30,
	}}
	handled := tester.handler.handleAdmissionDenied(c, false, err)
	require.True(t, handled)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "30", w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "capacity_exceeded")
	require.Contains(t, w.Body.String(), types.AdmissionReasonTPMExceeded)
}

func TestHandleAdmissionDenied_IgnoresOtherErrors(t *testing.T) {
	tester, c, _ := setupTest(t)
	handled := tester.handler.handleAdmissionDenied(c, false, context.DeadlineExceeded)
	require.False(t, handled)
}

func TestHandleAdmissionDenied_QueueTimeout_Renders408(t *testing.T) {
	tester, c, w := setupTest(t)

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action: types.AdmissionReject,
		Reason: types.AdmissionReasonQueueTimeout,
	}}
	handled := tester.handler.handleAdmissionDenied(c, false, err)
	require.True(t, handled)
	require.Equal(t, http.StatusRequestTimeout, w.Code)
	// 408 carries no Retry-After: the retry decision is the client's.
	require.Empty(t, w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "queue_timeout")
	require.Contains(t, w.Body.String(), "timeout_error")
}

// A stream request does not imply the stream has started: a plan-phase
// admission denial reaches the renderer before any byte is written, and the
// head must still carry the real status instead of the implicit 200 a bare
// Write would commit. Chat pre-sets the SSE content type on the uncommitted
// head, so the JSON rendering also has to clear it.
func TestHandleAdmissionDenied_StreamBeforeHeadCommit_Renders429(t *testing.T) {
	tester, c, w := setupTest(t)
	c.Writer.Header().Set("Content-Type", "text/event-stream")

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action:            types.AdmissionReject,
		Reason:            types.AdmissionReasonTPMExceeded,
		RetryAfterSeconds: 30,
	}}
	handled := tester.handler.handleAdmissionDenied(c, true, err)
	require.True(t, handled)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "30", w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "capacity_exceeded")
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestHandleAdmissionDenied_StreamBeforeHeadCommit_Renders408(t *testing.T) {
	tester, c, w := setupTest(t)
	c.Writer.Header().Set("Content-Type", "text/event-stream")

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action: types.AdmissionReject,
		Reason: types.AdmissionReasonQueueTimeout,
	}}
	handled := tester.handler.handleAdmissionDenied(c, true, err)
	require.True(t, handled)
	require.Equal(t, http.StatusRequestTimeout, w.Code)
	require.Empty(t, w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "timeout_error")
}

// Once the head is committed (denial after the stream started) the status
// can no longer change, so the same envelope goes into the event stream.
func TestHandleAdmissionDenied_StreamAfterHeadCommit_WritesErrorEvent(t *testing.T) {
	tester, c, w := setupTest(t)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte("data: {}\n\n"))

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action: types.AdmissionReject,
		Reason: types.AdmissionReasonTPMExceeded,
	}}
	handled := tester.handler.handleAdmissionDenied(c, true, err)
	require.True(t, handled)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "data: ")
	require.Contains(t, w.Body.String(), "capacity_exceeded")
	require.Contains(t, w.Body.String(), "data: [DONE]")
}

func TestHandleAdmissionDenied_QueueCancelled_Renders408(t *testing.T) {
	tester, c, w := setupTest(t)

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action: types.AdmissionReject,
		Reason: types.AdmissionReasonQueueCancelled,
	}}
	handled := tester.handler.handleAdmissionDenied(c, false, err)
	require.True(t, handled)
	require.Equal(t, http.StatusRequestTimeout, w.Code)
	require.Contains(t, w.Body.String(), "queue_cancelled")
}

func TestHandleAdmissionDenied_QueueFull_Renders429QueueFull(t *testing.T) {
	tester, c, w := setupTest(t)

	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{
		Action:            types.AdmissionReject,
		Reason:            types.AdmissionReasonQueueFull,
		RetryAfterSeconds: 5,
	}}
	handled := tester.handler.handleAdmissionDenied(c, false, err)
	require.True(t, handled)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "5", w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "queue_full")
}
