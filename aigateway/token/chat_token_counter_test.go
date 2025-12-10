package token_test

import (
	"context"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestChatTokenCounter_Usage_WithCompletion(t *testing.T) {
	// Test that when completion exists, directly return usage from completion
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)

	// Set up completion
	completion := types.ChatCompletion{
		ChatCompletion: openai.ChatCompletion{
			Usage: openai.CompletionUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
	}
	counter.Completion(completion)

	// Call Usage method
	usage, err := counter.Usage(context.Background())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, int64(10), usage.PromptTokens)
	assert.Equal(t, int64(20), usage.CompletionTokens)
	assert.Equal(t, int64(30), usage.TotalTokens)
}

func TestChatTokenCounter_Usage_WithChunksContainingUsage(t *testing.T) {
	// Test that when completion doesn't exist but chunks contain usage, return usage from chunks
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)

	// Add chunks, last chunk contains usage information
	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID:      "test-id",
		Choices: []types.ChatCompletionChunkChoice{},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})
	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "Hello",
				},
			},
		},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage: openai.CompletionUsage{
			PromptTokens:     15,
			CompletionTokens: 25,
			TotalTokens:      40,
		},
	})

	usage, err := counter.Usage(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(15), usage.PromptTokens)
	assert.Equal(t, int64(25), usage.CompletionTokens)
	assert.Equal(t, int64(40), usage.TotalTokens)
}

func TestChatTokenCounter_Usage_WithTokenizer(t *testing.T) {
	// Test that when neither completion nor chunks have usage, calculate usage using tokenizer
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)

	// Set up mock tokenizer behavior
	tokenizer.On("Encode", mock.Anything).Return(int64(5), nil)

	// Add prompt
	userContent := "Hello, how are you?"
	counter.AppendPrompts([]openai.ChatCompletionMessageParamUnion{openai.UserMessage(userContent)})

	// Add chunks (no usage information)
	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "I'm fine, thank you!",
				},
			},
		},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})

	usage, err := counter.Usage(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(8), usage.PromptTokens)     // 5 (prompt) + 3 (between)
	assert.Equal(t, int64(5), usage.CompletionTokens) // 5 (completion)
	assert.Equal(t, int64(13), usage.TotalTokens)     // 8 + 5

	// Verify tokenizer was called
	tokenizer.AssertCalled(t, "Encode", mock.Anything)
}

func TestChatTokenCounter_Usage_WithoutTokenizer(t *testing.T) {
	// Test that when tokenizer is nil, return error
	userContent := "Hello, how are you?"
	counter := token.NewLLMTokenCounter(nil)

	counter.AppendPrompts([]openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(userContent),
	})

	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID:      "test-id",
		Choices: []types.ChatCompletionChunkChoice{},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})

	usage, err := counter.Usage(context.Background())

	assert.Error(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, "no usage found in completion, and tokenizer not set", err.Error())
}

func TestChatTokenCounter_Usage_WithTextContentParts(t *testing.T) {
	// Test handling of prompt content with type []ChatCompletionContentPartTextParam
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)

	tokenizer.EXPECT().Encode(mock.Anything).Return(int64(10), nil)

	// Add prompt with text content parts
	userContent := "Hello, how are you?"
	counter.AppendPrompts([]openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(userContent),
	})

	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "I'm fine",
				},
			},
		},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})

	usage, err := counter.Usage(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(13), usage.PromptTokens)     // 10 (prompt) + 3 (between)
	assert.Equal(t, int64(10), usage.CompletionTokens) // 10 (completion)
	assert.Equal(t, int64(23), usage.TotalTokens)      // 13 + 10
}

func TestChatTokenCounter_Usage_WithTool(t *testing.T) {
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)
	tokenizer.EXPECT().Encode(mock.Anything).Return(int64(10), nil)
	prompts := []openai.ChatCompletionMessageParamUnion{
		openai.ToolMessage([]openai.ChatCompletionContentPartTextParam{
			{
				Text: "Hello, how are you?",
			},
		}, "tool-1"),
	}
	counter.AppendPrompts(prompts)
	usage, err := counter.Usage(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(13), usage.PromptTokens)
	assert.Equal(t, int64(10), usage.CompletionTokens)
	assert.Equal(t, int64(23), usage.TotalTokens)
}

