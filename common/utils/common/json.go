package common

import (
	"bytes"
	"encoding/json"
)

// MarshalJSONWithoutHTMLEscape marshals v like encoding/json.Marshal but does not escape &, <, > in strings.
// Presigned URLs and other strings with ampersands stay readable in JSON (no \u0026).
func MarshalJSONWithoutHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
