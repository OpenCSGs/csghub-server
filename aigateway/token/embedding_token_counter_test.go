package token_test

import (
	"context"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
)

func TestEmbeddingTokenCounter_Usage_WithEmbeddingResponse(t *testing.T) {
	// Test that when embedding response is set, Usage method returns usage from response
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewEmbeddingTokenCounter(tokenizer)

	// Set up embedding response
	embeddingUsage := openai.CreateEmbeddingResponseUsage{
		PromptTokens: 10,
		TotalTokens:  10,
	}
	counter.Embedding(embeddingUsage)

	// Call Usage method
	usage, err := counter.Usage(context.Background())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, int64(10), usage.PromptTokens)
	assert.Equal(t, int64(10), usage.TotalTokens)
	assert.Equal(t, int64(0), usage.CompletionTokens)
}

func TestEmbeddingTokenCounter_Usage_WithTokenizer(t *testing.T) {
	// Test that when embedding response is not set but tokenizer is available, use tokenizer to count tokens
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewEmbeddingTokenCounter(tokenizer)

	// Set up mock tokenizer behavior
	inputText := "Hello, world!"
	tokenizer.On("EmbeddingEncode", inputText).Return(int64(5), nil)

	// Set input
	counter.Input(inputText)

	// Call Usage method
	usage, err := counter.Usage(context.Background())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, int64(5), usage.PromptTokens)
	assert.Equal(t, int64(5), usage.TotalTokens)
	assert.Equal(t, int64(0), usage.CompletionTokens)

	// Verify tokenizer was called
	tokenizer.AssertCalled(t, "EmbeddingEncode", inputText)
}

func TestEmbeddingTokenCounter_Usage_WithoutTokenizer(t *testing.T) {
	// Test that when tokenizer is nil and no embedding response, return error
	inputText := "Hello, world!"
	counter := token.NewEmbeddingTokenCounter(nil)

	counter.Input(inputText)

	usage, err := counter.Usage(context.Background())

	assert.Error(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, "no usage found in embedding response, and tokenizer not set", err.Error())
}

func TestEmbeddingTokenCounter_Usage_WithDumyTokenizer(t *testing.T) {
	// Test with the existing DumyTokenizer
	tokenizer := &token.DumyTokenizer{}
	counter := token.NewEmbeddingTokenCounter(tokenizer)

	// Set input
	inputText := "Hello, world!"
	counter.Input(inputText)

	// Call Usage method
	usage, err := counter.Usage(context.Background())

	// Verify results
	assert.NoError(t, err)
	// DumyTokenizer counts characters as tokens
	assert.Equal(t, int64(len(inputText)), usage.PromptTokens)
	assert.Equal(t, int64(len(inputText)), usage.TotalTokens)
	assert.Equal(t, int64(0), usage.CompletionTokens)
}

func TestEmbeddingTokenCounter_Input(t *testing.T) {
	// Test that Input method correctly sets the input text
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewEmbeddingTokenCounter(tokenizer)

	// Set input
	inputText := "Test input text"
	counter.Input(inputText)

	// Verify input is stored correctly by calling Usage and checking tokenizer call
	tokenizer.EXPECT().EmbeddingEncode(inputText).Return(int64(5), nil)
	usage, err := counter.Usage(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), usage.PromptTokens)
	assert.Equal(t, int64(5), usage.TotalTokens)
	assert.Equal(t, int64(0), usage.CompletionTokens)
}

func TestEmbeddingTokenCounter_Embedding(t *testing.T) {
	// Test that Embedding method correctly sets the embedding usage
	tokenizer := mocktoken.NewMockTokenizer(t)
	counter := token.NewEmbeddingTokenCounter(tokenizer)

	// Set up embedding response
	embeddingUsage := openai.CreateEmbeddingResponseUsage{
		PromptTokens: 20,
		TotalTokens:  20,
	}
	counter.Embedding(embeddingUsage)

	// Verify embedding usage is stored correctly by calling Usage
	usage, err := counter.Usage(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(20), usage.PromptTokens)
	assert.Equal(t, int64(20), usage.TotalTokens)
}
