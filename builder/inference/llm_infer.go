package inference

import "opencsg.com/csghub-server/common/config"

type Client interface {
	Predict(string) (string, error)
}

var _ Client = (*llmInferClient)(nil)

type llmInferClient struct{}

func NewClient(cfg *config.Config) (Client, error) {
	c := &llmInferClient{}
	return c, nil
}

func (c *llmInferClient) Predict(input string) (string, error) {
	// TODO:implement this method
	return "test result", nil
}
