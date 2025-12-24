package token

import "context"

type CreateParam struct {
	Endpoint string
	Host     string
	Model    string
	ImageID  string
}

type CounterFactory interface {
	NewChat(param CreateParam) ChatTokenCounter
	NewEmbedding(param CreateParam) EmbeddingTokenCounter
}

func NewCounterFactory() CounterFactory {
	return &counterFactoryImpl{}
}

type counterFactoryImpl struct{}

func (f *counterFactoryImpl) NewChat(param CreateParam) ChatTokenCounter {
	tokenizer := NewTokenizerImpl(param.Endpoint, param.Host, param.Model, param.ImageID)
	return NewLLMTokenCounter(tokenizer)
}

func (f *counterFactoryImpl) NewEmbedding(param CreateParam) EmbeddingTokenCounter {
	tokenizer := NewTokenizerImpl(param.Endpoint, param.Host, param.Model, param.ImageID)
	return NewEmbeddingTokenCounter(tokenizer)
}

type Counter interface {
	Usage(context.Context) (*Usage, error)
}

type Usage struct {
	TotalTokens      int64
	PromptTokens     int64
	CompletionTokens int64
}
