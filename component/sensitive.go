package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	maxImageCount      = 10
	checkTimeoutSecond = 3
)

type sensitiveComponentImpl struct {
	checker rpc.ModerationSvcClient
}

type SensitiveComponent interface {
	CheckText(ctx context.Context, scenario, text string) (bool, error)
	CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error)
	CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error)
	CheckMarkdownContent(ctx context.Context, content, ossBucketName string) (bool, error)
}

func NewSensitiveComponent(cfg *config.Config) (SensitiveComponent, error) {
	if !cfg.SensitiveCheck.Enable {
		return &sensitiveComponentNoOpImpl{}, nil
	}

	c := &sensitiveComponentImpl{}
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
func (c sensitiveComponentImpl) CheckMarkdownContent(ctx context.Context, content, ossBucketName string) (bool, error) {
	// Create a context with checkTimeoutSecond seconds timeout
	ctx, cancel := context.WithTimeout(ctx, checkTimeoutSecond*time.Second)
	defer cancel()

	// Default OSS bucket name for image checking
	text, imageURLs := parseMarkdownContent(content)

	// Limit the number of pictures to prevent malicious large-scale data requests from consuming resources
	if len(imageURLs) > maxImageCount {
		return false, fmt.Errorf("too many images: %d, maximum allowed: %d", len(imageURLs), maxImageCount)
	}

	// Text only
	if text != "" && len(imageURLs) == 0 {
		isPass, err := c.CheckText(ctx, string(sensitive.ScenarioCommentDetection), text)
		if err != nil {
			return false, err
		}
		if !isPass {
			slog.Error("found sensitive words in request", slog.String("field", text))
			return false, errors.New("found sensitive words in field: " + text)
		}
		return isPass, nil
	}

	// Only one image
	if text == "" && len(imageURLs) == 1 {
		isPass, err := c.CheckImage(ctx, string(sensitive.ScenarioImageCommentCheck), ossBucketName, imageURLs[0])
		if err != nil {
			return false, err
		}
		if !isPass {
			slog.Error("found sensitive iamges in request", slog.String("field", imageURLs[0]))
			return false, errors.New("found sensitive iamges in field: " + text)
		}
		return isPass, nil
	}

	// Check multi content
	var wg sync.WaitGroup
	errChan := make(chan error, 1+len(imageURLs))

	// Check text content
	if text != "" {
		wg.Add(1)
		c.runCheckInGoroutine(ctx, &wg, errChan, func(ctx context.Context) (bool, error) {
			return c.CheckText(ctx, string(sensitive.ScenarioCommentDetection), text)
		})
	}

	// Check image URLs
	for _, url := range imageURLs {
		wg.Add(1)
		c.runCheckInGoroutine(ctx, &wg, errChan, func(ctx context.Context) (bool, error) {
			return c.CheckImage(ctx, string(sensitive.ScenarioImageCommentCheck), ossBucketName, url)
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

func (c *sensitiveComponentNoOpImpl) CheckMarkdownContent(ctx context.Context, content, ossBucketName string) (bool, error) {
	// Create a context with 3 seconds timeout for consistency
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return true, nil
}

// parseMarkdownContent parses markdown content to extract text and image URLs.
func parseMarkdownContent(content string) (string, []string) {
	// Regex to find markdown image syntax: ![alt text](image_url)
	re := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(content, -1)

	var imageURLs []string
	for _, match := range matches {
		if len(match) > 1 {
			imageURLs = append(imageURLs, match[1])
		}
	}

	// Remove image markdown from content to get pure text
	text := re.ReplaceAllString(content, "")

	return text, imageURLs
}
