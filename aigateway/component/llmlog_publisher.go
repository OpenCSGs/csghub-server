package component

type LLMLogPublisher interface {
	PublishTrainingLog(message []byte) error
}
