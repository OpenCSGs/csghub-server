package component

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
)

const (
	// max content length for moderation
	maxContentLength = 2000
	// sliding window size
	slidingWindowSize = 2000
	// cache ttl
	cacheTTL = 24 * time.Hour
	// moderation cache prefix
	moderationCachePrpmptPrefix = "moderation:prompt:"
)

type Moderation interface {
	CheckChatPrompts(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, uuid string) (*rpc.CheckResult, error)
	CheckChatStreamResponse(ctx context.Context, chunk types.ChatCompletionChunk, uuid string) (*rpc.CheckResult, error)
	CheckChatNonStreamResponse(ctx context.Context, completion types.ChatCompletion) (*rpc.CheckResult, error)
}

type moderationImpl struct {
	modSvcClient rpc.ModerationSvcClient
	cacheClient  cache.RedisClient
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
	return &moderationImpl{
		modSvcClient: rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port)),
		cacheClient:  cacheClient,
	}
}

func NewModerationImplWithClient(modSvcClient rpc.ModerationSvcClient, cacheClient cache.RedisClient) Moderation {
	return &moderationImpl{
		modSvcClient: modSvcClient,
		cacheClient:  cacheClient,
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
func (modImpl *moderationImpl) checkSingleChunk(ctx context.Context, content, key string) (*rpc.CheckResult, error) {
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

	result, err := modImpl.modSvcClient.PassLLMPromptCheck(ctx, content, key)
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
	return result, nil
}

func (modImpl *moderationImpl) checkBuffer(
	ctx context.Context,
	content string,
	currentBufferChunks []string,
	key string,
) (*rpc.CheckResult, error) {
	result, err := modImpl.modSvcClient.PassLLMPromptCheck(ctx, content, key)
	if err != nil {
		return nil, err
	}
	// TODO: if result is sensitive, cache unsensitive chunks
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

// CheckChatPrompts checks if any of the chat messages contain sensitive content.
// It processes each message, extracts text content, and uses CheckLLMPrompt for validation.
func (modImpl *moderationImpl) CheckChatPrompts(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, uuid string) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	// Process each message in the messages array
	for _, msg := range messages {
		// Skip system messages as they're typically predefined
		role := *msg.GetRole()

		// Handle different content types
		var content string
		switch rawContent := msg.GetContent().AsAny().(type) {
		case string:
			// Direct string content
			content = rawContent
		case *string:
			content = *rawContent
		case []interface{}:
			// Array content (e.g., for multi-modal inputs)
			contentBuilder := strings.Builder{}
			for _, item := range rawContent {
				// Try to extract text content from array items
				if itemMap, ok := item.(map[string]interface{}); ok {
					if text, exists := itemMap["text"].(string); exists {
						contentBuilder.WriteString(text)
						contentBuilder.WriteString(" ")
					}
				}
			}
			content = contentBuilder.String()
		default:
			// Convert to string as fallback
			contentBytes, _ := json.Marshal(rawContent)
			content = string(contentBytes)
		}

		// Skip empty content
		if strings.TrimSpace(content) == "" {
			continue
		}

		// Check if content is sensitive using existing method
		result, err := modImpl.checkLLMPrompt(ctx, content, uuid)
		if err != nil {
			return nil, fmt.Errorf("failed to check message content: %w", err)
		}

		// If sensitive content found, return immediately
		if result.IsSensitive {
			slog.Debug("sensitive content found in chat message",
				slog.String("role", role),
				slog.String("reason", result.Reason))
			return result, nil
		}
	}

	// No sensitive content found in any message
	return &rpc.CheckResult{IsSensitive: false}, nil
}

// CheckLLMPrompt checks if the prompt is sensitive.
// For long content, it first checks each chunk individually (with caching).
// Then, it uses a sliding window to check for sensitive combinations of chunks.
func (modImpl *moderationImpl) checkLLMPrompt(ctx context.Context, content, key string) (*rpc.CheckResult, error) {
	content = strings.ReplaceAll(content, `\\n`, "\n")
	content = strings.ReplaceAll(content, `\n`, "")
	if len(content) < maxContentLength {
		return modImpl.checkSingleChunk(ctx, content, key)
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

		if buffer.Len()+separatorLen+len(chunk) > maxContentLength && buffer.Len() > 0 {
			result, err := modImpl.checkBuffer(ctx, buffer.String(), currentBufferChunks, key)
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
		result, err := modImpl.checkBuffer(ctx, buffer.String(), currentBufferChunks, key)
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
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if len(chunk.Choices) == 0 {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if chunk.Choices[0].Delta.Content == "" && chunk.Choices[0].Delta.ReasoningContent == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}

	var result = &rpc.CheckResult{IsSensitive: false}
	var err error
	if strings.TrimSpace(chunk.Choices[0].Delta.Content) != "" {
		// moderate on content
		result, err = modImpl.modSvcClient.PassLLMRespCheck(ctx, chunk.Choices[0].Delta.Content, uuid)
	} else if strings.TrimSpace(chunk.Choices[0].Delta.ReasoningContent) != "" {
		// moderate on reasoning content
		result, err = modImpl.modSvcClient.PassLLMRespCheck(ctx, chunk.Choices[0].Delta.ReasoningContent, uuid)
	} else {
		slog.Error("Unknown data struct",
			slog.Any("raw data", chunk),
			slog.Any("unmarshal chunk", chunk))
	}
	return result, err
}

func (modImpl *moderationImpl) CheckChatNonStreamResponse(ctx context.Context, completion types.ChatCompletion) (*rpc.CheckResult, error) {
	if modImpl.modSvcClient == nil {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if len(completion.Choices) == 0 {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	if completion.Choices[0].Message.Content == "" {
		return &rpc.CheckResult{IsSensitive: false}, nil
	}
	return modImpl.modSvcClient.PassTextCheck(ctx, string(sensitive.ScenarioChatDetection), completion.Choices[0].Message.Content)
}
