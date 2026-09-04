package anthropic

import (
	"crypto/rand"
	"encoding/hex"
)

// randomHexID generates a short random hex string for response IDs.
func randomHexID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
