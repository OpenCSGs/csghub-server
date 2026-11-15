package checker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"opencsg.com/csghub-server/common/types"
	ss_type "opencsg.com/csghub-server/common/types/sensitive"
)

var knownTextFileExts = []string{".md", ".txt", ".csv", ".json", ".jsonl", ".html",
	//code file types
	".cs", ".js", ".ts", ".py", ".php", ".java", ".c", ".cpp", ".go", ".rb", ".sh"}

// FileCheckContext carries the file content reader and metadata needed by file checkers.
type FileCheckContext struct {
	Reader   io.Reader
	ImageURL string // publicly-accessible URL for image files; empty for non-image files
}

type FileChecker interface {
	Run(ctx context.Context, fctx FileCheckContext) (types.SensitiveCheckStatus, string)
}

// GetFileChecker returns a FileChecker for a given file based on its type and path.
//
// The checkers are chosen as follows:
// - folder: FolderChecker
// - LFS files: LfsFileChecker
// - unknown files: UnkownFileChecker
// - image files (with extensions .png, .jpg, .jpeg, .gif, .tif, .tiff, .svg, .bmp, .webp): ImageFileChecker
// - text files (with extensions .md, .txt, .csv, .json, .jsonl, .html, .cs, .js, .ts, .py, .php, .java, .c, .cpp, .go, .rb, .sh): TextFileChecker
func GetFileChecker(fileType string, filePath, lfsRelativePath string) FileChecker {

	if fileType == "folder" {
		return &FolderChecker{}
	}

	if lfsRelativePath != "" {
		return &LfsFileChecker{}
	}

	ext := path.Ext(filePath)
	if len(ext) == 0 {
		return &UnkownFileChecker{}
	}

	if slices.ContainsFunc(types.KnownImageFileExts, func(imageExt string) bool {
		return strings.EqualFold(ext, imageExt)
	}) {
		return NewImageFileChecker()
	}

	if slices.ContainsFunc(knownTextFileExts, func(textExt string) bool {
		return strings.EqualFold(ext, textExt)
	}) {
		return NewTextFileChecker()
	}

	return &UnkownFileChecker{}
}

// ImageFileChecker checks image files for sensitive content by delegating to a
// SensitiveChecker. The checker needs a publicly-accessible image URL so that
// the remote moderation service can fetch the image.
//
// The enable flag and scenario are injected at construction time to avoid
// global state and ensure test isolation.
type ImageFileChecker struct {
	checker  ss_type.SensitiveChecker
	enabled  bool
	scenario types.SensitiveScenario
}

func NewImageFileChecker() FileChecker {
	return &ImageFileChecker{
		checker:  contentChecker,
		enabled:  imageCheckEnabled,
		scenario: imageCheckScenario,
	}
}

func (c *ImageFileChecker) Run(ctx context.Context, fctx FileCheckContext) (types.SensitiveCheckStatus, string) {
	if !c.enabled {
		return types.SensitiveCheckSkip, "skip image file, image check is not enabled"
	}

	// For public repos, ImageURL is available — check by URL.
	// For private repos, ImageURL is empty — check by uploading the stream to S3.
	if fctx.ImageURL != "" {
		return c.checkByURL(ctx, fctx.ImageURL)
	}
	if fctx.Reader == nil {
		return types.SensitiveCheckException, "image url is empty and no stream available"
	}
	// The stream-based check uploads the image to OSS and retries on failure.
	// Retries require rewinding the reader, so only seekable readers are accepted.
	if _, seekable := fctx.Reader.(io.ReadSeeker); !seekable {
		return types.SensitiveCheckException, "image stream reader is not seekable, cannot retry on failure"
	}
	return c.checkByStream(ctx, fctx.Reader)
}

