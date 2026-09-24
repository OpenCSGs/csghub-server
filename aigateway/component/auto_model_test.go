package component

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/semanticrouter"
	"opencsg.com/csghub-server/common/config"
)

// stubRankClient stands in for the ranking service so the selection rule
// can be tested without a network.
type stubRankClient struct {
	ranking     []semanticrouter.Candidate
	rankErr     error
	healthErr   error
	healthCalls int32
	// indexVersion is what the readiness probe reports.
	indexVersion string
	// rankIndexVersion and rankPolicyVersion are what the ranking response
	// itself carries.  They are empty by default because the ordinary
	// response does not include them.
	rankIndexVersion  string
	rankPolicyVersion string
	rankElapsedMS     float64
	lastRequest       *semanticrouter.RankRequest
	// healthBlock, when non-nil, holds the readiness probe until the test
	// releases it, standing in for a slow or hung ranking service.
	healthBlock chan struct{}
}

func (s *stubRankClient) Rank(_ context.Context, req *semanticrouter.RankRequest) (*semanticrouter.RankResponse, error) {
	s.lastRequest = req
	if s.rankErr != nil {
		return nil, s.rankErr
	}
	return &semanticrouter.RankResponse{
		Ranking:       s.ranking,
		IndexVersion:  s.rankIndexVersion,
		PolicyVersion: s.rankPolicyVersion,
		ElapsedMS:     s.rankElapsedMS,
	}, nil
}

func (s *stubRankClient) Ready(context.Context) (*semanticrouter.Readiness, error) {
	atomic.AddInt32(&s.healthCalls, 1)
	if s.healthBlock != nil {
		<-s.healthBlock
	}
	if s.healthErr != nil {
		return nil, s.healthErr
	}
	version := s.indexVersion
	if version == "" {
		version = "live-abc"
	}
	return &semanticrouter.Readiness{Ready: true, IndexVersion: version}, nil
}

// probeNow runs a probe synchronously at the router's current generation,
// which is what Available would have scheduled in the background.
func probeNow(r *autoModelRouter, ctx context.Context) {
	r.mu.Lock()
	generation := r.generation
	r.mu.Unlock()
	r.probe(ctx, generation)
}

func newTestAutoRouter(client semanticrouter.Client) *autoModelRouter {
	return &autoModelRouter{client: client, modelID: "auto", healthTTL: time.Minute}
}

func userTurn(text string) types.AutoRouteInput {
	return types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "user", Content: text}}}
}

func TestNewAutoModelRouter_DisabledWithoutServerURL(t *testing.T) {
	router, err := NewAutoModelRouter(&config.Config{})
	require.NoError(t, err)
	require.Nil(t, router)
}

func TestNewAutoModelRouter_DefaultsAndOverrides(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.SemanticRouter.ServerURL = "https://router.test:1032"
	router, err := NewAutoModelRouter(cfg)
	require.NoError(t, err)
	require.NotNil(t, router)
	require.Equal(t, defaultAutoModelID, router.ModelID())

	cfg.AIGateway.SemanticRouter.ModelID = "smart"
	router, err = NewAutoModelRouter(cfg)
	require.NoError(t, err)
	require.Equal(t, "smart", router.ModelID())
}

func TestNewAutoModelRouter_RejectsInvalidURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.SemanticRouter.ServerURL = "router.test:1032"
	_, err := NewAutoModelRouter(cfg)
	require.ErrorContains(t, err, "semantic router client")
}

func TestAutoModelRouter_SelectPicksHighestRankedAvailableCandidate(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, Model: "deepseek-v4-flash-0731", ProviderModel: "deepseek-v4-flash", Score: 0.91},
		{Rank: 2, Model: "glm-5.1", ProviderModel: "GLM-5.1", Score: 0.82},
	}}
	router := newTestAutoRouter(client)

	// The top candidate is not in the catalogue, so the second one wins.
	// Matching is case-insensitive in both directions.
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}}

	probeNow(router, context.Background())
	decision, err := router.Select(context.Background(), userTurn("hello"), candidates)
	require.NoError(t, err)
	require.Equal(t, "glm-5.1", decision.ModelID)
	require.Equal(t, "glm-5.1", decision.BenchmarkID)
	require.Equal(t, 2, decision.Rank)
	require.InDelta(t, 0.82, decision.Score, 1e-9)
	require.Equal(t, "live-abc", decision.IndexVersion, "taken from the readiness probe, which is where it is reported")
}

