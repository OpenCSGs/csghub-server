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

func TestSafeContainerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal case",
			input:    "my-container",
			expected: "my-container",
		},
		{
			name:     "dots replaced with dash",
			input:    "my.service.v2",
			expected: "my-service-v2",
		},
		{
			name:     "version with dots",
			input:    "svc-qwen-qwen3-0.6b-pd-disaggregation-epp",
			expected: "svc-qwen-qwen3-0-6b-pd-disaggregation-epp",
		},
		{
			name:     "upper case to lower case",
			input:    "MyContainer",
			expected: "mycontainer",
		},
		{
			name:     "replace invalid chars with dash",
			input:    "my_container@123",
			expected: "my-container-123",
		},
		{
			name:     "trim non-alphanumeric chars from edges",
			input:    "-my-container-",
			expected: "my-container",
		},
		{
			name:     "empty string returns default",
			input:    "",
			expected: "default",
		},
		{
			name:     "only dots returns default",
			input:    "...",
			expected: "default",
		},
		{
			name:     "long string truncated to 63",
			input:    strings.Repeat("a", 100),
			expected: strings.Repeat("a", 63),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeContainerName(tt.input)
			assert.Equal(t, tt.expected, result)
			// Container names must never contain dots
			assert.NotContains(t, result, ".")
		})
	}
}
