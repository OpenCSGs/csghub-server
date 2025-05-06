package token_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	mock_token "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestTokenizer_EmbeddingEncode(t *testing.T) {
	tests := []struct {
		name    string
		message types.Message
		want    int64
		err     error
	}{
		{
			name: "empty message",
			message: types.Message{
				Content: "",
			},
			want: 0,
			err:  fmt.Errorf("EmbeddingTokenize input cannot be empty"),
		},
		{
			name: "simple message",
			message: types.Message{
				Content: "test",
			},
			want: 3,
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := mock_token.NewMockTokenizer(t)
			d.EXPECT().Encode(tt.message).Return(tt.want, tt.err)
			got, err := d.Encode(tt.message)
			if err != nil {
				assert.Equal(t, err, fmt.Errorf("EmbeddingTokenize input cannot be empty"))
			}
			if got != tt.want {
				t.Errorf("DumyTokenizer.Encode() = %v, want %v", got, tt.want)
			}
		})
	}
}
