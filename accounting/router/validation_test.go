package router

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidation_OptionalDateFormat(t *testing.T) {
	validate := validator.New()

	err := validate.RegisterValidation("optional_date_format", OptionalDateFormat)
	require.Nil(t, err)

	tests := []struct {
		dateStr string
		valid   bool
	}{
		{"2023-10-05", true},
		{"", true},
		{"2023-13-01", false},
		{"2023-10-32", false},
		{"invalid-date", false},
	}

	for _, tt := range tests {
		t.Run(tt.dateStr, func(t *testing.T) {
			err := validate.Var(tt.dateStr, "optional_date_format")
			if tt.valid {
				require.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidation_OptionalYearAndMonthFormat(t *testing.T) {
	validate := validator.New()

	err := validate.RegisterValidation("year_month_format", OptionalYearMonthFormat)
	require.Nil(t, err)

	tests := []struct {
		dateStr string
		valid   bool
	}{
		{"2023-01", true},
		{"2023-1", false},
		{"2023-10", true},
		{"invalid-date", false},
	}

	for _, tt := range tests {
		t.Run(tt.dateStr, func(t *testing.T) {
			err := validate.Var(tt.dateStr, "year_month_format")
			if tt.valid {
				require.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
