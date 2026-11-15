package decisions

import (
	"encoding/json"
	"fmt"
)

// patchDecisionRawModel rewrites the model field of the client's raw body
// before forwarding: the body is decoded into raw JSON values (so every
// field — including unknown extensions and the exact value bytes inside each
// field — survives), the model is replaced with the resolved upstream name,
// and the object is re-serialized.
//
// The raw-body guarantee is semantic, not byte identity: key order and
// top-level whitespace may change, and duplicate keys collapse to the last
// value (duplicate keys violate the RFC's SHOULD-unique rule). Byte-for-byte
// forwarding is reserved for the no-override fast path, which never calls
// this function. Numeric precision is preserved because values stay
// json.RawMessage.
func patchDecisionRawModel(rawBody []byte, modelName string) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse decisions request body: %w", err)
	}
	modelJSON, err := json.Marshal(modelName)
	if err != nil {
		return nil, fmt.Errorf("marshal model name: %w", err)
	}
	payload["model"] = modelJSON
	patched, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal decisions request body: %w", err)
	}
	return patched, nil
}
