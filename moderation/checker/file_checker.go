package checker

import (
	"context"
	"io"
	"path"
	"slices"
	"strings"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

var knownImageFileExts = []string{".png", ".jpg", ".jpeg", ".gif", ".tif", ".tiff", ".svg", ".bmp", ".webp"}
var knownTextFileExts = []string{".md", ".txt", ".csv", ".json", ".jsonl", ".html",
	//code file types
	".cs", ".js", ".ts", ".py", ".php", ".java", ".c", ".cpp", ".go", ".rb", ".sh"}

type FileChecker interface {
	Run(ctx context.Context, reader io.Reader) (types.SensitiveCheckStatus, string)
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

	if slices.ContainsFunc(knownImageFileExts, func(imageExt string) bool {
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

type ImageFileChecker struct {
	checker sensitive.SensitiveChecker
}

func NewImageFileChecker() FileChecker {
	return &ImageFileChecker{
		checker: contentChecker,
	}
}
func (c *ImageFileChecker) Run(context.Context, io.Reader) (types.SensitiveCheckStatus, string) {
	//TODO:check image in the future
	return types.SensitiveCheckSkip, "skip image file"
}

type LfsFileChecker struct {
}

func (c *LfsFileChecker) Run(context.Context, io.Reader) (types.SensitiveCheckStatus, string) {
	// dont need to check lfs file content
	return types.SensitiveCheckSkip, "skip lfs file"
}

type FolderChecker struct {
}

func (c *FolderChecker) Run(ctx context.Context, reader io.Reader) (types.SensitiveCheckStatus, string) {
	return types.SensitiveCheckSkip, "skip folder"
}
