package sample

import "time"

const (
	defaultSampleTimeout        = 30 * time.Second
	longRunningInferenceTimeout = 5 * time.Minute
)

// NewDefaultRegistry returns the sample providers supported by AIGateway.
// Add new protocol providers here as they are implemented.
func NewDefaultRegistry() *Registry {
	return NewRegistry(
		newProtocolProvider(chatCompletionsRoute, modelsL7Request(chatCompletionsRoute), chatCompletionsRequest, defaultSampleTimeout),
		newProtocolProvider(responsesRoute, modelsL7Request(responsesRoute), responsesRequest, defaultSampleTimeout),
		newProtocolProvider(messagesRoute, messagesModelsL7Request, messagesRequest, defaultSampleTimeout),
		newProtocolProvider(embeddingsRoute, modelsL7Request(embeddingsRoute), embeddingsRequest, defaultSampleTimeout),
		newProtocolProvider(rerankRoute, modelsL7Request(rerankRoute), rerankRequest, defaultSampleTimeout),
		newProtocolProvider(imageGenerationsRoute, modelsL7Request(imageGenerationsRoute), imageGenerationsRequest, longRunningInferenceTimeout),
		newProtocolProvider(imageEditsRoute, modelsL7Request(imageEditsRoute), imageEditsRequest, longRunningInferenceTimeout),
		newProtocolProvider(transcriptionsRoute, modelsL7Request(transcriptionsRoute), transcriptionsRequest, defaultSampleTimeout),
		newProtocolProvider(speechRoute, modelsL7Request(speechRoute), speechRequest, defaultSampleTimeout),
		newProtocolProvider(batchSpeechRoute, modelsL7Request(batchSpeechRoute), batchSpeechRequest, defaultSampleTimeout),
		newProtocolProvider(voiceUploadRoute, modelsL7Request(voiceUploadRoute), voiceUploadRequest, defaultSampleTimeout),
		newProtocolProvider(videoGenerationsRoute, modelsL7Request(videoGenerationsRoute), videoGenerationsRequest, defaultSampleTimeout),
	)
}
