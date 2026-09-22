package aliyunchecker

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "opencsg.com/csghub-server/plugins/aliyun_checker/v1"
)

func TestStreamReaderConcatenatesChunks(t *testing.T) {
	reader := &streamReader{stream: &fakeImageServerStream{frames: []*v1.ImageStreamFrame{
		{Frame: &v1.ImageStreamFrame_Chunk{Chunk: []byte("ab")}},
		{Frame: &v1.ImageStreamFrame_Chunk{Chunk: []byte("cd")}},
	}}}

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "abcd", string(content))
}
