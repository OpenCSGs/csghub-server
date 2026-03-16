package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal case",
			input:    "my-service",
			expected: "my-service",
		},
		{
			name:     "upper case to lower case",
			input:    "MyService",
			expected: "myservice",
		},
		{
			name:     "replace invalid chars with dash",
			input:    "my_service@123",
			expected: "my-service-123",
		},
		{
			name:     "trim non-alphanumeric chars from edges",
			input:    "-my-service-",
			expected: "my-service",
		},
		{
			name:     "dots are allowed",
			input:    "my.service",
			expected: "my.service",
		},
		{
			name:     "complex case",
			input:    "  My_Service@123..  ",
			expected: "my-service-123",
		},
		{
			name:     "empty string returns default",
			input:    "",
			expected: "default",
		},
		{
			name:     "only invalid chars returns default",
			input:    "___",
			expected: "default",
		},
		{
			name:     "long string truncated",
			input:    strings.Repeat("a", 300),
			expected: strings.Repeat("a", 253),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
