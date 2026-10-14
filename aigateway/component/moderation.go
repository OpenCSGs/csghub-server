package component

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	// max content length for moderation
	defaultMaxContentLength = 2000 // sliding window size
	slidingWindowSize       = 2000
	// cache ttl
	cacheTTL = 24 * time.Hour
	// moderation cache prefix
	moderationCachePromptPrefix = "moderation:prompt:"
	// default session cache size
	defaultSessionCacheSize = 10000
	// timeout for the background goroutine checking a prompt worth more than
	// one moderation call
	asyncPromptCheckTimeout = 30 * time.Second

	StreamCheckModeAsync       = "async"
	StreamCheckModeSync        = "sync"
	DefaultAsyncBufferMaxChars = 50
)

type Moderation interface {
	CheckChatPrompts(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, uuid string, isStream bool) (*rpc.CheckResult, error)
	CheckChatStreamResponse(ctx context.Context, chunk types.ChatCompletionChunk, uuid string) (*rpc.CheckResult, error)
	CheckChatNonStreamResponse(ctx context.Context, completion types.ChatCompletion) (*rpc.CheckResult, error)
	CheckText(ctx context.Context, req types.TextModerationRequest) (*rpc.CheckResult, error)
	CheckImagePrompts(ctx context.Context, prompt string, uuid string) (*rpc.CheckResult, error)
	CheckImage(ctx context.Context, completion types.ImageGenerationResponse) (*rpc.CheckResult, error)
	CloseStreamCheck(ctx context.Context, uuid string) (*rpc.CheckResult, error)
}

type sessionState struct {
	sync.Mutex
	buffer    strings.Builder
	sensitive bool
	reason    string
}

type StreamChecker interface {
	CheckStreamResponseText(ctx context.Context, content string, sessionID string) (*rpc.CheckResult, error)
	CloseStreamCheck(ctx context.Context, sessionID string) (*rpc.CheckResult, error)
}

type moderationImpl struct {
	modSvcClient     rpc.ModerationSvcClient
	cacheClient      cache.RedisClient
	config           *config.Config
	streamChecker    StreamChecker
	maxContentLength int
	// promptMaxChunks caps the estimated number of moderation calls per
	// prompt: above it, prompt moderation is skipped; within it but above
	// one call, the check runs asynchronously. Zero keeps the legacy
	// all-synchronous behavior.
	promptMaxChunks int
	// pendingPromptChecks tracks the in-flight asynchronous prompt moderation
	// of one request, keyed by its moderation session id, so the response
	// emitted for that request can gate on the verdict: stream chunk checks
	// peek at landed verdicts, and response finalization awaits an in-flight
	// check (bounded by pendingVerdictWait, independent of the caller's
	// check context so the wait never cannibalizes the body-check budget).
	// Entries are removed when the response-side check consumes them and
	// otherwise evicted by capacity.
	pendingPromptChecks *lru.Cache[string, *pendingPromptCheck]
	// pendingVerdictWait bounds how long response finalization waits for an
	// in-flight async prompt check. Defaults to asyncPromptCheckTimeout, the
	// background check's own budget, so the wait observes every completion
	// that happens within it.
	pendingVerdictWait time.Duration
}

// pendingPromptCheck tracks one in-flight asynchronous prompt moderation.
// done is closed once the check finished; result is only meaningful after
// that (nil when the check errored).
type pendingPromptCheck struct {
	done   chan struct{}
	result *rpc.CheckResult
}

type syncStreamChecker struct {
	modImpl *moderationImpl
}

func (s *syncStreamChecker) CheckStreamResponseText(ctx context.Context, content string, sessionID string) (*rpc.CheckResult, error) {
	if s.modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if strings.TrimSpace(content) == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}

	req := commontypes.LLMCheckRequest{
		Scenario:  commontypes.ScenarioLLMResModeration,
		Text:      content,
		SessionId: sessionID,
		Resumable: true,
		Stream:    true,
	}

	result, err := s.modImpl.modSvcClient.PassLLMRespCheck(ctx, req)
	s.modImpl.postCheck(ctx, result)
	return result, err
}

func (s *syncStreamChecker) CloseStreamCheck(ctx context.Context, sessionID string) (*rpc.CheckResult, error) {
	return &rpc.CheckResult{IsSensitive: false}, nil
}

type asyncStreamChecker struct {
	modImpl      *moderationImpl
	sessionCache *lru.Cache[string, *sessionState]
	maxChars     int
}

