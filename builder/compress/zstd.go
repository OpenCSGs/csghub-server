package compress

import (
	"bytes"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

var zstdMagic = []byte{0x28, 0xB5, 0x2F, 0xFD}

var (
	// EncodeAll/DecodeAll on a shared encoder/decoder are safe for concurrent use.
	zstdEncoder = mustZstdEncoder()
	zstdDecoder = mustZstdDecoder()
)

// mustZstdEncoder fails fast at package init: if encoder construction ever
// fails (e.g. an invalid option is added later), a nil encoder would panic on
// first use, which is harder to diagnose than panicking here.
func mustZstdEncoder() *zstd.Encoder {
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		panic(fmt.Sprintf("init zstd encoder: %v", err))
	}
	return encoder
}

func mustZstdDecoder() *zstd.Decoder {
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(64<<20))
	if err != nil {
		panic(fmt.Sprintf("init zstd decoder: %v", err))
	}
	return decoder
}

// ZstdEncode compresses data with zstd. EncodeAll on an initialized writer has
// no error path, so no error is returned.
func ZstdEncode(data []byte) []byte {
	return zstdEncoder.EncodeAll(data, nil)
}

// ZstdDecode decompresses zstd-encoded data, bounded by the 64MB decoder limit.
func ZstdDecode(data []byte) ([]byte, error) {
	return zstdDecoder.DecodeAll(data, nil)
}

// IsZstdEncoded reports whether data starts with the zstd frame magic number.
func IsZstdEncoded(data []byte) bool {
	return len(data) >= len(zstdMagic) && bytes.Equal(data[:len(zstdMagic)], zstdMagic)
}
