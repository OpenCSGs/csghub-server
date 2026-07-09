package checker

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
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
		png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))

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
		png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))

		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: &pngBuf})
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
		png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1)))

		mockChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			Return(&sensitive.CheckResult{IsSensitive: true, Reason: "label:porn"}, nil)

		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), FileCheckContext{Reader: &pngBuf})
		require.Equal(t, types.SensitiveCheckFail, status)
		require.Equal(t, "label:porn", msg)
	})
}