func (a *asyncStreamChecker) CheckStreamResponseText(ctx context.Context, content string, sessionID string) (*rpc.CheckResult, error) {
	if a.modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if strings.TrimSpace(content) == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}

	req := commontypes.LLMCheckRequest{
		Scenario:  commontypes.ScenarioLLMResModeration,
		Text:      content,
		SessionId: sessionID,
		Resumable: true,
		Stream:    true,
	}
	if a.sessionCache == nil {
		slog.Warn("moderation session cache is nil, fallback to sync mode")
		result, err := a.modImpl.modSvcClient.PassLLMRespCheck(ctx, req)
		a.modImpl.postCheck(ctx, result)
		return result, err
	}

	state, ok := a.sessionCache.Get(sessionID)
	if !ok {
		state = &sessionState{}
		a.sessionCache.Add(sessionID, state)
	}

	state.Lock()
	if state.sensitive {
		state.Unlock()
		return &rpc.CheckResult{IsSensitive: true, Reason: state.reason}, nil
	}

	state.buffer.WriteString(content)
	currentLen := state.buffer.Len()

	var textToCheck string
	if currentLen >= a.maxChars {
		textToCheck = state.buffer.String()
		state.buffer.Reset()
	}
	state.Unlock()

	if textToCheck != "" {
		go a.executeAsyncCheck(textToCheck, sessionID)
	}

	return &rpc.CheckResult{IsSensitive: false}, nil
}

func (a *asyncStreamChecker) executeAsyncCheck(text string, sessionId string) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := commontypes.LLMCheckRequest{
		Scenario:  commontypes.ScenarioLLMResModeration,
		Text:      text,
		SessionId: sessionId,
		Resumable: true,
		Stream:    true,
	}
	result, err := a.modImpl.modSvcClient.PassLLMRespCheck(bgCtx, req)
	if err != nil {
		slog.Warn("async moderation check failed", slog.Any("error", err))
		return
	}

	if result.IsSensitive {
		if a.modImpl.config != nil && a.modImpl.config.AIGateway.ModerationBypassSensitiveCheck {
			return
		}

		slog.ErrorContext(bgCtx, "sensitive content found asynchronously", slog.Any("reason", result.Reason))

		if s, ok := a.sessionCache.Get(sessionId); ok {
			s.Lock()
			s.sensitive = true
			s.reason = result.Reason
			s.Unlock()
		}
	}
}

func (a *asyncStreamChecker) CloseStreamCheck(ctx context.Context, sessionID string) (*rpc.CheckResult, error) {
	if a.sessionCache == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}

	state, ok := a.sessionCache.Get(sessionID)
	if !ok {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}

	state.Lock()
	defer func() {
		state.Unlock()
		a.sessionCache.Remove(sessionID)
	}()

	if state.sensitive {
		return &rpc.CheckResult{IsSensitive: true, Reason: state.reason}, nil
	}

	textToCheck := state.buffer.String()
	req := commontypes.LLMCheckRequest{
		Scenario:  commontypes.ScenarioLLMResModeration,
		Text:      textToCheck,
		SessionId: sessionID,
		Resumable: false,
		Stream:    true,
	}
	if textToCheck == "" {
		// set end text to trigger check of the end of the session stream
		go func() {
			req.Text = "[Done]"
			cancelCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = a.modImpl.modSvcClient.PassLLMRespCheck(cancelCtx, req)
		}()
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	result, err := a.modImpl.modSvcClient.PassLLMRespCheck(ctx, req)
	a.modImpl.postCheck(ctx, result)
	return result, err
}

func NewModerationImpl(config *config.Config) Moderation {
	cacheClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil
	}

	modImpl := &moderationImpl{
		modSvcClient:        rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port)),
		cacheClient:         cacheClient,
		promptMaxChunks:     promptMaxChunksFromConfig(config),
		pendingPromptChecks: newPendingPromptCheckCache(),
		config:              config,
	}

	initStreamChecker(modImpl)
	return modImpl
}

func NewModerationImplWithClient(config *config.Config, modSvcClient rpc.ModerationSvcClient, cacheClient cache.RedisClient) Moderation {
	maxContentLength := config.SensitiveCheck.MaxContentLength
	if config.SensitiveCheck.MaxContentLength <= 0 {
		maxContentLength = defaultMaxContentLength
	}
	modImpl := &moderationImpl{
		modSvcClient:        modSvcClient,
		cacheClient:         cacheClient,
		maxContentLength:    maxContentLength,
		promptMaxChunks:     promptMaxChunksFromConfig(config),
		pendingPromptChecks: newPendingPromptCheckCache(),
		pendingVerdictWait:  asyncPromptCheckTimeout,
		config:              config,
	}

	initStreamChecker(modImpl)
	return modImpl
}

