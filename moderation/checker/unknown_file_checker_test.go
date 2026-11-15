package checker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockio "opencsg.com/csghub-server/_mocks/io"
	mocksens "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/common/types/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	ss_type "opencsg.com/csghub-server/common/types/sensitive"
)

// TestUnkownFileChecker_Run covers how the 512-byte content-type probe treats
// read results. Image classification is covered separately in
// TestUnkownFileChecker_ImageDetection.
func TestUnkownFileChecker_Run(t *testing.T) {
	t.Run("fail to read file", func(t *testing.T) {
		reader := mockio.NewMockReader(t)
		reader.EXPECT().Read(mock.Anything).Return(0, errors.New("unknown exception"))
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: reader})
		require.Equal(t, types.SensitiveCheckException, status)
		require.Equal(t, "failed to read file contents", msg)
	})

	// An empty file returns (0, io.EOF) on the first read. That is not a read
	// failure: the checker returns Pass without invoking a content checker.
	t.Run("empty file passes", func(t *testing.T) {
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(nil)})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Equal(t, "empty file", msg)
	})

	// A (0, nil) read is a legal transient no-data read, not proof of an empty
	// file. The checker must keep reading and inspect the content that follows.
	t.Run("zero-byte non-EOF read is not treated as empty", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, false)
		checkedText := expectPassTextCheck(t, mockChecker)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: &zeroThenDataReader{data: []byte("hello")}})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)
		require.Equal(t, "hello", *checkedText)
	})

	// io.Reader allows returning the final bytes together with io.EOF in one
	// call. Those bytes must still be inspected instead of being dropped.
	t.Run("short file returning io.EOF is checked", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, false)
		checkedText := expectPassTextCheck(t, mockChecker)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: &eofWithDataReader{data: []byte("hello")}})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)
		require.Equal(t, "hello", *checkedText)
	})

	t.Run("skip audio file", func(t *testing.T) {
		reader := mockio.NewMockReader(t)
		reader.EXPECT().Read(mock.Anything).RunAndReturn(func(b []byte) (int, error) {
			header := []byte{0x46, 0x4F, 0x52, 0x4D, 0x00, 0x00, 0x00, 0x00, 0x41, 0x49, 0x46, 0x46}
			copy(b, header)
			return len(header), nil

		})
		c := &UnkownFileChecker{}
		status, _ := c.Run(context.Background(), FileCheckContext{Reader: reader})
		require.Equal(t, types.SensitiveCheckSkip, status)
	})
}

func TestUnkownFileChecker_ImageDetection(t *testing.T) {
	t.Run("image detected with URL uses URL check", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, true)
		pngData := newPNGData(t)

		const testURL = "http://example.com/image.png"
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testURL).
			Return(&ss_type.CheckResult{IsSensitive: false}, nil)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(pngData), ImageURL: testURL})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)
	})

	t.Run("image detected without URL falls back to stream check", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, true)

		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&ss_type.CheckResult{IsSensitive: false}, nil)

		// bytes.Reader implements io.ReadSeeker so rewindReader can seek it back.
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(newPNGData(t))})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)
	})

	t.Run("image detected without URL sensitive content detected", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, true)

		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&ss_type.CheckResult{IsSensitive: true, Reason: "label:porn"}, nil)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(newPNGData(t))})
		require.Equal(t, types.SensitiveCheckFail, status)
		require.Equal(t, "label:porn", msg)
	})

	// "image detected with seekable reader rewinds and passes full content"
	// verifies that when the reader is seekable (e.g. RepoFileContentReader),
	// UnkownFileChecker seeks it back to the start after detecting content type,
	// so the downstream ImageFileChecker receives the full file content from
	// the beginning. The mock reads the stream and verifies it starts with the
	// PNG magic header (proving the reader was rewound past the 512-byte
	// detection read).
	t.Run("image detected with seekable reader rewinds and passes full content", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, true)
		pngData := newPNGData(t)

		var readContent []byte
		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
				readContent, _ = io.ReadAll(r)
				return &ss_type.CheckResult{IsSensitive: false}, nil
			}).Once()

		// bytes.Reader implements io.ReadSeeker
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(pngData)})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)

		// The content read by the checker must be the full PNG data, including
		// the magic header that was consumed during detection.
		require.Equal(t, pngData, readContent, "downstream checker should receive full content from start")
	})

	// "image detected with non-seekable reader returns exception"
	// verifies that when the reader is NOT seekable, rewindReader returns nil
	// and ImageFileChecker.Run returns an exception instead of passing an
	// un-rewindable reader downstream.
	t.Run("image detected with non-seekable reader returns exception", func(t *testing.T) {
		mockChecker := initUnknownCheckerTest(t, true)
		pngData := newPNGData(t)

		// Wrap in a struct that only exposes Read (not Seek)
		nonSeekable := struct{ io.Reader }{bytes.NewReader(pngData)}
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: nonSeekable})
		require.Equal(t, types.SensitiveCheckException, status)
		require.Equal(t, "image stream reader is not seekable, cannot retry on failure", msg)

		// The sensitive checker should never be called.
		mockChecker.AssertNotCalled(t, "PassImageStreamCheck")
	})
}

// initUnknownCheckerTest installs a strict mock as the package content checker
// and returns it. Any checker call without an explicit expectation fails the
// test, which keeps "must not call the remote service" assertions honest.
func initUnknownCheckerTest(t *testing.T, imageCheckEnabled bool) *mocksens.MockSensitiveChecker {
	t.Helper()

	mockChecker := mocksens.NewMockSensitiveChecker(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.ImageCheckEnable = imageCheckEnabled
	InitWithContentChecker(cfg, mockChecker)
	return mockChecker
}

// expectPassTextCheck expects one text check that returns "not sensitive" and
// returns a pointer to the text that was checked.
func expectPassTextCheck(t *testing.T, mockChecker *mocksens.MockSensitiveChecker) *string {
	t.Helper()

	checkedText := new(string)
	mockChecker.EXPECT().PassTextCheck(mock.Anything, types.ScenarioCommentDetection, mock.Anything).
		RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, text string) (*ss_type.CheckResult, error) {
			*checkedText = text
			return &ss_type.CheckResult{IsSensitive: false}, nil
		})
	return checkedText
}

// newPNGData returns a minimal PNG so http.DetectContentType classifies it as
// image/png.
func newPNGData(t *testing.T) []byte {
	t.Helper()

	var pngBuf bytes.Buffer
	require.NoError(t, png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	return pngBuf.Bytes()
}

// eofWithDataReader returns its data together with io.EOF on the first read,
// which io.Reader explicitly allows for the final bytes of a stream.
type eofWithDataReader struct {
	data []byte
	done bool
}

func (r *eofWithDataReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), io.EOF
}

// zeroThenDataReader returns a legal (0, nil) read before its data, modelling
// a transient no-data read that must not be treated as an empty file.
type zeroThenDataReader struct {
	data []byte
	step int
}

func (r *zeroThenDataReader) Read(p []byte) (int, error) {
	switch r.step {
	case 0:
		r.step++
		return 0, nil
	case 1:
		r.step++
		return copy(p, r.data), nil
	default:
		return 0, io.EOF
	}
}
