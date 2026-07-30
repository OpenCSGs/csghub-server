package checker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

// UnkownFileChecker handles the unknown file types (no file extension)
//
// Internally, it will read the first 512 bytes and detect the content type
// and use the corresponding checker
type UnkownFileChecker struct {
}

func (c *UnkownFileChecker) Run(ctx context.Context, fctx FileCheckContext) (types.SensitiveCheckStatus, string) {
	reader := fctx.Reader
	// read the first 512 bytes and detect the content type
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil {
		return types.SensitiveCheckException, "failed to read file contents"
	}

	// remove zero bytes before detecting content type,
	// see: https://gist.github.com/rayrutjes/db9b9ea8e02255d62ce2?permalink_comment_id=3418419#gistcomment-3418419
	buffer = buffer[:n]
	// Detect the file content type like text/plain, image/jpeg, etc
	detectedType := http.DetectContentType(buffer)

	switch {
	case strings.HasPrefix(detectedType, "text"):
		slog.Debug("use text file checker for unknown file", slog.String("content_type", detectedType))
		tc := NewTextFileChecker()
		mreader := io.MultiReader(bytes.NewReader(buffer), reader)
		return tc.Run(ctx, FileCheckContext{Reader: mreader})
	case strings.HasPrefix(detectedType, "image"):
		slog.Debug("use image file checker for unknown file", slog.String("content_type", detectedType))
		ic := NewImageFileChecker()
		// When a URL is available, the Aliyun service fetches the image directly,
		// so the reader is not needed. Only rewind the reader for stream-based
		// checks (no URL). If the reader is not seekable, rewindReader returns
		// nil — short-circuit with an exception instead of passing an
		// un-rewindable reader downstream.
		if fctx.ImageURL != "" {
			return ic.Run(ctx, FileCheckContext{ImageURL: fctx.ImageURL})
		}
		rewoundReader := rewindReader(reader, buffer)
		if rewoundReader == nil {
			return types.SensitiveCheckException, "image stream reader is not seekable, cannot retry on failure"
		}
		return ic.Run(ctx, FileCheckContext{Reader: rewoundReader})
	case strings.HasPrefix(detectedType, "audio"):
		slog.Debug("skip audio checker for unknown file", slog.String("content_type", detectedType))
		return types.SensitiveCheckSkip, "skip binary audio file"
	case strings.HasPrefix(detectedType, "video"):
		slog.Debug("skip video checker for unknown file", slog.String("content_type", detectedType))
		return types.SensitiveCheckSkip, "skip binary video file"
	default:
		slog.Debug("skip binary checker for unknown file", slog.String("content_type", detectedType))
		return types.SensitiveCheckSkip, "skip binary file"
	}

}

// rewindReader resets the reader to the beginning after 512 bytes have been
// consumed for content-type detection. If the reader implements io.Seeker,
// it is seeked back to position 0 and returned directly — this preserves
// seekability for downstream retry/chain logic. If the reader is not seekable,
// nil is returned so the caller can short-circuit instead of passing an
// un-rewindable reader downstream.
func rewindReader(reader io.Reader, consumed []byte) io.Reader {
	if seeker, ok := reader.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			slog.Error("failed to seek reader back to start", slog.Any("error", err))
			return nil
		}
		return reader
	}
	return nil
}
