package utils_test

import (
	"testing"

	"opencsg.com/csghub-server/notification/utils"
)

// TestIsStringInArray tests the IsStringInArray function
func TestIsStringInArray(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		arr      []string
		expected bool
	}{
		{
			name:     "string found in array",
			str:      "apple",
			arr:      []string{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "string not found in array",
			str:      "date",
			arr:      []string{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "empty array",
			str:      "apple",
			arr:      []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.IsStringInArray(tt.str, tt.arr)
			if result != tt.expected {
				t.Errorf("IsStringInArray(%q, %v) = %v; want %v", tt.str, tt.arr, result, tt.expected)
			}
		})
	}
}