func newPendingPromptCheckCache() *lru.Cache[string, *pendingPromptCheck] {
	checks, err := lru.New[string, *pendingPromptCheck](defaultSessionCacheSize)
	if err != nil {
		slog.Error("failed to init pending prompt check cache, async prompt verdicts will not gate responses", slog.Any("error", err))
		return nil
	}
	return checks
}

func promptMaxChunksFromConfig(cfg *config.Config) int {
	if cfg == nil || cfg.SensitiveCheck.PromptMaxChunks <= 0 {
		return 0
	}
	return cfg.SensitiveCheck.PromptMaxChunks
}

func initStreamChecker(modImpl *moderationImpl) {
	isAsync := modImpl.config != nil && modImpl.config.SensitiveCheck.StreamCheckMode == StreamCheckModeAsync

	if isAsync {
		sessionCache, err := lru.New[string, *sessionState](defaultSessionCacheSize)
		if err != nil {
			slog.Error("failed to init moderation session cache, fallback to sync mode", slog.Any("error", err))
			modImpl.streamChecker = &syncStreamChecker{modImpl: modImpl}
			return
		}

		maxChars := DefaultAsyncBufferMaxChars
		if modImpl.config.SensitiveCheck.AsyncBufferMaxChars > 0 {
			maxChars = modImpl.config.SensitiveCheck.AsyncBufferMaxChars
		}

		modImpl.streamChecker = &asyncStreamChecker{
			modImpl:      modImpl,
			sessionCache: sessionCache,
			maxChars:     maxChars,
		}
	} else {
		modImpl.streamChecker = &syncStreamChecker{modImpl: modImpl}
	}
}

func splitContentIntoChunksByWindow(content string) []string {
	re := regexp.MustCompile(`[.?!]`)
	sentences := re.Split(content, -1)
	chunks := []string{}

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		if len(sentence) > slidingWindowSize {
			for i := 0; i < len(sentence); i += slidingWindowSize {
				end := i + slidingWindowSize
				if end > len(sentence) {
					end = len(sentence)
				}
				chunks = append(chunks, sentence[i:end])
			}
		} else {
			chunks = append(chunks, sentence)
		}
	}
	return chunks
}

//TODO: use cdc to get chunk

// cachedPromptChunkResult returns the cached moderation verdict for one
// content chunk, or false when caching is disabled or the chunk is not
// cached.
func (modImpl *moderationImpl) cachedPromptChunkResult(ctx context.Context, chunk string) (*rpc.CheckResult, bool) {
	if modImpl.cacheClient == nil {
		return nil, false
	}
	chunkHash := md5.Sum([]byte(chunk))
	cacheKey := moderationCachePromptPrefix + fmt.Sprintf("%x", chunkHash)
	cached, err := modImpl.cacheClient.Get(ctx, cacheKey)
	if err != nil {
		slog.Debug("failed to get cached prompt chunk in redis", slog.String("error", err.Error()))
		return nil, false
	}
	var result rpc.CheckResult
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return nil, false
	}
	return &result, true
}

// used for single chunk or short content
func (modImpl *moderationImpl) checkSingleChunk(ctx context.Context, content, key string, isStream bool) (*rpc.CheckResult, error) {
	if modImpl.cacheClient != nil {
		if cached, ok := modImpl.cachedPromptChunkResult(ctx, content); ok {
			slog.Debug("moderation check cache hit", slog.String("content", content))
			return cached, nil
		}
	}

	req := commontypes.LLMCheckRequest{
		Scenario:  commontypes.ScenarioLLMQueryModeration,
		Text:      content,
		AccountId: key,
		Resumable: true,
		Stream:    isStream,
	}
	result, err := modImpl.modSvcClient.PassLLMPromptCheck(ctx, req)
	if err != nil {
		return nil, err
	}

	if modImpl.cacheClient != nil {
		// Cache the result for the single chunk
		chunkHash := md5.Sum([]byte(content))
		cacheKey := moderationCachePromptPrefix + fmt.Sprintf("%x", chunkHash)
		resultBytes, err := json.Marshal(result)
		if err == nil {
			err := modImpl.cacheClient.SetEx(ctx, cacheKey, string(resultBytes), cacheTTL)
			if err != nil {
				slog.Warn("failed to cache moderation result", slog.String("error", err.Error()))
			}
		}
	}
	modImpl.postCheck(ctx, result)
	return result, nil
}

