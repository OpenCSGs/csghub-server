package compress

import (
	"bytes"
	"testing"
)

func TestZstdEncodeDecode_RoundTrip(t *testing.T) {
	original := []byte(`{"request_id":"req-1","model_id":"model-a","prompt":"hello world"}`)

	encoded := ZstdEncode(original)
	if bytes.Equal(encoded, original) {
		t.Fatal("ZstdEncode() returned the input unchanged")
	}

	decoded, err := ZstdDecode(encoded)
	if err != nil {
		t.Fatalf("ZstdDecode() error = %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("ZstdDecode(ZstdEncode(x)) = %q, want %q", decoded, original)
	}
}

func TestIsZstdEncoded(t *testing.T) {
	encoded := ZstdEncode([]byte(`{"request_id":"req-1"}`))

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "encoded payload", data: encoded, want: true},
		{name: "raw json", data: []byte(`{"request_id":"req-1"}`), want: false},
		{name: "empty slice", data: []byte{}, want: false},
		{name: "shorter than magic", data: []byte{0x28, 0xB5, 0x2F}, want: false},
		{name: "magic plus garbage", data: append([]byte{0x28, 0xB5, 0x2F, 0xFD}, []byte("garbage")...), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsZstdEncoded(tt.data); got != tt.want {
				t.Errorf("IsZstdEncoded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestZstdDecode_Corrupted(t *testing.T) {
	data := append([]byte{0x28, 0xB5, 0x2F, 0xFD}, []byte("garbage")...)
	if _, err := ZstdDecode(data); err == nil {
		t.Fatal("ZstdDecode() expected error for corrupted payload, got nil")
	}
}
