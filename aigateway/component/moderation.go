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
	moderationCachePrpmptPrefix = "moderation:prompt:"
	// default session cache size
	defaultSessionCacheSize = 10000

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
		modSvcClient: rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port)),
		cacheClient:  cacheClient,
		config:       config,
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
		modSvcClient:     modSvcClient,
		cacheClient:      cacheClient,
		maxContentLength: maxContentLength,
		config:           config,
	}

	initStreamChecker(modImpl)
	return modImpl
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

// used for single chunk or short content
func (modImpl *moderationImpl) checkSingleChunk(ctx context.Context, content, key string, isStream bool) (*rpc.CheckResult, error) {
	if modImpl.cacheClient != nil {
		chunkHash := md5.Sum([]byte(content))
		cacheKey := moderationCachePrpmptPrefix + fmt.Sprintf("%x", chunkHash)
		cached, err := modImpl.cacheClient.Get(ctx, cacheKey)
		if err == nil {
			var result rpc.CheckResult
			if err := json.Unmarshal([]byte(cached), &result); err == nil {
				slog.Debug("moderation check cache hit", slog.String("content", content))
				return &result, nil
			}
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
		cacheKey := moderationCachePrpmptPrefix + content
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
	// TODO: if result is sensitive, cache unsensitive chunks
	modImpl.postCheck(ctx, result)
	if result.IsSensitive {
		return result, nil
	}
	// Buffer check passed
	if modImpl.cacheClient != nil {
		// cache each chunk in the current buffer
		for _, chunk := range currentBufferChunks {
			chunkHash := md5.Sum([]byte(chunk))
			cacheKey := moderationCachePrpmptPrefix + fmt.Sprintf("%x", chunkHash)
			resultBytes, err := json.Marshal(result)
			if err == nil {
				err := modImpl.cacheClient.SetEx(ctx, cacheKey, string(resultBytes), cacheTTL)
				if err != nil {
					slog.Warn("failed to cache moderation result", slog.String("error", err.Error()))
				}
			}
		}
	}
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

// CheckLLMPrompt checks if the prompt is sensitive.
// For long content, it first checks each chunk individually (with caching).
// Then, it uses a sliding window to check for sensitive combinations of chunks.
func (modImpl *moderationImpl) checkLLMPrompt(ctx context.Context, content, key string, isStream bool) (*rpc.CheckResult, error) {
	content = strings.ReplaceAll(content, `\\n`, "\n")
	content = strings.ReplaceAll(content, `\n`, "")
	if len(content) < modImpl.maxContentLength {
		return modImpl.checkSingleChunk(ctx, content, key, isStream)
	}

	chunks := splitContentIntoChunksByWindow(content)

	// 1. First check individual chunks from cache
	unCheckedChunks := make([]string, 0)
	for _, chunk := range chunks {
		// Check if chunk is in cache
		if modImpl.cacheClient != nil {
			chunkHash := md5.Sum([]byte(chunk))
			cacheKey := moderationCachePrpmptPrefix + fmt.Sprintf("%x", chunkHash)
			cached, err := modImpl.cacheClient.Get(ctx, cacheKey)
			if err == nil {
				var result rpc.CheckResult
				if err = json.Unmarshal([]byte(cached), &result); err == nil {
					slog.Debug("moderation check cache hit for chunk", slog.String("chunk", chunk))
					modImpl.postCheck(ctx, &result)
					if result.IsSensitive {
						return &result, nil
					} else {
						continue // Skip this chunk as it's already in cache
					}
				}
			} else {
				slog.Debug("failed to get cache chunk in redis",
					slog.String("error", err.Error()))
			}
		}

		// If not in cache, add to unCheckedChunks for further checking
		unCheckedChunks = append(unCheckedChunks, chunk)
	}

	// 2. Check for sensitive combinations using sliding window with the remaining chunks
	var buffer strings.Builder
	var currentBufferChunks []string

	for _, chunk := range unCheckedChunks {
		if modImpl.cacheClient != nil {
			chunkHash := md5.Sum([]byte(chunk))
			cacheKey := moderationCachePrpmptPrefix + fmt.Sprintf("%x", chunkHash)
			cached, err := modImpl.cacheClient.Get(ctx, cacheKey)
			if err == nil {
				var result rpc.CheckResult
				if err = json.Unmarshal([]byte(cached), &result); err == nil {
					slog.Debug("moderation check cache hit for chunk", slog.String("chunk", chunk))
					modImpl.postCheck(ctx, &result)
					if result.IsSensitive {
						return &result, nil
					} else {
						continue // Skip this chunk as it's already in cache
					}
				}
			} else {
				slog.Debug("failed to get cache chunk in redis",
					slog.String("error", err.Error()))
			}
		}

		separatorLen := 0
		if buffer.Len() > 0 {
			separatorLen = 1 // for "."
		}

		if buffer.Len()+separatorLen+len(chunk) > modImpl.maxContentLength && buffer.Len() > 0 {
			result, err := modImpl.checkBuffer(ctx, buffer.String(), currentBufferChunks, key, isStream)
			if err != nil {
				return nil, fmt.Errorf("failed to call moderation on buffer: %w", err)
			}
			modImpl.postCheck(ctx, result)
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
		modImpl.postCheck(ctx, result)
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
	if modImpl.streamChecker == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	return modImpl.streamChecker.CheckStreamResponseText(ctx, content, sessionID)
}

func (modImpl *moderationImpl) CheckChatNonStreamResponse(ctx context.Context, completion types.ChatCompletion) (*rpc.CheckResult, error) {
	if len(completion.Choices) == 0 {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if completion.Choices[0].Message.Content == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	return modImpl.CheckText(ctx, types.TextModerationRequest{
		Content: completion.Choices[0].Message.Content,
		Key:     "",
		Phase:   types.TextModerationPhaseResponse,
		Mode:    types.TextModerationModeNonStream,
	})
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
	if modImpl.streamChecker != nil {
		return modImpl.streamChecker.CloseStreamCheck(ctx, uuid)
	}
	return &rpc.CheckResult{IsSensitive: false}, nil
}

func (modImpl *moderationImpl) CheckImagePrompts(ctx context.Context, prompt string, uuid string) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	return modImpl.checkLLMPrompt(ctx, prompt, uuid, false)
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