func TestAutoModelRouter_SelectReturnsCatalogueCasing(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, Model: "minimax-2.5", ProviderModel: "minimax-m2.5"},
	}}
	router := newTestAutoRouter(client)
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "MiniMax-M2.5"}}}

	decision, err := router.Select(context.Background(), userTurn("hello"), candidates)
	require.NoError(t, err)
	require.Equal(t, "MiniMax-M2.5", decision.ModelID, "the catalogue's own ID must be returned, not the ranking service's spelling")
}

// A deployment that reports only its ordered benchmark identifiers leaves
// the upstream model name empty, so the benchmark identifier is what has
// to be matched against the catalogue.
func TestAutoModelRouter_SelectFallsBackToBenchmarkID(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, Model: "deepseek-v4-flash-0731"},
		{Rank: 2, Model: "glm-5.1"},
	}}
	router := newTestAutoRouter(client)
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}}

	decision, err := router.Select(context.Background(), userTurn("hello"), candidates)
	require.NoError(t, err)
	require.Equal(t, "glm-5.1", decision.ModelID)
	require.Equal(t, 2, decision.Rank)
}

// An unnamed candidate must not match a catalogue entry just because both
// sides are blank.
func TestAutoModelRouter_SelectIgnoresBlankNames(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{{Rank: 1}}}
	router := newTestAutoRouter(client)
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "  "}}}

	_, err := router.Select(context.Background(), userTurn("hello"), candidates)
	require.ErrorContains(t, err, "none of the 1 ranked candidates")
}

func TestAutoModelRouter_SelectErrors(t *testing.T) {
	t.Run("no user message", func(t *testing.T) {
		router := newTestAutoRouter(&stubRankClient{})
		_, err := router.Select(context.Background(), types.AutoRouteInput{
			Messages: []types.AutoRouteMessage{{Role: "system", Content: "be helpful"}},
		}, []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
		require.ErrorContains(t, err, "at least one user message")
	})

	t.Run("no candidates", func(t *testing.T) {
		router := newTestAutoRouter(&stubRankClient{})
		_, err := router.Select(context.Background(), userTurn("hello"), nil)
		require.ErrorContains(t, err, "no model to choose from")
	})

	t.Run("ranking service failure is propagated", func(t *testing.T) {
		router := newTestAutoRouter(&stubRankClient{rankErr: errors.New("boom")})
		_, err := router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
		require.ErrorContains(t, err, "boom")
	})

	t.Run("nothing matches", func(t *testing.T) {
		client := &stubRankClient{ranking: []semanticrouter.Candidate{
			{Rank: 1, ProviderModel: "some-model-we-do-not-host"},
		}}
		router := newTestAutoRouter(client)
		_, err := router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
		require.ErrorContains(t, err, "none of the 1 ranked candidates")
	})
}

// The distinction matters to callers: a downed service is retryable, a
// catalogue mismatch is not.
// A candidate whose name does not match any gateway model is skipped and
// the next usable one wins, so a drift in names silently costs the
// better-ranked model.  The warning is what makes that visible.
func TestAutoModelRouter_WarnsAboutCandidatesThatNameNoModel(t *testing.T) {
	logs := captureLogs(t)
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, Model: "deepseek-v4-flash-0731"},
		{Rank: 2, Model: "minimax-2.5"},
		{Rank: 3, Model: "glm-5.1"},
	}}
	router := newTestAutoRouter(client)
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}}

	decision, err := router.Select(context.Background(), userTurn("hello"), candidates)
	require.NoError(t, err)
	require.Equal(t, "glm-5.1", decision.ModelID)

	out := logs.String()
	require.Contains(t, out, "name no gateway model")
	require.Contains(t, out, "deepseek-v4-flash-0731,minimax-2.5")
	require.Contains(t, out, "skipped_count=2")

	// The same gap is not re-logged on every request.
	logs.Reset()
	_, err = router.Select(context.Background(), userTurn("hello"), candidates)
	require.NoError(t, err)
	require.NotContains(t, logs.String(), "name no gateway model")
}

