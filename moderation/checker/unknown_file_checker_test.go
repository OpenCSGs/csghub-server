package checker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockio "opencsg.com/csghub-server/_mocks/io"
	"opencsg.com/csghub-server/common/types"
)

func TestUnkownFileChecker_Run(t *testing.T) {
	t.Run("fail to read file", func(t *testing.T) {
		reader := mockio.NewMockReader(t)
		reader.EXPECT().Read(mock.Anything).Return(0, errors.New("unknown exception"))
		c := &UnkownFileChecker{}
		status, msg := c.Run(context.Background(), reader)
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
		status, _ := c.Run(context.Background(), reader)
		require.Equal(t, types.SensitiveCheckSkip, status)
	})
}
