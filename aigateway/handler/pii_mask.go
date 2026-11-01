package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// PIICheckHeader opts a single request into PII detection. Detection may
// rewrite the prompt before it reaches the upstream model (mask/skip today,
// block later), so it never happens without the client asking for it; the
// deployment-level switch (config.AIGateway.PIICheckEnabled) gates whether
// the header is honored at all.
const PIICheckHeader = "X-PII-Check"

// piiCheckRequested reports whether the client explicitly opted in.
func piiCheckRequested(c *gin.Context) bool {
	value := strings.TrimSpace(c.GetHeader(PIICheckHeader))
	return strings.EqualFold(value, "enable") || strings.EqualFold(value, "true")
}
