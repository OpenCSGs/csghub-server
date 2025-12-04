package compress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecode(t *testing.T) {
	testData := []byte("this is a test string for compress")
	encTypes := []string{"gzip", "deflate", "br", "unknown"}

	for _, encType := range encTypes {
		t.Run(encType, func(t *testing.T) {
			encodedData, err := Encode(encType, testData)
			assert.NoError(t, err)

			decodedData, err := Decode(encType, encodedData)
			assert.NoError(t, err)
			assert.Equal(t, testData, decodedData)
		})
	}
}
