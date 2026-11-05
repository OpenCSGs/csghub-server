package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLicenseProduct(t *testing.T) {
	product, ok := ParseLicenseProduct("csglite")
	require.True(t, ok)
	require.Equal(t, LicenseProductCSGLite, product)
	for _, unsupported := range []string{"", "csgclaw", "CSGLite", "4"} {
		_, ok := ParseLicenseProduct(unsupported)
		require.False(t, ok, unsupported)
	}
}
