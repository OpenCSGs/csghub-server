//go:build !ee && !saas

package component

type logPublisherImpl struct{}

func NewLLMLogPublisher(_ bool) LLMLogPublisher {
	return &logPublisherImpl{}
}

func (p *logPublisherImpl) PublishTrainingLog(message []byte) error {
	return nil
}