// A near-miss must not be resolved by guessing: the listing holds IDs that
// a prefix or suffix rule would confuse.
func TestAutoModelRouter_DoesNotGuessAtNearMisses(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, Model: "deepseek-v4-flash-0731"},
	}}
	router := newTestAutoRouter(client)
	candidates := []types.Model{
		{BaseModel: types.BaseModel{ID: "deepseek-v4-flash"}},
		{BaseModel: types.BaseModel{ID: "deepseek-v4-flash-message"}},
	}

	_, err := router.Select(context.Background(), userTurn("hello"), candidates)
	require.ErrorContains(t, err, "none of the 1 ranked candidates")
}

// The ordinary ranking response carries no index version, so the one the
// readiness probe reported is what makes a decision auditable.
func TestAutoModelRouter_SelectReportsTheProbedIndexVersion(t *testing.T) {
	client := &stubRankClient{
		indexVersion: "live-22ba8309",
		ranking:      []semanticrouter.Candidate{{Rank: 1, Model: "glm-5.1"}},
	}
	router := newTestAutoRouter(client)
	probeNow(router, context.Background())

	decision, err := router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
	require.NoError(t, err)
	require.Equal(t, "live-22ba8309", decision.IndexVersion)
}

func TestAutoModelRouter_SelectClassifiesFailures(t *testing.T) {
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}}

	cases := []struct {
		name     string
		client   *stubRankClient
		input    types.AutoRouteInput
		wantCode string
	}{
		{
			name:     "the ranking service is down",
			client:   &stubRankClient{rankErr: errors.New("connection refused")},
			input:    userTurn("hello"),
			wantCode: autoRouteCodeUnavailable,
		},
		{
			name:     "the ranking service answered but nothing matches",
			client:   &stubRankClient{ranking: []semanticrouter.Candidate{{Rank: 1, ProviderModel: "not-hosted-here"}}},
			input:    userTurn("hello"),
			wantCode: autoRouteCodeNotFound,
		},
		{
			name:     "the request carries no user message",
			client:   &stubRankClient{},
			input:    types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "system", Content: "x"}}},
			wantCode: autoRouteCodeNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestAutoRouter(tc.client).Select(context.Background(), tc.input, candidates)
			require.Error(t, err)
			var coded *autoRouteError
			require.ErrorAs(t, err, &coded)
			require.Equal(t, tc.wantCode, coded.ModelErrorCode())
		})
	}
}

// When the service disappears mid-flight the virtual model has to stop
// being advertised at once, rather than staying listed until the cached
// health probe expires.
func TestAutoModelRouter_FailedCallWithdrawsTheModel(t *testing.T) {
	client := &stubRankClient{}
	router := newTestAutoRouter(client)
	ctx := context.Background()
	candidates := []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}}

	probeNow(router, ctx)
	require.True(t, router.Available(ctx))

	// The service goes down between the probe and the request.
	client.rankErr = errors.New("connection refused")
	client.healthErr = client.rankErr
	_, err := router.Select(ctx, userTurn("hello"), candidates)
	require.Error(t, err)

	router.mu.Lock()
	healthy := router.healthy
	router.mu.Unlock()
	require.False(t, healthy, "the virtual model is withdrawn without waiting for the health TTL")

	// The cached answer is expired rather than pinned, so recovery is
	// picked up on the next listing instead of after the full TTL.
	client.rankErr = nil
	client.healthErr = nil
	require.False(t, router.Available(ctx))
	require.Eventually(t, func() bool {
		return router.Available(ctx)
	}, 2*time.Second, time.Millisecond, "a recovered service is re-published on the next listing")
}

