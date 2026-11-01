package types

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/common/errorx"
)

// ParseGuestIDHeader validates the client-generated anonymous guest ID from the
// X-CSG-Guest-ID header. Only UUIDv4 values are accepted.
func ParseGuestIDHeader(raw string) (string, error) {
	guestID := strings.TrimSpace(raw)
	if guestID == "" {
		return "", errorx.ErrBadRequest
	}
	parsed, err := uuid.Parse(guestID)
	if err != nil || parsed.Version() != 4 {
		return "", errorx.ErrBadRequest
	}
	return parsed.String(), nil
}

// DeriveGuestActorID builds the stable guest actor ID for a logical agent
// instance. The digest format must stay identical across all callers so that
// pre-warm and chat hit the same allocation.
func DeriveGuestActorID(guestID string, logicalID int64) string {
	value := fmt.Sprintf("logical-agent:%d:guest:%s", logicalID, guestID)
	digest := sha256.Sum256([]byte(value))
	return "guest-" + base64.RawURLEncoding.EncodeToString(digest[:])
}

// SharedSandboxProxyRequest carries the anonymous request attributes needed to
// enforce per-guest chat limits on shared runtime agents.
type SharedSandboxProxyRequest struct {
	Method  string
	APIPath string
	GuestID string
}
