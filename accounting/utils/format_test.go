package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func TestFormat_ValidateDateTimeFormat(t *testing.T) {
	timeStr := "2024-11-08 12:13:23"
	layout := "2006-01-02 15:04:05"

	res := ValidateDateTimeFormat(timeStr, layout)

	require.True(t, res)

	timeStr = "2024-11-08"

	res = ValidateDateTimeFormat(timeStr, layout)

	require.False(t, res)
}

func TestFormat_ValidateQueryDate(t *testing.T) {
	layout := "2006-01-02 15:04:05"
	startDateStr := "2024-11-08 12:13:23"
	endDateStr := "2024-11-09 12:13:23"
	start, end, err := ValidateQueryDate(startDateStr, endDateStr, layout)
	require.Nil(t, err)
	require.Equal(t, startDateStr, start)
	require.Equal(t, "2024-11-10 12:13:23", end)
}

func TestFormat_ConvertToGlobalTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	config.SetGlobalTimeZone(loc)
	t.Cleanup(func() { config.SetGlobalTimeZone(nil) })

	got, err := ConvertToGlobalTimeZone("2026-08-01 08:00:00", "2006-01-02 15:04:05")
	require.NoError(t, err)
	require.Equal(t, "2026-08-01 08:00:00+08:00", got)

	_, err = ConvertToGlobalTimeZone("invalid", "2006-01-02 15:04:05")
	require.Error(t, err)
}

func TestFormat_ConvertDateRangeToGlobalTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	config.SetGlobalTimeZone(loc)
	t.Cleanup(func() { config.SetGlobalTimeZone(nil) })

	start, end, err := ConvertDateRangeToGlobalTimeZone("2026-08-01", "2026-08-02", "2006-01-02")
	require.NoError(t, err)
	require.Equal(t, "2026-08-01 00:00:00+08:00", start)
	require.Equal(t, "2026-08-03 00:00:00+08:00", end)

	_, _, err = ConvertDateRangeToGlobalTimeZone("invalid", "2026-08-02", "2006-01-02")
	require.Error(t, err)
}