// A catalogue mismatch is not the service's fault, so the virtual model
// stays published.
func TestAutoModelRouter_NoMatchKeepsTheModelPublished(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{{Rank: 1, ProviderModel: "not-hosted-here"}}}
	router := newTestAutoRouter(client)
	ctx := context.Background()

	probeNow(router, ctx)
	require.True(t, router.Available(ctx))

	_, err := router.Select(ctx, userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
	require.Error(t, err)
	require.True(t, router.Available(ctx))
}

// A model the caller can see but which is unhealthy right now must not be
// reported as missing: the same request succeeds once it recovers.
func TestAutoModelRouter_TemporarilyUnavailableIsRetryable(t *testing.T) {
	unavailable := types.Model{
		BaseModel:    types.BaseModel{ID: "glm-5.1"},
		Availability: &types.ModelAvailability{IsAvailable: false},
	}
	client := &stubRankClient{ranking: []semanticrouter.Candidate{{Rank: 1, Model: "glm-5.1"}}}

	_, err := newTestAutoRouter(client).Select(context.Background(), userTurn("hello"), []types.Model{unavailable})
	require.Error(t, err)
	var coded *autoRouteError
	require.ErrorAs(t, err, &coded)
	require.Equal(t, autoRouteCodeUnavailable, coded.ModelErrorCode(),
		"a model that is merely out of service is a temporary condition")
	require.ErrorContains(t, err, "temporarily unavailable")
}

// A lower-ranked healthy model is preferred over failing the request.
func TestAutoModelRouter_SkipsUnhealthyAndTakesTheNextOne(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, Model: "glm-5.1"},
		{Rank: 2, Model: "qwen3.8-flash"},
	}}
	candidates := []types.Model{
		{BaseModel: types.BaseModel{ID: "glm-5.1"}, Availability: &types.ModelAvailability{IsAvailable: false}},
		{BaseModel: types.BaseModel{ID: "qwen3.8-flash"}, Availability: &types.ModelAvailability{IsAvailable: true}},
	}

	decision, err := newTestAutoRouter(client).Select(context.Background(), userTurn("hello"), candidates)
	require.NoError(t, err)
	require.Equal(t, "qwen3.8-flash", decision.ModelID)
	require.Equal(t, 2, decision.Rank)
}

// A ranking that names nothing the caller has stays a not-found, because
// retrying it cannot help.
func TestAutoModelRouter_UnknownNamesStayNotFound(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{{Rank: 1, Model: "not-hosted-here"}}}
	candidates := []types.Model{
		{BaseModel: types.BaseModel{ID: "glm-5.1"}, Availability: &types.ModelAvailability{IsAvailable: false}},
	}

	_, err := newTestAutoRouter(client).Select(context.Background(), userTurn("hello"), candidates)
	require.Error(t, err)
	var coded *autoRouteError
	require.ErrorAs(t, err, &coded)
	require.Equal(t, autoRouteCodeNotFound, coded.ModelErrorCode())
}

// A probe that was overtaken by a failed call must not resurrect the
// model behind that newer, more definitive result.
func TestAutoModelRouter_StaleProbeDoesNotOverwriteAFailure(t *testing.T) {
	client := &stubRankClient{healthBlock: make(chan struct{})}
	router := newTestAutoRouter(client)
	ctx := context.Background()

	// A probe starts and blocks inside the readiness call.
	require.False(t, router.Available(ctx))
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&client.healthCalls) == 1
	}, 2*time.Second, time.Millisecond)

	// While it is in flight a real call fails and withdraws the model.
	router.markUnavailable(ctx)

	// The older probe now returns successfully.
	close(client.healthBlock)
	require.Eventually(t, func() bool {
		router.mu.Lock()
		defer router.mu.Unlock()
		return !router.probing
	}, 2*time.Second, time.Millisecond)

	router.mu.Lock()
	healthy := router.healthy
	router.mu.Unlock()
	require.False(t, healthy, "the stale probe must not overwrite the newer failure")
}

