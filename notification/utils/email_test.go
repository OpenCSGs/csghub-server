package utils

import (
	"testing"
)

func TestExtractDisplayNameFromEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{
			name:     "simple email with lowercase",
			email:    "community@opencsg.com",
			expected: "Community",
		},
		{
			name:     "email with uppercase",
			email:    "Contact@opencsg.com",
			expected: "Contact",
		},
		{
			name:     "invalid email",
			email:    "invalid",
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractDisplayNameFromEmail(tt.email)
			if result != tt.expected {
				t.Errorf("ExtractDisplayNameFromEmail(%q) = %q, want %q", tt.email, result, tt.expected)
			}
		})
	}
}
