package trace

import (
	"context"

	"opencsg.com/csghub-server/aigateway/types"
)

type LLMTracer interface {
	StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, GenerationRecorder)
	StartStreamingGeneration(ctx context.Context, input types.GenerationStart) (context.Context, GenerationRecorder)
	StartEmbedding(ctx context.Context, input types.EmbeddingStart) (context.Context, EmbeddingRecorder)
	Shutdown(ctx context.Context) error
}

type GenerationRecorder interface {
	SetUsage(usage types.TokenUsage)
	SetResponse(response types.GenerationResponse)
	SetFirstChunk(firstChunk types.GenerationFirstChunk)
	SetError(err error, code string)
	End()
}

type EmbeddingRecorder interface {
	SetResult(result types.EmbeddingResult)
	SetError(err error, code string)
	End()
}
