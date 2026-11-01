package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLfsPartSize(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured int
		size, want int64
	}{
		{"ordinary", 100, 200 * 1024 * 1024, 104857600},
		{"below threshold", 100, 1000*104857600 - 1, 104857600},
		{"at threshold", 100, 1000 * 104857600, 104857600},
		{"above threshold", 100, 1000*104857600 + 1, 104857601},
		{"large", 100, 100 * 1024 * 1024 * 1024, 107374183},
		{"minimum", 1, 2 * 1024 * 1024 * 1024, 5 * 1024 * 1024},
		{"zero config", 0, 1024, 0},
		{"negative config", -1, 1024, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			partSize := LfsPartSize(tc.configured, tc.size)
			require.Equal(t, tc.want, partSize)
			if partSize > 0 && tc.size > 0 {
				require.LessOrEqual(t, (tc.size-1)/partSize+1, int64(1000))
			}
		})
	}
}
