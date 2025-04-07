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

type TGITokenizeResponse struct {
	Id    int64  `json:"id"`
	Start int64  `json:"start"`
	Stop  int64  `json:"stop"`
	Text  string `json:"text"`
}

type SGLangTokenizeReq struct {
}

type SGLangTokenizeResponse struct {
}
