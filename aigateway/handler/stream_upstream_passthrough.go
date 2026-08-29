package handler

import "net/http"

func isUpstreamHTTPError(statusCode int) bool {
	return statusCode >= http.StatusBadRequest
}

func isSuccessfulStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func shouldPassthroughUpstreamError(statusCode int, sseStarted bool) bool {
	return isUpstreamHTTPError(statusCode) && !sseStarted
}
