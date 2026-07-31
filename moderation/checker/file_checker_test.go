package checker

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	mocksens "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

func TestGetFileChecker(t *testing.T) {
	// Test case 1: Test with a folder
	fileType1 := "folder"
	filePath1 := "/path/to/folder"
	lfsRelativePath1 := ""
	expected1 := &FolderChecker{}
	result1 := GetFileChecker(fileType1, filePath1, lfsRelativePath1)
	if _, ok := result1.(*FolderChecker); !ok {
		t.Errorf("Expected %T, got %T", expected1, result1)
	}

	// Test case 2: Test with a LFS file
	fileType2 := "file"
	filePath2 := "/path/to/lfs/file"
	lfsRelativePath2 := "lfs/path/to/file"
	expected2 := &LfsFileChecker{}
	result2 := GetFileChecker(fileType2, filePath2, lfsRelativePath2)
	if _, ok := result2.(*LfsFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected2, result2)
	}

	// Test case 3: Test with an unknown file type
	fileType3 := "file"
	filePath3 := "/path/to/unknown/file"
	lfsRelativePath3 := ""
	expected3 := &UnkownFileChecker{}
	result3 := GetFileChecker(fileType3, filePath3, lfsRelativePath3)
	if _, ok := result3.(*UnkownFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected3, result3)
	}

	// Test case 4: Test with an image file
	fileType4 := "file"
	filePath4 := "/path/to/image.png"
	lfsRelativePath4 := ""
	expected4 := &ImageFileChecker{}
	result4 := GetFileChecker(fileType4, filePath4, lfsRelativePath4)
	if _, ok := result4.(*ImageFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected4, result4)
	}

	// Test case 5: Test with a text file
	fileType5 := "file"
	filePath5 := "/path/to/text.md"
	lfsRelativePath5 := ""
	expected5 := &TextFileChecker{}
	result5 := GetFileChecker(fileType5, filePath5, lfsRelativePath5)
	if _, ok := result5.(*TextFileChecker); !ok {
		t.Errorf("Expected %T, got %T", expected5, result5)
	}
}

