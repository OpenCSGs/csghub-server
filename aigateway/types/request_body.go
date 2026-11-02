package types

import (
	"bytes"
	"io"
)

// requestBody owns immutable, already-marshaled bytes. A metadata observer may
// borrow the unread slice; the proxy still reads the same bytes afterwards.
type requestBody struct {
	*bytes.Reader
	raw []byte
}

// NewRequestBody takes ownership of raw; callers must not mutate it afterwards.
func NewRequestBody(raw []byte) io.ReadCloser {
	return &requestBody{Reader: bytes.NewReader(raw), raw: raw}
}
func (b *requestBody) Close() error { return nil }

// ReadRequestBody borrows an owned body or reads a generic body. Callers must
// install a fresh reader before forwarding, since the generic fallback consumes it.
func ReadRequestBody(body io.ReadCloser) ([]byte, error) {
	if owned, ok := body.(*requestBody); ok {
		start := len(owned.raw) - owned.Len()
		return owned.raw[start:], nil
	}
	return io.ReadAll(body)
}
