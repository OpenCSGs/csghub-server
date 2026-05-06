package types

import "time"

const (
	DeployBenchmarkStatusPending   = "pending"
	DeployBenchmarkStatusRunning   = "running"
	DeployBenchmarkStatusSuccess   = "success"
	DeployBenchmarkStatusFailed    = "failed"
	DeployBenchmarkStatusSkipped   = "skipped"
	DeployBenchmarkStatusCancelled = "cancelled"
)

const (
	DeployBenchmarkTriggerSourceRunningWebhook = "runner_webhook"
	DeployBenchmarkTriggerSourceManual         = "manual"
	DeployBenchmarkTypeOpenAIChatCompletions   = "openai_chat_completions"
	DeployBenchmarkTypeOpenAIEmbeddings        = "openai_embeddings"
	DeployBenchmarkTypeOpenAIImageGeneration   = "openai_image_generation"
	DeployBenchmarkTypeOpenAIVideoGeneration   = "openai_video_generation"
)

const DeployBenchmarkMaxConcurrency = 100000

type DeployBenchmarkTemplate struct {
	APIPath             string            `json:"api_path"`
	Method              string            `json:"method"`
	Headers             map[string]string `json:"headers"`
	RequestBody         map[string]any    `json:"request_body"`
	RequestBodyVariants []map[string]any  `json:"request_body_variants,omitempty"`
	Stream              bool              `json:"stream"`
	ExpectedTask        string            `json:"expected_task"`
}

type DeployBenchmarkConfig struct {
	WarmupRequests  int     `json:"warmup_requests"`
	DurationSeconds int     `json:"duration_seconds"`
	Concurrency     int     `json:"concurrency"`
	MaxConcurrency  int     `json:"max_concurrency"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
	SuccessRateMin  float64 `json:"success_rate_min"`
	P95LatencyMsMax float64 `json:"p95_latency_ms_max"` // max allowed p95 latency in milliseconds, 0 means no limit
	TPMTarget       float64 `json:"tpm_target"`
	EnableStream    bool    `json:"enable_stream"`
	SampleMessage   string  `json:"sample_message"`
}

type DeployBenchmarkSummary struct {
	TotalRequests    int64   `json:"total_requests"`
	SuccessRequests  int64   `json:"success_requests"`
	FailedRequests   int64   `json:"failed_requests"`
	SuccessRate      float64 `json:"success_rate"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
	TTFTMs           float64 `json:"ttft_ms"`        // Time To First Token, only meaningful for streaming requests
	TTFTAvailable    bool    `json:"ttft_available"` // true if TTFT was measured from streaming response
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	TPM              float64 `json:"tpm"` // Tokens Per Minute
	RPS              float64 `json:"rps"` // Requests Per Second
}