func TestImageFileChecker_Run(t *testing.T) {
	const testImageURL = "http://example.com/images/test.png"

	// newEnabledChecker creates an ImageFileChecker with the given enable flag
	// and mock sensitive checker, avoiding global state mutation.
	newEnabledChecker := func(enabled bool, mockChecker sensitive.SensitiveChecker) *ImageFileChecker {
		return &ImageFileChecker{checker: mockChecker, enabled: enabled, scenario: types.ScenarioImageBaseLineCheck}
	}

	t.Run("image check disabled should skip", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		c := newEnabledChecker(false, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader(""), ImageURL: testImageURL})
		if status != types.SensitiveCheckSkip {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckSkip, status)
		}
		if message != "skip image file, image check is not enabled" {
			t.Errorf("Expected message '%s', got '%s'", "skip image file, image check is not enabled", message)
		}
	})

	t.Run("sensitive image detected", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).
			Return(&sensitive.CheckResult{IsSensitive: true, Reason: "label:porn,confidence:0.95"}, nil)

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader(""), ImageURL: testImageURL})
		if status != types.SensitiveCheckFail {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckFail, status)
		}
		if message != "label:porn,confidence:0.95" {
			t.Errorf("Expected message '%s', got '%s'", "label:porn,confidence:0.95", message)
		}
	})

	t.Run("image passes check", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil)

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader(""), ImageURL: testImageURL})
		if status != types.SensitiveCheckPass {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckPass, status)
		}
		if message != "" {
			t.Errorf("Expected empty message, got '%s'", message)
		}
	})

	t.Run("empty image url with reader falls back to stream check", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil)

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader("image-bytes")})
		if status != types.SensitiveCheckPass {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckPass, status)
		}
		if message != "" {
			t.Errorf("Expected empty message, got '%s'", message)
		}
	})

	t.Run("empty image url without reader returns exception", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{})
		if status != types.SensitiveCheckException {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckException, status)
		}
		if message != "image url is empty and no stream available" {
			t.Errorf("Expected message '%s', got '%s'", "image url is empty and no stream available", message)
		}
	})

	t.Run("checker error with retry", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).Once().
			Return(nil, errors.New("network error"))
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).Once().
			Return(nil, errors.New("network error"))
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).Once().
			Return(&sensitive.CheckResult{IsSensitive: false}, nil)

		c := newEnabledChecker(true, mockChecker)
		status, _ := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader(""), ImageURL: testImageURL})
		if status != types.SensitiveCheckPass {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckPass, status)
		}
	})

	t.Run("all retries failed", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).Times(3).
			Return(nil, errors.New("service unavailable"))

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader(""), ImageURL: testImageURL})
		if status != types.SensitiveCheckException {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckException, status)
		}
		if message != "call sensitive image url checker api failed" {
			t.Errorf("Expected message '%s', got '%s'", "call sensitive image url checker api failed", message)
		}
	})

	t.Run("context canceled", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		// mock a slow call that will be interrupted by context cancellation
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testImageURL).
			RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(5 * time.Second):
					return &sensitive.CheckResult{IsSensitive: false}, nil
				}
			})

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(ctx, FileCheckContext{Reader: strings.NewReader(""), ImageURL: testImageURL})
		if status != types.SensitiveCheckException {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckException, status)
		}
		if message != "call sensitive image url checker api failed" {
			t.Errorf("Expected message '%s', got '%s'", "call sensitive image url checker api failed", message)
		}
	})

	t.Run("stream check sensitive image detected", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&sensitive.CheckResult{IsSensitive: true, Reason: "label:porn,confidence:0.95"}, nil)

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader("image-bytes")})
		if status != types.SensitiveCheckFail {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckFail, status)
		}
		if message != "label:porn,confidence:0.95" {
			t.Errorf("Expected message '%s', got '%s'", "label:porn,confidence:0.95", message)
		}
	})

	t.Run("stream check all retries failed", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).Times(3).
			Return(nil, errors.New("service unavailable"))

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader("image-bytes")})
		if status != types.SensitiveCheckException {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckException, status)
		}
		if message != "call sensitive image stream checker api failed" {
			t.Errorf("Expected message '%s', got '%s'", "call sensitive image stream checker api failed", message)
		}
	})

	// "stream check retry receives identical content" verifies that retries
	// work correctly when the reader is seekable. checkByStream only retries
	// when the reader implements io.ReadSeeker; the actual rewind before each
	// attempt is handled by chainImpl.PassImageStreamCheck (which seeks before
	// each checker). This mock simulates that behavior by seeking the reader
	// to the start before reading on each attempt. The first attempt fails and
	// the second succeeds; both must receive the exact same bytes.
	t.Run("stream check retry receives identical content", func(t *testing.T) {
		imageContent := "the-quick-brown-fox"
		mockChecker := mocksens.NewMockSensitiveChecker(t)

		// Track the content read on each call.
		var contents []string
		var mu sync.Mutex
		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, r io.Reader) (*sensitive.CheckResult, error) {
				// Simulate chain layer: seek to start before reading.
				if seeker, ok := r.(io.Seeker); ok {
					_, _ = seeker.Seek(0, io.SeekStart)
				}
				b, _ := io.ReadAll(r)
				mu.Lock()
				contents = append(contents, string(b))
				mu.Unlock()
				// First attempt fails, forcing a retry.
				if len(contents) == 1 {
					return nil, errors.New("transient upload error")
				}
				return &sensitive.CheckResult{IsSensitive: false}, nil
			}).Twice()

		c := newEnabledChecker(true, mockChecker)
		status, message := c.Run(context.Background(), FileCheckContext{Reader: strings.NewReader(imageContent)})
		if status != types.SensitiveCheckPass {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckPass, status)
		}
		if message != "" {
			t.Errorf("Expected empty message, got '%s'", message)
		}

		mu.Lock()
		defer mu.Unlock()
		if len(contents) != 2 {
			t.Fatalf("expected 2 stream check attempts, got %d", len(contents))
		}
		for i, got := range contents {
			if got != imageContent {
				t.Errorf("attempt %d: expected content %q, got %q", i, imageContent, got)
			}
		}
	})

	// "stream check non-seekable reader returns exception" verifies that
	// when the reader does not implement io.ReadSeeker, Run rejects it
	// immediately without calling the sensitive checker, because retries
	// require a seekable reader to rewind the stream.
	t.Run("stream check non-seekable reader returns exception", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)

		c := newEnabledChecker(true, mockChecker)
		// Wrap a seekable reader to hide Seek, making it non-seekable.
		nonSeekable := struct{ io.Reader }{strings.NewReader("image-bytes")}
		status, message := c.Run(context.Background(), FileCheckContext{Reader: nonSeekable})
		if status != types.SensitiveCheckException {
			t.Errorf("Expected status %v, got %v", types.SensitiveCheckException, status)
		}
		if message != "image stream reader is not seekable, cannot retry on failure" {
			t.Errorf("Expected message '%s', got '%s'", "image stream reader is not seekable, cannot retry on failure", message)
		}
		// The sensitive checker should never be called.
		mockChecker.AssertNotCalled(t, "PassImageStreamCheck")
	})
}

func TestLfsFileChecker_Run(t *testing.T) {
	c := &LfsFileChecker{}
	reader := strings.NewReader("lfs content")
	expectedStatus := types.SensitiveCheckSkip
	expectedMessage := "skip lfs file"
	status, message := c.Run(context.Background(), FileCheckContext{Reader: reader})
	if status != expectedStatus || message != expectedMessage {
		t.Errorf("Expected (%v, %v), got (%v, %v)", expectedStatus, expectedMessage, status, message)
	}
}

func TestFolderChecker_Run(t *testing.T) {
	c := &FolderChecker{}
	reader := strings.NewReader("folder content")
	expectedStatus := types.SensitiveCheckSkip
	expectedMessage := "skip folder"
	status, message := c.Run(context.Background(), FileCheckContext{Reader: reader})
	if status != expectedStatus || message != expectedMessage {
		t.Errorf("Expected (%v, %v), got (%v, %v)", expectedStatus, expectedMessage, status, message)
	}
}