func (modImpl *moderationImpl) checkBuffer(
	ctx context.Context,
	content string,
	currentBufferChunks []string,
	key string,
	isStream bool,
) (*rpc.CheckResult, error) {
	req := commontypes.LLMCheckRequest{
		Scenario:  commontypes.ScenarioLLMQueryModeration,
		Text:      content,
		AccountId: key,
		Resumable: true,
		Stream:    isStream,
	}
	result, err := modImpl.modSvcClient.PassLLMPromptCheck(ctx, req)
	if err != nil {
		return nil, err
	}
	// Cache each chunk of the buffer under its own md5 key, whatever the
	// buffer verdict: caching sensitive chunk results is what lets the async
	// tier's replay scan block a repeated sensitive prompt without another
	// moderation call.
	if modImpl.cacheClient != nil {
		for _, chunk := range currentBufferChunks {
			chunkHash := md5.Sum([]byte(chunk))
			cacheKey := moderationCachePromptPrefix + fmt.Sprintf("%x", chunkHash)
			resultBytes, err := json.Marshal(result)
			if err == nil {
				err := modImpl.cacheClient.SetEx(ctx, cacheKey, string(resultBytes), cacheTTL)
				if err != nil {
					slog.Warn("failed to cache moderation result", slog.String("error", err.Error()))
				}
			}
		}
	}
	modImpl.postCheck(ctx, result)
	return result, nil
}

func (modImpl *moderationImpl) CheckChatPrompts(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, uuid string, isStream bool) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	for _, msg := range messages {
		role := *msg.GetRole()
		var content string
		switch rawContent := msg.GetContent().AsAny().(type) {
		case string:
			content = rawContent
		case *string:
			content = *rawContent
		case []interface{}:
			contentBuilder := strings.Builder{}
			for _, item := range rawContent {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if text, exists := itemMap["text"].(string); exists {
						contentBuilder.WriteString(text)
						contentBuilder.WriteString(" ")
					}
				}
			}
			content = contentBuilder.String()
		default:
			contentBytes, _ := json.Marshal(rawContent)
			content = string(contentBytes)
		}

		if strings.TrimSpace(content) == "" {
			continue
		}

		mode := types.TextModerationModeNonStream
		if isStream {
			mode = types.TextModerationModeStream
		}
		result, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: content,
			Key:     uuid,
			Phase:   types.TextModerationPhasePrompt,
			Mode:    mode,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to check message content: %w", err)
		}

		modImpl.postCheck(ctx, result)
		// If sensitive content found, return immediately
		if result.IsSensitive {
			slog.Debug("sensitive content found in chat message",
				slog.String("role", role),
				slog.String("reason", result.Reason))
			return result, nil
		}
	}

	return &rpc.CheckResult{IsSensitive: false}, nil
}

func (modImpl *moderationImpl) CheckText(ctx context.Context, req types.TextModerationRequest) (*rpc.CheckResult, error) {
	switch req.Phase {
	case types.TextModerationPhasePrompt:
		return modImpl.checkPromptText(ctx, req.Content, req.Key, req.Mode == types.TextModerationModeStream)
	case types.TextModerationPhaseResponse:
		if req.Mode == types.TextModerationModeStream {
			return modImpl.checkStreamResponseText(ctx, req.Content, req.Key)
		}
		// The writer session id travels in Key and equals the planner's id
		// when the writer derived it from the request context; the check
		// context may be deliberately detached from the request (e.g.
		// responsespkg.ModerationContext), so both keys are consulted. The
		// await also covers verdicts still in flight: a response must not
		// finish before its asynchronous prompt check did.
		if blocked, ok := modImpl.awaitAndBlockNonStream(ctx, req.Key); ok {
			return blocked, nil
		}
		return modImpl.checkResponseText(ctx, req.Content)
	default:
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
}

func (modImpl *moderationImpl) checkPromptText(ctx context.Context, content string, checkKey string, isStream bool) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if strings.TrimSpace(content) == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	result, err := modImpl.checkLLMPrompt(ctx, content, checkKey, isStream)
	if err != nil {
		return nil, err
	}
	modImpl.postCheck(ctx, result)
	return result, nil
}

