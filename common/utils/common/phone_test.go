package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserComponent_GetCountryCode(t *testing.T) {
	t.Run("test get country code for china", func(t *testing.T) {
		countryCode, err := GetCountryCodeByPhoneArea("12345678901", "+86")
		require.Nil(t, err)
		require.Equal(t, "CN", countryCode)
	})
	t.Run("test get country code for us", func(t *testing.T) {
		countryCode, err := GetCountryCodeByPhoneArea("4155552671", "+1")
		require.Nil(t, err)
		require.Equal(t, "US", countryCode)
	})
	t.Run("test get country code for hongkong", func(t *testing.T) {
		countryCode, err := GetCountryCodeByPhoneArea("66668877", "+852")
		require.Nil(t, err)
		require.Equal(t, "HK", countryCode)
	})
	t.Run("test get country code for invalid phone area", func(t *testing.T) {
		countryCode, err := GetCountryCodeByPhoneArea("12345678901", "+11")
		require.NotNil(t, err)
		require.Equal(t, "", countryCode)
	})
}

func TestUserComponent_GetPhoneAreaByCountryCode(t *testing.T) {
	t.Run("test get phone area by country code for china", func(t *testing.T) {
		phoneArea, err := GetPhoneAreaByCountryCode("12345678901", "CN")
		require.Nil(t, err)
		require.Equal(t, "+86", phoneArea)
	})
	t.Run("test get phone area by country code for us", func(t *testing.T) {
		phoneArea, err := GetPhoneAreaByCountryCode("4155552671", "US")
		require.Nil(t, err)
		require.Equal(t, "+1", phoneArea)
	})
	t.Run("test get phone area by country code for hongkong", func(t *testing.T) {
		phoneArea, err := GetPhoneAreaByCountryCode("66668877", "HK")
		require.Nil(t, err)
		require.Equal(t, "+852", phoneArea)
	})
	t.Run("test get phone area by country code for invalid phone area", func(t *testing.T) {
		phoneArea, err := GetPhoneAreaByCountryCode("12345678901", "OZ")
		require.NotNil(t, err)
		require.Equal(t, "", phoneArea)
	})
}

func TestUserComponent_IsValidNumber(t *testing.T) {
	t.Run("test is valid number for china", func(t *testing.T) {
		isValid, err := IsValidNumber("13626487789", "+86")
		require.Nil(t, err)
		require.True(t, isValid)
	})
	t.Run("test is invalid number for hongkong", func(t *testing.T) {
		isValid, err := IsValidNumber("13626487789", "+852")
		require.Nil(t, err)
		require.False(t, isValid)
	})
}

func TestNormalizePhoneArea(t *testing.T) {
	t.Run("test normalize phone area without plus prefix", func(t *testing.T) {
		result := NormalizePhoneArea("86")
		require.Equal(t, "+86", result)
	})
	t.Run("test normalize phone area with plus prefix", func(t *testing.T) {
		result := NormalizePhoneArea("+86")
		require.Equal(t, "+86", result)
	})
}
