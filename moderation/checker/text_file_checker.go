package checker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/avast/retry-go/v4"
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

func (c *TextFileChecker) Run(ctx context.Context, reader io.Reader) (types.SensitiveCheckStatus, string) {
	type result struct {
		status  types.SensitiveCheckStatus
		message string
	}

	resultCh := make(chan result, 1)

	go func() {
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
				resultCh <- result{types.SensitiveCheckException, "failed to read file content"}
				return
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
			var res *sensitive.CheckResult
			var err error
			slog.Debug("check text", slog.String("scenario", string(types.ScenarioCommentDetection)), slog.String("text", buf.String()))
			txt := buf.String()
			//call remote checker
			res, err = retry.DoWithData(
				func() (*sensitive.CheckResult, error) {
					reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					res, err = c.PassTextCheck(reqCtx, types.ScenarioCommentDetection, txt)
					cancel()
					if err != nil {
						return nil, err
					}
					return res, nil
				}, retry.Attempts(3), retry.DelayType(retry.BackOffDelay), retry.LastErrorOnly(true))

			if err != nil {
				resultCh <- result{types.SensitiveCheckException, "call sensitive checker api failed"}
				return
			}

			if res.IsSensitive {
				resultCh <- result{types.SensitiveCheckFail, res.Reason}
				return
			}
		}

		resultCh <- result{types.SensitiveCheckPass, ""}
	}()

	select {
	case <-ctx.Done():
		return types.SensitiveCheckException, "context canceled"
	case res := <-resultCh:
		return res.status, res.message
	}
}
