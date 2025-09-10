package loki

import "time"

// LokiStream represents a Loki log stream
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// LokiPushRequest represents the request body for Loki push API
type LokiPushRequest struct {
	Streams []LokiStream `json:"streams"`
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
