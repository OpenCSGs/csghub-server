package inference

type ModelID struct {
	Owner, Name, Version string
}

var _ App = (*llmInferApp)(nil)

type llmInferApp struct {
	modelID ModelID
}

func Model(id ModelID) (App, error) {
	c := &llmInferApp{}
	return c, nil
}

func (c *llmInferApp) Predict(input string) (string, error) {
	// TODO:implement this method
	return "test result", nil
}