// checkByURL checks an image via its publicly-accessible URL.
func (c *ImageFileChecker) checkByURL(ctx context.Context, imageURL string) (types.SensitiveCheckStatus, string) {
	// Each attempt gets a 10s timeout. With 3 attempts and backoff delay,
	// worst case is ~30s per file. This is acceptable because CheckRepoFiles
	// is a Temporal activity with no hard 30s limit, and concurrency is
	// bounded by concurrencyLimit in the repo component.
	res, err := retry.DoWithData(
		func() (*ss_type.CheckResult, error) {
			reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return c.checker.PassImageURLCheck(reqCtx, c.scenario, imageURL)
		}, retry.Attempts(3), retry.DelayType(retry.BackOffDelay), retry.LastErrorOnly(true),
		retry.RetryIf(func(error) bool {
			// stop retrying once the parent context is cancelled.
			// All other errors (including Aliyun 4xx) are retried because
			// the Aliyun SDK does not expose a structured error type that
			// reliably distinguishes transient vs permanent failures.
			return ctx.Err() == nil
		}))

	if err != nil {
		slog.ErrorContext(ctx, "failed to check image content by url", slog.String("imageURL", imageURL), slog.Any("error", err))
		return types.SensitiveCheckException, "call sensitive image url checker api failed"
	}

	if res.IsSensitive {
		slog.InfoContext(ctx, "url image content is sensitive", slog.String("imageURL", imageURL), slog.String("reason", res.Reason))
		return types.SensitiveCheckFail, res.Reason
	}

	return types.SensitiveCheckPass, ""
}

// checkByStream checks an image by uploading its stream to S3 and generating
// a presigned URL. Used for private repos where no public URL is available.
//
// Each attempt rewinds the reader, probes one byte to reject empty images
// without a remote call, rewinds again, and then uploads the full stream. The
// probe lives inside the retry loop so a transient Git read failure is retried
// instead of being recorded as a permanent exception. Retries are only enabled
// when the reader is seekable (io.ReadSeeker), because a failed attempt
// consumes the stream and retries need a rewind to upload the full content.
func (c *ImageFileChecker) checkByStream(ctx context.Context, reader io.Reader) (types.SensitiveCheckStatus, string) {
	seeker, seekable := reader.(io.ReadSeeker)

	type attemptResult struct {
		check *ss_type.CheckResult
		empty bool
	}

	res, err := retry.DoWithData(
		func() (attemptResult, error) {
			if seekable {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return attemptResult{}, err
				}
			}
			// Decide emptiness from the content that will actually be checked.
			// The persisted file size comes from an earlier tree snapshot and
			// may not match the current default branch, so it must not be used
			// to skip the check. A zero-byte image would otherwise be uploaded
			// as an empty object and rejected by the remote service.
			var probe [1]byte
			n, err := reader.Read(probe[:])
			if err != nil && !errors.Is(err, io.EOF) {
				return attemptResult{}, err
			}
			if n == 0 && errors.Is(err, io.EOF) {
				return attemptResult{empty: true}, nil
			}
			if seekable {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return attemptResult{}, err
				}
			}
			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			check, err := c.checker.PassImageStreamCheck(reqCtx, c.scenario, reader)
			return attemptResult{check: check}, err
		}, retry.Attempts(3), retry.DelayType(retry.BackOffDelay), retry.LastErrorOnly(true),
		retry.RetryIf(func(error) bool {
			// Only retry when the reader is seekable; otherwise the stream
			// has been consumed and retries would upload empty content.
			return seekable && ctx.Err() == nil
		}))

	if err != nil {
		slog.ErrorContext(ctx, "failed to check image content by stream", slog.Any("error", err))
		return types.SensitiveCheckException, "call sensitive image stream checker api failed"
	}

	if res.empty {
		return types.SensitiveCheckPass, "empty file"
	}

	if res.check != nil && res.check.IsSensitive {
		slog.InfoContext(ctx, "stream image content is sensitive", slog.String("reason", res.check.Reason))
		return types.SensitiveCheckFail, res.check.Reason
	}

	return types.SensitiveCheckPass, ""
}

type LfsFileChecker struct {
}

func (c *LfsFileChecker) Run(ctx context.Context, fctx FileCheckContext) (types.SensitiveCheckStatus, string) {
	// dont need to check lfs file content
	return types.SensitiveCheckSkip, "skip lfs file"
}

type FolderChecker struct {
}

func (c *FolderChecker) Run(ctx context.Context, fctx FileCheckContext) (types.SensitiveCheckStatus, string) {
	return types.SensitiveCheckSkip, "skip folder"
}
