package common

import (
	"testing"
)

func TestValidateURLFormat(t *testing.T) {
	tests := []struct {
		name        string
		urlString   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid http url",
			urlString:   "http://example.com",
			expectError: false,
		},
		{
			name:        "valid https url",
			urlString:   "https://example.com",
			expectError: false,
		},
		{
			name:        "valid url with path",
			urlString:   "https://example.com/path",
			expectError: false,
		},
		{
			name:        "valid url with query parameters",
			urlString:   "https://example.com/path?param=value",
			expectError: false,
		},
		{
			name:        "valid url with fragment",
			urlString:   "https://example.com/path#fragment",
			expectError: false,
		},
		{
			name:        "valid url with port",
			urlString:   "https://example.com:8080",
			expectError: false,
		},
		{
			name:        "valid ftp url",
			urlString:   "ftp://ftp.example.com",
			expectError: false,
		},
		{
			name:        "valid ssh url",
			urlString:   "ssh://user@example.com",
			expectError: false,
		},
		{
			name:        "valid git url",
			urlString:   "git://github.com/user/repo.git",
			expectError: false,
		},
		{
			name:        "empty url",
			urlString:   "",
			expectError: true,
			errorMsg:    "url is empty",
		},
		{
			name:        "url without scheme",
			urlString:   "example.com",
			expectError: true,
			errorMsg:    "url must have a scheme",
		},
		{
			name:        "url with empty scheme",
			urlString:   "://example.com",
			expectError: true,
		},
		{
			name:        "url without host",
			urlString:   "http://",
			expectError: true,
			errorMsg:    "url must have a host",
		},
		{
			name:        "url with empty host",
			urlString:   "http:///path",
			expectError: true,
			errorMsg:    "url must have a host",
		},
		{
			name:        "malformed url with spaces",
			urlString:   "http://example com",
			expectError: true,
		},
		{
			name:        "url with only scheme",
			urlString:   "http:",
			expectError: true,
			errorMsg:    "url must have a host",
		},
		{
			name:        "url with scheme and colon only",
			urlString:   "http:",
			expectError: true,
			errorMsg:    "url must have a host",
		},
		{
			name:        "url with scheme and double slash but no host",
			urlString:   "http://",
			expectError: true,
			errorMsg:    "url must have a host",
		},
		{
			name:        "url with scheme and path but no host",
			urlString:   "http:///path",
			expectError: true,
			errorMsg:    "url must have a host",
		},
		{
			name:        "valid url with ip address",
			urlString:   "http://192.168.1.1",
			expectError: false,
		},
		{
			name:        "valid url with ip address and port",
			urlString:   "http://192.168.1.1:8080",
			expectError: false,
		},
		{
			name:        "valid url with localhost",
			urlString:   "http://localhost:3000",
			expectError: false,
		},
		{
			name:        "valid url with subdomain",
			urlString:   "https://api.example.com",
			expectError: false,
		},
		{
			name:        "valid url with multiple subdomains",
			urlString:   "https://www.api.example.com",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURLFormat(tt.urlString)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateURLFormat() expected error but got none")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("ValidateURLFormat() error = %v, want error containing %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateURLFormat() unexpected error = %v", err)
				}
			}
		})
	}
}
