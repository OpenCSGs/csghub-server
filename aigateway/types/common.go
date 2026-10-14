package types

import (
	"net/http"

	commontypes "opencsg.com/csghub-server/common/types"
)

func ApplyRequestAuthHeaders(header http.Header, authHeadStr string) error {
	header.Del("Authorization")
	if authHeadStr == "" {
		return nil
	}
	authMap, err := commontypes.ParseAuthHeaders(authHeadStr)
	if err != nil {
		return err
	}
	for authKey, authVal := range authMap {
		header.Set(authKey, authVal)
	}
	return nil
}

// ShouldAttemptFailureStatus evaluates fail status codes to see if AIGateway should retry.
// - 499 is a client-closed connection and should not be retried.
// - 400 indicates a client argument error and should not be retried.
// Returns true for other status codes greater than 400.
func ShouldAttemptFailureStatus(statusCode int) bool {
	return statusCode > http.StatusBadRequest && statusCode != 499
}

// IsCircuitNeutralStatus reports whether an upstream HTTP status carries no
// signal about upstream availability, so it must not be recorded by the
// circuit breaker in either direction. 429 means the upstream responded but
// is throttling, and 499 means the client gave up before the upstream
// finished. Neither may count as a failure that opens the circuit, nor as a
// success that could spuriously close a half-open circuit.
func IsCircuitNeutralStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode == 499
}
