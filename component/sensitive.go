package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/markdownparser"
)

const (
	checkTimeoutSecond = 3
)

type sensitiveComponentImpl struct {
	checker        rpc.ModerationSvcClient
	markDownParser markdownparser.OSSParserComponent
	maxImageCount  int
}

type SensitiveComponent interface {
	CheckText(ctx context.Context, scenario, text string) (bool, error)
	CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error)
	CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error)
	CheckMarkdownContent(ctx context.Context, content string) (bool, error)
}

func NewSensitiveComponent(cfg *config.Config) (SensitiveComponent, error) {
	if !cfg.SensitiveCheck.Enable {
		return &sensitiveComponentNoOpImpl{}, nil
	}

	c := &sensitiveComponentImpl{
		markDownParser: markdownparser.NewOSSParserComponent(),
		maxImageCount:  cfg.SensitiveCheck.MaxImageCount,
	}
	c.checker = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", cfg.Moderation.Host, cfg.Moderation.Port))
	return c, nil
}

func (c sensitiveComponentImpl) CheckText(ctx context.Context, scenario, text string) (bool, error) {
	result, err := c.checker.PassTextCheck(ctx, scenario, text)
	if err != nil {
		return false, err
	}

	return !result.IsSensitive, nil
}

func (c sensitiveComponentImpl) CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error) {
	result, err := c.checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		return false, err
	}
	return !result.IsSensitive, nil
}

func (c sensitiveComponentImpl) CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error) {
	fields := req.GetSensitiveFields()
	for _, field := range fields {
		if len(field.Value()) == 0 {
			continue
		}
		result, err := c.checker.PassTextCheck(ctx, field.Scenario, field.Value())
		if err != nil {
			slog.Error("fail to check request sensitivity", slog.String("field", field.Name), slog.Any("error", err))
			return false, fmt.Errorf("fail to check '%s' sensitivity, error: %w", field.Name, err)
		}
		if result.IsSensitive {
			slog.Error("found sensitive words in request", slog.String("field", field.Name))
			return false, errors.New("found sensitive words in field: " + field.Name)
		}
	}
	return true, nil
}

// runSensitiveCheckInGoroutine runs a sensitive check function in a goroutine
// checkFunc should return (bool, error) where bool indicates if content is safe (true) or sensitive (false)
func (c sensitiveComponentImpl) runCheckInGoroutine(ctx context.Context, wg *sync.WaitGroup, errChan chan<- error, checkFunc func(context.Context) (bool, error)) {
	go func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				slog.Error("panic in sensitive check", slog.Any("panic", r))
				errChan <- fmt.Errorf("panic in sensitive check: %v", r)
			}
		}()

		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		default:
			isPass, err := checkFunc(ctx)
			if err != nil {
				errChan <- err
				return
			}
			if !isPass {
				errChan <- errors.New("found sensitive content")
				return
			}
		}
	}()
}

// CheckMarkdownContent concurrently checks markdown content for sensitive text and images.
func (c sensitiveComponentImpl) CheckMarkdownContent(ctx context.Context, content string) (bool, error) {
	// Create a context with checkTimeoutSecond seconds timeout
	ctx, cancel := context.WithTimeout(ctx, checkTimeoutSecond*time.Second)
	defer cancel()

	// Default OSS bucket name for image checking
	parseResult, err := c.markDownParser.ParseMarkdownAndFilter(content)
	if err != nil {
		return false, err
	}

	// Limit the number of pictures to prevent malicious large-scale data requests from consuming resources
	if len(parseResult.OssImageInfoList) > c.maxImageCount {
		return false, fmt.Errorf("too many images: %d, maximum allowed: %d", len(parseResult.OssImageInfoList), c.maxImageCount)
	}

	// Text only
	if parseResult.Text != "" && len(parseResult.OssImageInfoList) == 0 {
		isPass, err := c.CheckText(ctx, string(sensitive.ScenarioCommentDetection), parseResult.Text)
		if err != nil {
			return false, err
		}
		if !isPass {
			slog.Error("found sensitive words in request", slog.String("field", parseResult.Text))
			return false, errors.New("found sensitive words in field: " + parseResult.Text)
		}
		return isPass, nil
	}

	// Only one image
	if parseResult.Text == "" && len(parseResult.OssImageInfoList) == 1 {
		isPass, err := c.CheckImage(
			ctx,
			string(sensitive.ScenarioImagePostImageCheck),
			parseResult.OssImageInfoList[0].BucketName,
			parseResult.OssImageInfoList[0].ObjectName)
		if err != nil {
			return false, err
		}
		if !isPass {
			slog.Error("found sensitive iamges in request", slog.String("field", parseResult.OssImageInfoList[0].ObjectName))
			return false, errors.New("found sensitive iamges in field: " + parseResult.OssImageInfoList[0].ObjectName)
		}
		return isPass, nil
	}

	// Check multi content
	var wg sync.WaitGroup
	numWorkers := 0
	if parseResult.Text != "" {
		numWorkers += 1
	}
	numWorkers += len(parseResult.OssImageInfoList)
	errChan := make(chan error, numWorkers)

	// Check text content
	if parseResult.Text != "" {
		wg.Add(1)
		c.runCheckInGoroutine(ctx, &wg, errChan, func(ctx context.Context) (bool, error) {
			return c.CheckText(ctx, string(sensitive.ScenarioCommentDetection), parseResult.Text)
		})
	}

	// Check image URLs
	for _, ossImageInfo := range parseResult.OssImageInfoList {
		wg.Add(1)
		c.runCheckInGoroutine(ctx, &wg, errChan, func(ctx context.Context) (bool, error) {
			return c.CheckImage(ctx, string(sensitive.ScenarioImagePostImageCheck), ossImageInfo.BucketName, ossImageInfo.ObjectName)
		})
	}

	// Wait for all checks to complete or context to be cancelled
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		// Context cancelled (e.g., timeout)
		errChan <- ctx.Err()
	}

	close(errChan)

	// Collect errors
	var collectErrors []error
	for err := range errChan {
		if err != nil {
			collectErrors = append(collectErrors, err)
		}
	}

	if len(collectErrors) > 0 {
		return false, fmt.Errorf("sensitive content check failed: %v", collectErrors)
	}

	return true, nil
}

// sensitiveComponentNoOpImpl this implementation provides a "no-op" (no operation) version of the SensitiveComponent interface,
// where all methods simply return a "not sensitive" result without performing any actual checks.
type sensitiveComponentNoOpImpl struct {
}

func (c *sensitiveComponentNoOpImpl) CheckText(ctx context.Context, scenario, text string) (bool, error) {
	return true, nil
}

func (c *sensitiveComponentNoOpImpl) CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error) {
	return true, nil
}

// implements SensitiveComponent
func (c *sensitiveComponentNoOpImpl) CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error) {
	return true, nil
}

func (c *sensitiveComponentNoOpImpl) CheckMarkdownContent(ctx context.Context, content string) (bool, error) {
	return true, nil
}
