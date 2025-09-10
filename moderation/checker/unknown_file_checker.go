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

func (c *UnkownFileChecker) Run(ctx context.Context, reader io.Reader) (types.SensitiveCheckStatus, string) {
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
		return tc.Run(ctx, mreader)
	case strings.HasPrefix(detectedType, "image"):
		slog.Debug("use image file checker for unknown file", slog.String("content_type", detectedType))
		ic := NewImageFileChecker()
		mreader := io.MultiReader(bytes.NewReader(buffer), reader)
		return ic.Run(ctx, mreader)
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
