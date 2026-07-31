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
	mocksens "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestUnkownFileChecker_Run(t *testing.T) {
	t.Run("fail to read file", func(t *testing.T) {
		reader := mockio.NewMockReader(t)
		reader.EXPECT().Read(mock.Anything).Return(0, errors.New("unknown exception"))
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: reader})
		require.Equal(t, types.SensitiveCheckException, status)
		require.Equal(t, "failed to read file contents", msg)
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

	t.Run("image detected with URL uses URL check", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		InitWithContentChecker(cfg, mockChecker)

		// Build a minimal PNG so http.DetectContentType returns "image/png"
		var pngBuf bytes.Buffer
		err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))
		require.NoError(t, err)

		const testURL = "http://example.com/image.png"
		mockChecker.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioImageBaseLineCheck, testURL).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: &pngBuf, ImageURL: testURL})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)
	})

	t.Run("image detected without URL falls back to stream check", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		InitWithContentChecker(cfg, mockChecker)

		var pngBuf bytes.Buffer
		err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))
		require.NoError(t, err)

		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil)

		// bytes.Reader implements io.ReadSeeker so rewindReader can seek it back.
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(pngBuf.Bytes())})
		require.Equal(t, types.SensitiveCheckPass, status)
		require.Empty(t, msg)
	})

	t.Run("image detected without URL sensitive content detected", func(t *testing.T) {
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		InitWithContentChecker(cfg, mockChecker)

		var pngBuf bytes.Buffer
		err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))
		require.NoError(t, err)

		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&sensitive.CheckResult{IsSensitive: true, Reason: "label:porn"}, nil)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: bytes.NewReader(pngBuf.Bytes())})
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
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		InitWithContentChecker(cfg, mockChecker)

		var pngBuf bytes.Buffer
		err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))
		require.NoError(t, err)
		pngData := pngBuf.Bytes()

		var readContent []byte
		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, r io.Reader) (*sensitive.CheckResult, error) {
				readContent, _ = io.ReadAll(r)
				return &sensitive.CheckResult{IsSensitive: false}, nil
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
		mockChecker := mocksens.NewMockSensitiveChecker(t)
		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		InitWithContentChecker(cfg, mockChecker)

		var pngBuf bytes.Buffer
		err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))
		require.NoError(t, err)
		pngData := pngBuf.Bytes()

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