func TestChatTokenCounter_Usage_WithMixedContentParts(t *testing.T) {
	// Test handling of prompt content with type []ChatCompletionContentPartUnionParam
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)
	tokenizer.EXPECT().Encode(mock.Anything).Return(int64(8), nil)

	// Add prompt with mixed content parts
	mixedParts := []openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart("Hello, "),
		openai.TextContentPart("world!"),
	}
	counter.AppendPrompts([]openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(mixedParts),
	})

	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "Hi there",
				},
			},
		},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})

	usage, err := counter.Usage(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(11), usage.PromptTokens)    // 8 (prompt) + 3 (between)
	assert.Equal(t, int64(8), usage.CompletionTokens) // 8 (completion)
	assert.Equal(t, int64(19), usage.TotalTokens)     // 11 + 8
}

func TestChatTokenCounter_Usage_WithAssistantMessageContentParts(t *testing.T) {
	// Test handling of assistant message content with type []ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)

	// Mock the tokenizer to return different token counts for prompt and completion
	// For assistant message with content parts array, content should be empty string
	tokenizer.EXPECT().Encode(types.Message{
		Content: "",
		Role:    "",
	}).Return(int64(3), nil).Once()

	// For completion, return a specific token count
	tokenizer.EXPECT().Encode(types.Message{
		Content: "Assistant response",
	}).Return(int64(5), nil).Once()

	// Add assistant message with content parts array
	// Note: In OpenAI Go SDK v3.8.1, we need to create an assistant message with content parts
	assistantMsg := openai.AssistantMessage(
		[]openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
			{
				OfText: &openai.ChatCompletionContentPartTextParam{
					Text: "This is part one. ",
				},
			},
		})
	counter.AppendPrompts([]openai.ChatCompletionMessageParamUnion{assistantMsg})

	// Add completion chunk
	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "Assistant response",
				},
			},
		},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})

	// Call Usage method
	usage, err := counter.Usage(context.Background())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, int64(6), usage.PromptTokens)     // 3 (assistant msg with empty content) + 3 (between)
	assert.Equal(t, int64(5), usage.CompletionTokens) // 5 (completion)
	assert.Equal(t, int64(11), usage.TotalTokens)     // 6 + 5

	// Verify tokenizer was called correctly
	tokenizer.AssertExpectations(t)
}

func TestChatTokenCounter_Usage_WithImageAudioFileContentParts(t *testing.T) {
	// Test handling of image, audio, and file content parts
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewLLMTokenCounter(tokenizer)

	// Mock the tokenizer to return specific token counts
	// For mixed content parts including images, audio, and files, only text content should be included
	tokenizer.EXPECT().Encode(types.Message{
		Content: "Text content",
		Role:    "",
	}).Return(int64(10), nil).Once()

	// For completion, return a specific token count
	tokenizer.EXPECT().Encode(types.Message{
		Content: "Completion response",
	}).Return(int64(8), nil).Once()

	// Add user message with mixed content parts including image, audio, and file
	mixedParts := []openai.ChatCompletionContentPartUnionParam{
		// Text content part (should be included)
		openai.TextContentPart("Text content"),
		// Image content part (should be ignored with warning)
		openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: "https://example.com/image.jpg",
		}),
		// Audio content part (should be ignored with warning)
		openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
			Data: "https://example.com/audio.mp3",
		}),
		// File content part (should be ignored with warning)
		openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
			FileID: param.Opt[string]{
				Value: "file-123456789",
			},
		}),
	}

	counter.AppendPrompts([]openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(mixedParts),
	})

	// Add completion chunk
	counter.AppendCompletionChunk(types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "Completion response",
				},
			},
		},
		Created: 1234567890,
		Model:   "test-model",
		Object:  new(constant.ChatCompletion).Default(),
		Usage:   openai.CompletionUsage{},
	})

	// Call Usage method
	usage, err := counter.Usage(context.Background())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, int64(13), usage.PromptTokens)    // 10 (text content) + 3 (between)
	assert.Equal(t, int64(8), usage.CompletionTokens) // 8 (completion)
	assert.Equal(t, int64(21), usage.TotalTokens)     // 13 + 8

	// Verify tokenizer was called correctly
	tokenizer.AssertExpectations(t)
}