func TestAutoModelRouter_SelectForwardsToolsAndTrimsHistory(t *testing.T) {
	client := &stubRankClient{ranking: []semanticrouter.Candidate{{Rank: 1, ProviderModel: "glm-5.1"}}}
	router := newTestAutoRouter(client)

	input := types.AutoRouteInput{Tools: []json.RawMessage{json.RawMessage(`{"type":"function"}`)}}
	for i := 0; i < maxAutoRouteMessages+5; i++ {
		input.Messages = append(input.Messages, types.AutoRouteMessage{Role: "user", Content: "turn"})
	}
	// An entirely empty turn contributes nothing.
	input.Messages = append(input.Messages, types.AutoRouteMessage{})

	_, err := router.Select(context.Background(), input, []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
	require.NoError(t, err)
	require.Len(t, client.lastRequest.Messages, maxAutoRouteMessages)
	require.Len(t, client.lastRequest.Tools, 1)
}

func TestAutoModelRouter_AvailableCachesProbe(t *testing.T) {
	client := &stubRankClient{}
	router := newTestAutoRouter(client)

	// The first call has nothing cached yet and refreshes in the
	// background rather than waiting for the answer.
	require.False(t, router.Available(context.Background()))
	waitForProbes(t, router, client, 1)
	require.True(t, router.Available(context.Background()))

	require.True(t, router.Available(context.Background()))
	require.Equal(t, int32(1), atomic.LoadInt32(&client.healthCalls), "the probe result is reused within the TTL")

	// Once the cached result expires the service is probed again, and a
	// failing probe turns availability off.
	router.healthTTL = time.Nanosecond
	client.healthErr = errors.New("down")
	require.True(t, router.Available(context.Background()), "the stale answer is served while the refresh runs")
	waitForProbes(t, router, client, 2)
	require.False(t, router.Available(context.Background()))
}

// A hung ranking service must not delay a caller that is only listing
// models, nor pile up one in-flight probe per caller.
func TestAutoModelRouter_AvailableNeverBlocksOnAHungService(t *testing.T) {
	client := &stubRankClient{healthBlock: make(chan struct{})}
	router := newTestAutoRouter(client)

	done := make(chan bool, 1)
	go func() { done <- router.Available(context.Background()) }()
	select {
	case available := <-done:
		require.False(t, available)
	case <-time.After(2 * time.Second):
		t.Fatal("Available blocked on the hung ranking service")
	}

	// Wait until the probe has actually reached the service before
	// asserting that no second one is started behind it.
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&client.healthCalls) == 1
	}, 2*time.Second, time.Millisecond)

	// Further calls return immediately too, and do not start more probes.
	for i := 0; i < 5; i++ {
		require.False(t, router.Available(context.Background()))
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&client.healthCalls), "a probe in flight is not duplicated")

	close(client.healthBlock)
	waitForProbes(t, router, client, 1)
}

// waitForProbes waits until the background probe count has reached want
// and the last probe has published its result.
func waitForProbes(t *testing.T, router *autoModelRouter, client *stubRankClient, want int32) {
	t.Helper()
	require.Eventually(t, func() bool {
		if atomic.LoadInt32(&client.healthCalls) < want {
			return false
		}
		router.mu.Lock()
		defer router.mu.Unlock()
		return !router.probing
	}, 2*time.Second, time.Millisecond, "the background probe did not settle")
}

func TestAutoModelRouter_AvailableAgainstRealServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ready":true,"index_version":"live-abc","training_requests":4214}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.AIGateway.SemanticRouter.ServerURL = server.URL
	router, err := NewAutoModelRouter(cfg)
	require.NoError(t, err)

	// The first call kicks off the probe; availability follows once it
	// lands.
	require.False(t, router.Available(context.Background()))
	require.Eventually(t, func() bool {
		return router.Available(context.Background())
	}, 2*time.Second, time.Millisecond)
}