// checkLLMPrompt checks if the prompt is sensitive.
// A prompt worth at most one moderation call (maxContentLength) is checked
// synchronously with caching. When promptMaxChunks is configured, a larger
// prompt is either checked in the background (within the limit, so the
// request does not wait on moderation and TTFT stays low) or skipped
// entirely (above the limit, where chunked checking would cost too much
// time and resources). With promptMaxChunks unset, every prompt is checked
// synchronously as before.
func (modImpl *moderationImpl) checkLLMPrompt(ctx context.Context, content, key string, isStream bool) (*rpc.CheckResult, error) {
	content = strings.ReplaceAll(content, `\\n`, "\n")
	content = strings.ReplaceAll(content, `\n`, "")
	if len(content) < modImpl.maxContentLength {
		return modImpl.checkSingleChunk(ctx, content, key, isStream)
	}

	if modImpl.promptMaxChunks > 0 {
		// Approximate estimate of the moderation calls the chunked check
		// below would make: sentence-boundary packing can make the real
		// count drift slightly in either direction.
		estimatedChunks := (len(content) + modImpl.maxContentLength - 1) / modImpl.maxContentLength
		if estimatedChunks > modImpl.promptMaxChunks {
			slog.WarnContext(ctx, "prompt exceeds moderation chunk limit, skipping prompt moderation",
				slog.Int("content_length", len(content)),
				slog.Int("estimated_chunks", estimatedChunks),
				slog.Int("max_chunks", modImpl.promptMaxChunks))
			return &rpc.CheckResult{IsSensitive: false}, nil
		}
		if estimatedChunks > 1 {
			// Replay protection: consult the per-chunk cache synchronously
			// before deferring to the background check, so content whose
			// chunks were already judged is answered without another async
			// pass (a cached sensitive chunk blocks the replay).
			if verdict := modImpl.cachedChunkVerdictFor(ctx, content); verdict != nil {
				return verdict, nil
			}
			// The background context carries no values, so the session id is
			// extracted before the goroutine starts.
			sessionID := types.ModerationSessionIDFromContext(ctx)
			modImpl.registerPendingPromptCheck(sessionID)
			go modImpl.checkLLMPromptAsync(content, key, isStream, estimatedChunks, sessionID)
			return &rpc.CheckResult{IsSensitive: false}, nil
		}
	}

	return modImpl.checkLLMPromptSync(ctx, content, key, isStream)
}

// checkLLMPromptAsync runs the full chunked prompt check in the background.
// The originating request has already proceeded, so a sensitive verdict can
// no longer block the model call; it is recorded under the request's
// moderation session id so the response-side checks gate what that request
// emits, and the per-chunk results are cached so replaying the same content
// is caught by the synchronous path.
func (modImpl *moderationImpl) checkLLMPromptAsync(content, key string, isStream bool, estimatedChunks int, sessionID string) {
	bgCtx, cancel := context.WithTimeout(context.Background(), asyncPromptCheckTimeout)
	defer cancel()

	result, err := modImpl.checkLLMPromptSync(bgCtx, content, key, isStream)
	if err != nil {
		slog.Warn("async prompt moderation failed",
			slog.Int("estimated_chunks", estimatedChunks),
			slog.Any("error", err))
		modImpl.completePendingPromptCheck(sessionID, nil)
		return
	}
	if result.IsSensitive {
		slog.Error("sensitive content found in async prompt moderation",
			slog.Any("reason", result.Reason),
			slog.Int("estimated_chunks", estimatedChunks),
			slog.String("session_id", sessionID))
	}
	modImpl.completePendingPromptCheck(sessionID, result)
}

// registerPendingPromptCheck reserves the tracking entry for one request's
// in-flight asynchronous check so response finalization can await it.
func (modImpl *moderationImpl) registerPendingPromptCheck(sessionID string) {
	if modImpl.pendingPromptChecks == nil || sessionID == "" {
		return
	}
	modImpl.pendingPromptChecks.Add(sessionID, &pendingPromptCheck{done: make(chan struct{})})
}

// completePendingPromptCheck stores the final verdict (nil on error) and
// wakes everything waiting for this check.
func (modImpl *moderationImpl) completePendingPromptCheck(sessionID string, result *rpc.CheckResult) {
	if modImpl.pendingPromptChecks == nil || sessionID == "" {
		return
	}
	if p, ok := modImpl.pendingPromptChecks.Get(sessionID); ok && p != nil {
		p.result = result
		close(p.done)
	}
}

