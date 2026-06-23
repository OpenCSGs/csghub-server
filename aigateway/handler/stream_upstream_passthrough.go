package handler

import "net/http"

func isUpstreamHTTPError(statusCode int) bool {
	return statusCode >= http.StatusBadRequest
}

func shouldPassthroughUpstreamError(statusCode int, sseStarted bool) bool {
	return isUpstreamHTTPError(statusCode) && !sseStarted
}
