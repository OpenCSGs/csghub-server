package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
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