type DeployBenchmarkTriggerReq struct {
	Concurrency     int     `json:"concurrency"`
	MaxConcurrency  int     `json:"max_concurrency"`
	DurationSeconds int     `json:"duration_seconds"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
	SuccessRateMin  float64 `json:"success_rate_min"`
	P95LatencyMsMax float64 `json:"p95_latency_ms_max"`
	TPMTarget       float64 `json:"tpm_target"`
	SampleMessage   string  `json:"sample_message"`
	EnableStream    *bool   `json:"enable_stream"`
}

type DeployBenchmarkTriggerResp struct {
	BenchmarkTaskID int64  `json:"benchmark_task_id"`
	WorkflowID      string `json:"workflow_id"`
}

type DeployBenchmarkReq struct {
	DeployActReq
	BenchmarkID int64 `json:"benchmark_id"`
	PageOpts
}

type DeployBenchmarkResp struct {
	ID                 int64                   `json:"id"`
	DeployID           int64                   `json:"deploy_id"`
	SourceDeployTaskID int64                   `json:"source_deploy_task_id"`
	WorkflowID         string                  `json:"workflow_id"`
	TriggerSource      string                  `json:"trigger_source"`
	Status             string                  `json:"status"`
	BenchmarkType      string                  `json:"benchmark_type"`
	RuntimeFramework   string                  `json:"runtime_framework"`
	Task               string                  `json:"task"`
	Endpoint           string                  `json:"endpoint"`
	Summary            DeployBenchmarkSummary  `json:"summary"`
	BenchmarkConfig    DeployBenchmarkConfig   `json:"benchmark_config"`
	RequestTemplate    DeployBenchmarkTemplate `json:"request_template"`
	ErrorMessage       string                  `json:"error_message"`
	StartedAt          *time.Time              `json:"started_at,omitempty"`
	FinishedAt         *time.Time              `json:"finished_at,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type DeployBenchmarkWorkflowInput struct {
	BenchmarkTaskID    int64                   `json:"benchmark_task_id"`
	DeployID           int64                   `json:"deploy_id"`
	SourceDeployTaskID int64                   `json:"source_deploy_task_id"`
	TriggerSource      string                  `json:"trigger_source"`
	TriggerKey         string                  `json:"trigger_key"`
	BenchmarkType      string                  `json:"benchmark_type"`
	Endpoint           string                  `json:"endpoint"`
	Host               string                  `json:"host"`
	SvcName            string                  `json:"svc_name"`
	ClusterID          string                  `json:"cluster_id"`
	RuntimeFramework   string                  `json:"runtime_framework"`
	PipelineTask       string                  `json:"pipeline_task"`
	Hardware           map[string]any          `json:"hardware"`
	MinReplica         int                     `json:"min_replica"`
	MaxReplica         int                     `json:"max_replica"`
	OwnerNamespace     string                  `json:"owner_namespace"`
	UserUUID           string                  `json:"user_uuid"`
	RequestTemplate    DeployBenchmarkTemplate `json:"request_template"`
	Config             DeployBenchmarkConfig   `json:"config"`
}

type DeployBenchmarkWorkflowResult struct {
	BenchmarkTaskID int64                  `json:"benchmark_task_id"`
	Status          string                 `json:"status"`
	Summary         DeployBenchmarkSummary `json:"summary"`
	RawResult       map[string]any         `json:"raw_result"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
}

type DeployBenchmarkScriptInput struct {
	Endpoint        string                  `json:"endpoint"`
	Host            string                  `json:"host"`
	RequestTemplate DeployBenchmarkTemplate `json:"request_template"`
	Config          DeployBenchmarkConfig   `json:"config"`
	PreparedBodies  [][]byte                `json:"-"`
	IsFirstAttempt  bool                    `json:"-"`
}

type DeployBenchmarkScriptResult struct {
	Summary   DeployBenchmarkSummary `json:"summary"`
	RawResult map[string]any         `json:"raw_result"`
}

type DeployBenchmarkLaunchReq struct {
	Deploy             DeployRequest              `json:"deploy"`
	SourceDeployTaskID int64                      `json:"source_deploy_task_id"`
	TriggerSource      string                     `json:"trigger_source"`
	TriggerKey         string                     `json:"trigger_key"`
	ManualOverride     *DeployBenchmarkTriggerReq `json:"manual_override,omitempty"`
}

func ResolveDeployBenchmarkType(task PipelineTask) (string, bool) {
	switch task {
	case TextGeneration, ImageText2Text, VideoText2Text:
		return DeployBenchmarkTypeOpenAIChatCompletions, true
	case FeatureExtraction, SentenceSimilarity:
		return DeployBenchmarkTypeOpenAIEmbeddings, true
	case Text2Image:
		return DeployBenchmarkTypeOpenAIImageGeneration, true
	case Text2Video:
		return DeployBenchmarkTypeOpenAIVideoGeneration, true
	default:
		return "", false
	}
}

func IsDeployBenchmarkTaskSupported(task PipelineTask) bool {
	_, ok := ResolveDeployBenchmarkType(task)
	return ok
}

func IsDeployTypeBenchmarkSupported(drType int) bool {
	return drType == ServerlessType
}
