package geo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxmindIPLocator_V2_GetIPLocation(t *testing.T) {
	db, err := tryOpenEmbedGeoDB()
	assert.NoError(t, err)
	locator := &maxmindIPLocatorV2{
		db: db,
	}
	t.Run("found china", func(t *testing.T) {
		ip := "39.144.179.123"
		location, err := locator.GetIPLocation(ip)
		assert.NoError(t, err)
		assert.NotNil(t, location)
		assert.Equal(t, "China", location.Nation)
		assert.Equal(t, "Henan", location.Province)
		assert.Equal(t, "Zhumadian", location.City)
	})

	t.Run("found hk", func(t *testing.T) {
		ip := "8.218.255.255"
		location, err := locator.GetIPLocation(ip)
		assert.NoError(t, err)
		assert.NotNil(t, location)
		assert.Equal(t, "Hong Kong", location.Nation)
		assert.Equal(t, "", location.Province)
		assert.Equal(t, "Hong Kong", location.City)
	})

	t.Run("found singapore", func(t *testing.T) {
		ip := "8.34.202.0"
		location, err := locator.GetIPLocation(ip)
		assert.NoError(t, err)
		assert.NotNil(t, location)
		assert.Equal(t, "Singapore", location.Nation)
		assert.Equal(t, "", location.Province)
		assert.Equal(t, "Singapore", location.City)
	})

	t.Run("not found", func(t *testing.T) {
		ip := "127.0.0.1"
		location, err := locator.GetIPLocation(ip)
		assert.NoError(t, err)
		assert.Empty(t, location)
	})

	t.Run("invalid ip", func(t *testing.T) {
		ip := "invalid-ip"
		location, err := locator.GetIPLocation(ip)
		assert.Error(t, err)
		assert.Nil(t, location)
	})
}
