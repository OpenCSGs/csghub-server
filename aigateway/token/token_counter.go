package token

type Counter interface {
	Usage() (*Usage, error)
}

type Usage struct {
	TotalTokens      int64
	PromptTokens     int64
	CompletionTokens int64
}
