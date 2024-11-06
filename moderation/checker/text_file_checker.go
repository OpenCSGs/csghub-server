package checker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

type TextFileChecker struct {
	sensitive.SensitiveChecker
}

func NewTextFileChecker() *TextFileChecker {
	return &TextFileChecker{
		contentChecker,
	}
}

func (c *TextFileChecker) Run(reader io.Reader) (types.SensitiveCheckStatus, string) {
	//at most 1MB
	reader = io.LimitReader(reader, 1024*1024)
	const blockSize = 10 * 9000
	// const blockSize = 3000
	var bufs []bytes.Buffer
	for {
		buf := bytes.Buffer{}
		var err error
		var avaliableSize int64
		if avaliableSize, err = io.CopyN(&buf, reader, blockSize); err != nil && err != io.EOF {
			return types.SensitiveCheckException, "failed to read file content"
		}
		if avaliableSize > 0 {
			bufs = append(bufs, buf)
		}
		//no more data to read
		if avaliableSize < blockSize {
			break
		}
	}
	for _, buf := range bufs {
		var result *sensitive.CheckResult
		var err error
		slog.Debug("check text", slog.String("scenario", string(sensitive.ScenarioCommentDetection)), slog.String("text", buf.String()))
		//do local check first
		txt := buf.String()
		contains := localWordChecker.ContainsSensitiveWord(txt)
		if contains {
			return types.SensitiveCheckFail, "contains sensitive word"
		}
		//call remote checker
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		result, err = c.PassTextCheck(ctx, sensitive.ScenarioCommentDetection, txt)
		cancel()
		if err != nil {
			return types.SensitiveCheckException, "call sensitive checker api failed"
		}

		if result.IsSensitive {
			return types.SensitiveCheckFail, result.Reason
		}
	}

	return types.SensitiveCheckPass, ""
}
