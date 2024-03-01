package inference

type Client interface {
	Predict(id ModelID, req *PredictRequest) (*PredictResponse, error)
	GetModelInfo(id ModelID) (ModelInfo, error)
}

type PredictRequest struct {
	Prompt string `json:"prompt"`
}

type PredictResponse struct {
	GeneratedText               string  `json:"generated_text"`
	NumInputTokens              int     `json:"num_input_tokens"`
	NumInputTokensBatch         int     `json:"num_input_tokens_batch"`
	NumGeneratedTokens          int     `json:"num_generated_tokens"`
	NumGeneratedTokensBatch     int     `json:"num_generated_tokens_batch"`
	PreprocessingTime           float64 `json:"preprocessing_time"`
	GenerationTime              float64 `json:"generation_time"`
	PostprocessingTime          float64 `json:"postprocessing_time"`
	GenerationTimePerToken      float64 `json:"generation_time_per_token"`
	GenerationTimePerTokenBatch float64 `json:"generation_time_per_token_batch"`
	NumTotalTokens              int     `json:"num_total_tokens"`
	NumTotalTokensBatch         int     `json:"num_total_tokens_batch"`
	TotalTime                   float64 `json:"total_time"`
	TotalTimePerToken           float64 `json:"total_time_per_token"`
	TotalTimePerTokenBatch      float64 `json:"total_time_per_token_batch"`
}