// pendingPromptCheckFor returns the tracked check for the request-context id
// first, then the writer-provided session id (the two coincide when the
// writer derived its id from the request context).
func (modImpl *moderationImpl) pendingPromptCheckFor(ctx context.Context, sessionID string) *pendingPromptCheck {
	if modImpl.pendingPromptChecks == nil {
		return nil
	}
	ctxID := types.ModerationSessionIDFromContext(ctx)
	if ctxID != "" {
		if p, ok := modImpl.pendingPromptChecks.Get(ctxID); ok && p != nil {
			return p
		}
	}
	if sessionID != "" && sessionID != ctxID {
		if p, ok := modImpl.pendingPromptChecks.Get(sessionID); ok && p != nil {
			return p
		}
	}
	return nil
}

// awaitAndBlockNonStream consumes the request's pending prompt verdict at the
// non-stream finalization boundary and renders the blocked result when it is
// sensitive. The writer session id is consulted alongside the request-context
// id, because the check context may be deliberately detached from the request
// (e.g. responsespkg.ModerationContext).
func (modImpl *moderationImpl) awaitAndBlockNonStream(ctx context.Context, sessionID string) (*rpc.CheckResult, bool) {
	verdict, ok := modImpl.awaitPendingPromptVerdict(ctx, sessionID)
	if !ok {
		return nil, false
	}
	blocked := *verdict
	modImpl.postCheck(ctx, &blocked)
	slog.WarnContext(ctx, "blocking non-stream response: async prompt moderation found sensitive content",
		slog.String("session_id", sessionID), slog.Any("reason", blocked.Reason))
	return &blocked, true
}

// peekPendingPromptVerdict reports a landed sensitive verdict without
// consuming it or waiting; per-chunk stream checks consult it non-blockingly.
func (modImpl *moderationImpl) peekPendingPromptVerdict(ctx context.Context, sessionID string) (*rpc.CheckResult, bool) {
	p := modImpl.pendingPromptCheckFor(ctx, sessionID)
	if p == nil {
		return nil, false
	}
	select {
	case <-p.done:
	default:
		return nil, false
	}
	if p.result == nil || !p.result.IsSensitive {
		return nil, false
	}
	return p.result, true
}

// awaitPendingPromptVerdict waits for an in-flight check to finish and
// consumes the tracking entry, so response finalization gates on verdicts
// that have not landed yet. The wait runs on its own budget over a
// cancellation-detached copy of the context: it must not consume the
// caller's check-context timeout, which the follow-up response-text check
// still needs. When the budget expires the response proceeds and the verdict
// only feeds later replays.
func (modImpl *moderationImpl) awaitPendingPromptVerdict(ctx context.Context, sessionID string) (*rpc.CheckResult, bool) {
	p := modImpl.pendingPromptCheckFor(ctx, sessionID)
	if p == nil {
		return nil, false
	}
	wait := modImpl.pendingVerdictWait
	if wait <= 0 {
		wait = asyncPromptCheckTimeout
	}
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wait)
	defer cancel()
	select {
	case <-p.done:
	case <-waitCtx.Done():
		return nil, false
	}
	if ctxID := types.ModerationSessionIDFromContext(ctx); ctxID != "" {
		modImpl.pendingPromptChecks.Remove(ctxID)
	}
	if sessionID != "" {
		modImpl.pendingPromptChecks.Remove(sessionID)
	}
	if p.result == nil || !p.result.IsSensitive {
		return nil, false
	}
	return p.result, true
}

// cachedChunkVerdictFor scans the per-chunk cache synchronously. A cached
// sensitive chunk blocks the prompt outright; when every chunk is already
// cached safe it returns non-sensitive so a replay does not re-enter the
// background check. nil means at least one chunk is unjudged and the check
// must still run.
func (modImpl *moderationImpl) cachedChunkVerdictFor(ctx context.Context, content string) *rpc.CheckResult {
	if modImpl.cacheClient == nil {
		return nil
	}
	chunks := splitContentIntoChunksByWindow(content)
	if len(chunks) == 0 {
		return nil
	}
	for _, chunk := range chunks {
		result, ok := modImpl.cachedPromptChunkResult(ctx, chunk)
		if !ok {
			return nil
		}
		if result.IsSensitive {
			modImpl.postCheck(ctx, result)
			return result
		}
	}
	return &rpc.CheckResult{IsSensitive: false}
}

