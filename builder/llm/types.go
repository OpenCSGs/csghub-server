package llm

type VllmGPUTokenizeReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type VllmGPUTokenizeResponse struct {
	Count       int64   `json:"count"`
	MaxModelLen int64   `json:"max_model_len"`
	Tokens      []int64 `json:"tokens"`
}

type LlamacppTokenizeReq struct {
	Content string `json:"content"`
}
type LlamacppTokenizeResponse struct {
	Tokens []int64 `json:"tokens"`
}

type TGITokenizeReq struct {
	Inputs string `json:"inputs"`
}

type TGITokenizeSingleResponse struct {
	Id    int64  `json:"id"`
	Start int64  `json:"start"`
	Stop  int64  `json:"stop"`
	Text  string `json:"text"`
}

type TGITokenizeResponse []TGITokenizeSingleResponse

// ref https://huggingface.github.io/text-embeddings-inference/#/Text%20Embeddings%20Inference/tokenize
type TEIEmbeddingTokenizeReq struct {
	AddSpecialTokens bool    `json:"add_special_tokens"`
	Inputs           *string `json:"inputs"`
	PromptName       *string `json:"prompt_name"`
}

type TEISingleEmbeddingToken struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Special bool   `json:"special"`
	Start   *int   `json:"start"`
	Stop    *int   `json:"stop"`
}

type TEIEmbeddingTokenizeResponse [][]TEISingleEmbeddingToken
