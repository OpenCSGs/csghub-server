package loki

import "time"

// loki max limit
const MaxLimit int = 5000

// LokiStream represents a Loki log stream
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// DroppedEntry represents a dropped log entry indicator from Loki tail API
type DroppedEntry struct {
	Labels    map[string]string `json:"labels"`
	Timestamp string            `json:"timestamp"`
}

// LokiPushRequest represents the request body for Loki push API
// Also used for tail API response which may include dropped_entries
type LokiPushRequest struct {
	Streams       []LokiStream    `json:"streams"`
	DroppedEntries []DroppedEntry `json:"dropped_entries,omitempty"`
}

// LokiQueryResponse represents the response from a Loki query
type LokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []LokiStream `json:"result"`
	} `json:"data"`
}

// QueryRangeParams holds the parameters for a Loki query_range request.
type QueryRangeParams struct {
	Query     string
	Limit     int
	Start     time.Time
	End       time.Time
	Since     time.Duration
	Step      time.Duration
	Interval  time.Duration
	Direction string
}
