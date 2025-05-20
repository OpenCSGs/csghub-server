package types

type MonitorReq struct {
	CurrentUser  string         `json:"current_user"`
	Namespace    string         `json:"namespace"`
	Name         string         `json:"name"`
	RepoType     RepositoryType `json:"repo_type"`
	DeployType   string         `json:"deploy_type"`
	DeployID     int64          `json:"deploy_id"`
	Instance     string         `json:"instance"`
	LastDuration string         `json:"last_duration"`
	TimeRange    string         `json:"time_range"`
}

// prometheus response definition

type PrometheusResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
	Value  []any             `json:"value"`
}

type PrometheusData struct {
	ResultType string             `json:"resultType"`
	Result     []PrometheusResult `json:"result"`
}

type PrometheusResponse struct {
	Status string         `json:"status"`
	Data   PrometheusData `json:"data"`
}

// monitor response definition

type MonitorValue struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type MonitorData struct {
	Metric map[string]string `json:"metric"`
	Values []MonitorValue    `json:"values"`
	Value  MonitorValue      `json:"value"`
}

type MonitorLatency struct {
	Metric map[string]string `json:"metric"`
	Value  MonitorValue      `json:"value"`
}

type MonitorCPUResp struct {
	ResultType string        `json:"result_type"`
	Result     []MonitorData `json:"result"`
}

type MonitorMemoryResp = MonitorCPUResp
type MonitorRequestCountResp struct {
	ResultType        string        `json:"result_type"`
	Result            []MonitorData `json:"result"`
	TotalRequestCount int64         `json:"total_request_count"`
}
type MonitorRequestLatencyResp = MonitorCPUResp

var MonitorValidDurations = map[string]string{
	"30m": "1m",
	"1h":  "1m",
	"3h":  "5m",
	"6h":  "5m",
	"12h": "5m",
	"1d":  "10m",
	"3d":  "30m",
	"1w":  "60m",
}