// checkLLMPromptSync checks a long prompt by splitting it into chunks.
// Cached chunks are skipped; the rest are packed into buffers of at most
// maxContentLength and checked with one moderation call per buffer.
func (modImpl *moderationImpl) checkLLMPromptSync(ctx context.Context, content, key string, isStream bool) (*rpc.CheckResult, error) {
	chunks := splitContentIntoChunksByWindow(content)

	// 1. Check individual chunks from cache
	unCheckedChunks := make([]string, 0)
	for _, chunk := range chunks {
		result, ok := modImpl.cachedPromptChunkResult(ctx, chunk)
		if !ok {
			unCheckedChunks = append(unCheckedChunks, chunk)
			continue
		}
		slog.Debug("moderation check cache hit for chunk", slog.String("chunk", chunk))
		modImpl.postCheck(ctx, result)
		if result.IsSensitive {
			return result, nil
		}
	}

	// 2. Check for sensitive combinations using sliding window with the remaining chunks
	var buffer strings.Builder
	var currentBufferChunks []string

	for _, chunk := range unCheckedChunks {
		separatorLen := 0
		if buffer.Len() > 0 {
			separatorLen = 1 // for "."
		}

		if buffer.Len()+separatorLen+len(chunk) > modImpl.maxContentLength && buffer.Len() > 0 {
			result, err := modImpl.checkBuffer(ctx, buffer.String(), currentBufferChunks, key, isStream)
			if err != nil {
				return nil, fmt.Errorf("failed to call moderation on buffer: %w", err)
			}
			if result.IsSensitive {
				slog.Debug("sensitive content in buffer", slog.String("reason", result.Reason), slog.String("buffer", buffer.String()))
				return result, nil
			}

			buffer.Reset()
			currentBufferChunks = []string{}
		}

		if buffer.Len() > 0 {
			buffer.WriteString(".")
		}
		buffer.WriteString(chunk)
		currentBufferChunks = append(currentBufferChunks, chunk)
	}

	// Check any remaining content in the buffer.
	if buffer.Len() > 0 {
		result, err := modImpl.checkBuffer(ctx, buffer.String(), currentBufferChunks, key, isStream)
		if err != nil {
			return nil, fmt.Errorf("failed to call moderation on remaining buffer: %w", err)
		}
		if result.IsSensitive {
			slog.Debug("sensitive content in remaining buffer", slog.String("reason", result.Reason), slog.String("buffer", buffer.String()))
			return result, nil
		}
	}

	return &rpc.CheckResult{IsSensitive: false}, nil
}

func (modImpl *moderationImpl) CheckChatStreamResponse(ctx context.Context, chunk types.ChatCompletionChunk, uuid string) (*rpc.CheckResult, error) {
	return modImpl.checkStreamResponseText(ctx, chatCompletionChunkModerationText(chunk), uuid)
}

func (modImpl *moderationImpl) checkStreamResponseText(ctx context.Context, content string, sessionID string) (*rpc.CheckResult, error) {
	if verdict, ok := modImpl.peekPendingPromptVerdict(ctx, sessionID); ok {
		blocked := *verdict
		modImpl.postCheck(ctx, &blocked)
		slog.WarnContext(ctx, "blocking response chunk: async prompt moderation found sensitive content",
			slog.String("session_id", sessionID), slog.Any("reason", blocked.Reason))
		return &blocked, nil
	}
	if modImpl.streamChecker == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	return modImpl.streamChecker.CheckStreamResponseText(ctx, content, sessionID)
}