// captureLogs redirects the default logger for the duration of a test so
// the operator-facing lines can be asserted on.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func TestAutoModelRouter_LogsTheSelection(t *testing.T) {
	logs := captureLogs(t)
	client := &stubRankClient{
		indexVersion: "live-abc",
		ranking:      []semanticrouter.Candidate{{Rank: 1, Model: "glm-5.1", Score: 0.82}},
	}
	router := newTestAutoRouter(client)
	probeNow(router, context.Background())
	logs.Reset()

	_, err := router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
	require.NoError(t, err)

	out := logs.String()
	assert.Contains(t, out, "semantic router selected a model")
	assert.Contains(t, out, "model=glm-5.1")
	assert.Contains(t, out, "rank=1")
	assert.Contains(t, out, "elapsed_ms=", "the round trip has to be measurable from the logs")
	assert.Contains(t, out, "index_version=live-abc")
	assert.Contains(t, out, "candidate_count=1")
	assert.NotContains(t, out, "service_elapsed_ms=",
		"the ordinary ranking response does not report the service's own timing")

	// A deployment that does report its own timing has it logged too.
	logs.Reset()
	client.rankElapsedMS = 12.5
	_, err = router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
	require.NoError(t, err)
	assert.Contains(t, logs.String(), "service_elapsed_ms=12.5")
}

func TestAutoModelRouter_LogsARankFailure(t *testing.T) {
	logs := captureLogs(t)
	router := newTestAutoRouter(&stubRankClient{rankErr: errors.New("connection refused")})

	_, err := router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "glm-5.1"}}})
	require.Error(t, err)

	out := logs.String()
	assert.Contains(t, out, "semantic router rank call failed")
	assert.Contains(t, out, "connection refused")
	assert.Contains(t, out, "elapsed_ms=")
	assert.Contains(t, out, "message_count=1")
}

func TestAutoModelRouter_LogsWhatItCouldNotMatch(t *testing.T) {
	logs := captureLogs(t)
	client := &stubRankClient{ranking: []semanticrouter.Candidate{
		{Rank: 1, ProviderModel: "deepseek-v4-flash"},
		{Rank: 2, Model: "glm-9.9-unreleased"},
	}}
	router := newTestAutoRouter(client)

	_, err := router.Select(context.Background(), userTurn("hello"), []types.Model{{BaseModel: types.BaseModel{ID: "some-other-model"}}})
	require.Error(t, err)

	out := logs.String()
	assert.Contains(t, out, "no candidate this caller can use")
	// Both sides of the mismatch have to be in the line for it to be
	// diagnosable at all.
	assert.Contains(t, out, "deepseek-v4-flash")
	assert.Contains(t, out, "glm-9.9-unreleased")
	assert.Contains(t, out, "candidate_count=1")
}

func TestAutoModelRouter_LogsAvailabilityTransitionsOnly(t *testing.T) {
	logs := captureLogs(t)
	client := &stubRankClient{}
	router := newTestAutoRouter(client)
	ctx := context.Background()

	// The probe is driven directly so the assertions do not race with the
	// background refresh Available would start.
	probeNow(router, ctx)
	assert.Contains(t, logs.String(), "semantic router is reachable")
	assert.True(t, router.Available(ctx))

	// A steady service stays quiet across further probes.
	logs.Reset()
	probeNow(router, ctx)
	assert.Empty(t, logs.String())

	client.healthErr = errors.New("connection refused")
	probeNow(router, ctx)
	out := logs.String()
	assert.Contains(t, out, "withdrawing the automatic routing model")
	assert.Contains(t, out, "connection refused")
	assert.Contains(t, out, "probe_ms=")
	assert.False(t, router.Available(ctx))

	// And the outage is not re-logged on every probe either.
	logs.Reset()
	probeNow(router, ctx)
	assert.Empty(t, logs.String())

	// Recovery is logged.
	client.healthErr = nil
	probeNow(router, ctx)
	assert.Contains(t, logs.String(), "semantic router is reachable")
}

func TestAutoModelEntry(t *testing.T) {
	entry := autoModelEntry("auto")
	require.Equal(t, "auto", entry.ID)
	require.Equal(t, "model", entry.Object)
	require.True(t, entry.AutoRoute)
	require.True(t, entry.SupportFunctionCall)
	require.Equal(t, "text-generation", entry.Task)
	require.NotNil(t, entry.Metadata)
	require.NotNil(t, entry.Availability)
	require.True(t, entry.Availability.IsAvailable)
	require.Empty(t, entry.Upstreams, "the virtual model must never carry an upstream of its own")
}