func (modImpl *moderationImpl) CheckChatNonStreamResponse(ctx context.Context, completion types.ChatCompletion) (*rpc.CheckResult, error) {
	// Await the request's asynchronous prompt verdict before any early
	// return: a completion with empty content (tool calls only, the common
	// agent shape) must still be gated, since the client may execute the
	// returned calls. The session id travels on the request context.
	if verdict, ok := modImpl.awaitPendingPromptVerdict(ctx, ""); ok {
		blocked := *verdict
		modImpl.postCheck(ctx, &blocked)
		slog.WarnContext(ctx, "blocking non-stream response: async prompt moderation found sensitive content",
			slog.Any("reason", blocked.Reason))
		return &blocked, nil
	}
	if len(completion.Choices) == 0 {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	text := chatCompletionMessageModerationText(completion.Choices[0].Message)
	if strings.TrimSpace(text) == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	return modImpl.CheckText(ctx, types.TextModerationRequest{
		Content: text,
		Phase:   types.TextModerationPhaseResponse,
		Mode:    types.TextModerationModeNonStream,
	})
}

// chatCompletionMessageModerationText extracts the moderation text of one
// non-stream chat completion message, mirroring the responses adapter: tool
// calls (and the legacy function call) are part of the audited text, so an
// empty content with tool calls is still submitted for checking.
func chatCompletionMessageModerationText(message openai.ChatCompletionMessage) string {
	var b strings.Builder
	writeTextModerationPart(&b, message.Content)
	writeTextModerationPart(&b, message.Refusal)
	//nolint:staticcheck // legacy field still emitted by some upstreams and must stay audited
	writeTextModerationPart(&b, message.FunctionCall.Name)
	//nolint:staticcheck // legacy field still emitted by some upstreams and must stay audited
	writeTextModerationPart(&b, message.FunctionCall.Arguments)
	for _, call := range message.ToolCalls {
		writeTextModerationPart(&b, call.Function.Name)
		writeTextModerationPart(&b, call.Function.Arguments)
		writeTextModerationPart(&b, call.Custom.Name)
		writeTextModerationPart(&b, call.Custom.Input)
	}
	return b.String()
}

func (modImpl *moderationImpl) checkResponseText(ctx context.Context, content string) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if strings.TrimSpace(content) == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	result, err := modImpl.modSvcClient.PassTextCheck(ctx, commontypes.ScenarioChatDetection, content)
	if err != nil {
		return nil, err
	}
	modImpl.postCheck(ctx, result)
	return result, nil
}

func chatCompletionChunkModerationText(chunk types.ChatCompletionChunk) string {
	var b strings.Builder
	for _, choice := range chunk.Choices {
		writeTextModerationPart(&b, choice.Delta.Content)
		writeTextModerationPart(&b, choice.Delta.ReasoningContent)
		writeTextModerationPart(&b, choice.Delta.Refusal)
		for _, call := range choice.Delta.ToolCalls {
			writeTextModerationPart(&b, call.Function.Name)
			writeTextModerationPart(&b, call.Function.Arguments)
		}
	}
	return b.String()
}

func writeTextModerationPart(b *strings.Builder, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(text)
}

func (modImpl *moderationImpl) postCheck(ctx context.Context, result *rpc.CheckResult) {
	if result.IsSensitive {
		slog.ErrorContext(ctx, "sensitive content found", slog.Any("reason", result.Reason))
		// If ModerationBypassSensitiveCheck is enabled, don't block the response
		if modImpl.config != nil && modImpl.config.AIGateway.ModerationBypassSensitiveCheck {
			result.IsSensitive = false
			result.Reason = ""
		}
	}
}

func (modImpl *moderationImpl) CloseStreamCheck(ctx context.Context, uuid string) (*rpc.CheckResult, error) {
	if verdict, ok := modImpl.awaitPendingPromptVerdict(ctx, uuid); ok {
		blocked := *verdict
		modImpl.postCheck(ctx, &blocked)
		slog.WarnContext(ctx, "closing stream blocked by async prompt moderation verdict",
			slog.String("session_id", uuid), slog.Any("reason", blocked.Reason))
		return &blocked, nil
	}
	if modImpl.streamChecker != nil {
		return modImpl.streamChecker.CloseStreamCheck(ctx, uuid)
	}
	return &rpc.CheckResult{IsSensitive: false}, nil
}

// CheckImagePrompts checks image-generation prompts (also used by the speech
// and video handlers). These paths have no response-side consumer for an
// asynchronous verdict, so the async tier would let a first-time sensitive
// prompt pass unchecked; the full check therefore runs synchronously
// regardless of how many moderation calls it is worth.
func (modImpl *moderationImpl) CheckImagePrompts(ctx context.Context, prompt string, uuid string) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	content := strings.ReplaceAll(prompt, `\n`, "\n")
	content = strings.ReplaceAll(content, `\n`, "")
	if len(content) < modImpl.maxContentLength {
		return modImpl.checkSingleChunk(ctx, content, uuid, false)
	}
	return modImpl.checkLLMPromptSync(ctx, content, uuid, false)
}

func (modImpl *moderationImpl) CheckImage(ctx context.Context, completion types.ImageGenerationResponse) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if len(completion.Data) == 0 {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	for _, item := range completion.Data {
		if item.URL == "" && item.B64JSON == "" {
			continue
		}
		if item.URL != "" {
			slog.Debug("check image url", slog.String("url", item.URL))
		} else if item.B64JSON != "" {
			slog.Debug("check image b64json", slog.String("b64json", item.B64JSON))
		}
	}
	return &rpc.CheckResult{IsSensitive: false}, nil
}
