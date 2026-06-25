package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateHeader(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		value       string
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid header",
			key:   "X-API-Key",
			value: "secret",
		},
		{
			name:        "invalid header name",
			key:         "Bad Header",
			value:       "secret",
			expectError: true,
			errorMsg:    "invalid header name",
		},
		{
			name:        "invalid header value",
			key:         "X-API-Key",
			value:       "bad\r\nvalue",
			expectError: true,
			errorMsg:    "invalid header value",
		},
		{
			name:        "unsafe header",
			key:         "Host",
			value:       "example.com",
			expectError: true,
			errorMsg:    "unsafe header",
		},
		{
			name:        "unsafe header with whitespace",
			key:         " Transfer-Encoding ",
			value:       "chunked",
			expectError: true,
			errorMsg:    "unsafe header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeader(tt.key, tt.value)
			if tt.expectError {
				if err == nil {
					t.Fatalf("ValidateHeader() expected error but got none")
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Fatalf("ValidateHeader() error = %q, want %q", err.Error(), tt.errorMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateHeader() unexpected error = %v", err)
			}
		})
	}
}

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

func TestValidateImageURL(t *testing.T) {
	t.Run("format validation without server", func(t *testing.T) {
		tests := []struct {
			name        string
			urlString   string
			expectError bool
			errorMsg    string
		}{
			{
				name:        "empty url",
				urlString:   "",
				expectError: true,
				errorMsg:    "url is empty",
			},
			{
				name:        "non-http scheme",
				urlString:   "ftp://example.com/avatar.jpg",
				expectError: true,
				errorMsg:    "url scheme must be http or https",
			},
			{
				name:        "url without scheme",
				urlString:   "//example.com/avatar.jpg",
				expectError: true,
				errorMsg:    "url scheme must be http or https",
			},
			{
				name:        "relative url - logout attack",
				urlString:   "/logout",
				expectError: true,
				errorMsg:    "url scheme must be http or https",
			},
			{
				name:        "relative path with extension",
				urlString:   "/images/avatar.jpg",
				expectError: true,
				errorMsg:    "url scheme must be http or https",
			},
			{
				name:        "url without host",
				urlString:   "https:///avatar.jpg",
				expectError: true,
				errorMsg:    "url must have a host",
			},
			{
				name:        "malformed url",
				urlString:   "https://ex ample.com/avatar.jpg",
				expectError: true,
			},
			{
				name:        "private IP - 127.0.0.1",
				urlString:   "http://127.0.0.1/avatar.jpg",
				expectError: true,
				errorMsg:    "image url must not be a private or internal IP address",
			},
			{
				name:        "private IP - 10.0.0.1",
				urlString:   "https://10.0.0.1/avatar.jpg",
				expectError: true,
				errorMsg:    "image url must not be a private or internal IP address",
			},
			{
				name:        "private IP - 192.168.1.1",
				urlString:   "https://192.168.1.1/avatar.png",
				expectError: true,
				errorMsg:    "image url must not be a private or internal IP address",
			},
			{
				name:        "link-local IP - 169.254.169.254",
				urlString:   "http://169.254.169.254/latest/meta-data/",
				expectError: true,
				errorMsg:    "image url must not be a private or internal IP address",
			},
			{
				name:        "loopback IPv6",
				urlString:   "http://[::1]/avatar.jpg",
				expectError: true,
				errorMsg:    "image url must not be a private or internal IP address",
			},
			{
				name:        "blocked port 22",
				urlString:   "http://203.0.113.1:22/avatar.jpg",
				expectError: true,
				errorMsg:    "image url port must be 80 or 443, got 22",
			},
			{
				name:        "blocked port 6379",
				urlString:   "http://203.0.113.1:6379/avatar.jpg",
				expectError: true,
				errorMsg:    "image url port must be 80 or 443, got 6379",
			},
			{
				name:        "blocked port 5432",
				urlString:   "https://203.0.113.1:5432/avatar.jpg",
				expectError: true,
				errorMsg:    "image url port must be 80 or 443, got 5432",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateImageURL(tt.urlString)
				if tt.expectError {
					if err == nil {
						t.Errorf("ValidateImageURL() expected error but got none")
						return
					}
					if tt.errorMsg != "" && err.Error() != tt.errorMsg {
						t.Errorf("ValidateImageURL() error = %v, want error containing %v", err.Error(), tt.errorMsg)
					}
				} else {
					if err != nil {
						t.Errorf("ValidateImageURL() unexpected error = %v", err)
					}
				}
			})
		}
	})

	t.Run("content type validation with HTTP server", func(t *testing.T) {
		tests := []struct {
			name         string
			contentType  string
			statusCode   int
			expectError  bool
			errorContain string
		}{
			{
				name:        "valid image/png",
				contentType: "image/png",
			},
			{
				name:        "valid image/jpeg",
				contentType: "image/jpeg",
			},
			{
				name:        "valid image/png with charset",
				contentType: "image/png; charset=utf-8",
			},
			{
				name:         "non-image content type text/html",
				contentType:  "text/html",
				expectError:  true,
				errorContain: "must be image/png or image/jpeg",
			},
			{
				name:         "non-image content type image/gif",
				contentType:  "image/gif",
				expectError:  true,
				errorContain: "must be image/png or image/jpeg",
			},
			{
				name:         "non-image content type image/svg+xml",
				contentType:  "image/svg+xml",
				expectError:  true,
				errorContain: "must be image/png or image/jpeg",
			},
			{
				name:         "server returns 404",
				statusCode:   404,
				contentType:  "image/png",
				expectError:  true,
				errorContain: "status 404",
			},
			{
				name:         "server returns 500",
				statusCode:   500,
				contentType:  "image/png",
				expectError:  true,
				errorContain: "status 500",
			},
			{
				name:         "server returns 301 redirect",
				statusCode:   301,
				contentType:  "image/png",
				expectError:  true,
				errorContain: "redirect status 301",
			},
			{
				name:         "server returns 302 redirect",
				statusCode:   302,
				contentType:  "image/png",
				expectError:  true,
				errorContain: "redirect status 302",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodHead {
						t.Errorf("expected HEAD request, got %s", r.Method)
					}
					if tt.statusCode > 0 {
						w.WriteHeader(tt.statusCode)
					}
					if tt.contentType != "" {
						w.Header().Set("Content-Type", tt.contentType)
					}
				}))
				defer server.Close()

				origValidator := imageHostValidator
				imageHostValidator = func(host string) error { return nil }
				origPortValidator := imagePortValidator
				imagePortValidator = func(port string) error { return nil }
				origClient := imageHTTPClient
				imageHTTPClient = server.Client()
				defer func() { imageHTTPClient = origClient }()
				defer func() { imagePortValidator = origPortValidator }()
				defer func() { imageHostValidator = origValidator }()

				err := ValidateImageURL(server.URL)
				if tt.expectError {
					if err == nil {
						t.Errorf("ValidateImageURL() expected error but got none")
						return
					}
					if tt.errorContain != "" && !strings.Contains(err.Error(), tt.errorContain) {
						t.Errorf("ValidateImageURL() error = %v, want error containing %v", err.Error(), tt.errorContain)
					}
				} else {
					if err != nil {
						t.Errorf("ValidateImageURL() unexpected error = %v", err)
					}
				}
			})
		}
	})
}

func TestExtractURLPath(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "endpoint with path",
			endpoint: "https://api.example.com/v1/videos",
			want:     "/v1/videos",
		},
		{
			name:     "endpoint with path and query",
			endpoint: "https://api.example.com/v1/videos?api-version=2026-01-01",
			want:     "/v1/videos",
		},
		{
			name:     "endpoint with surrounding spaces",
			endpoint: "  https://api.example.com/v1/videos  ",
			want:     "/v1/videos",
		},
		{
			name:     "endpoint without path",
			endpoint: "https://api.example.com",
			want:     "",
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			want:     "",
		},
		{
			name:     "invalid endpoint",
			endpoint: "://bad-url",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractURLPath(tt.endpoint); got != tt.want {
				t.Errorf("ExtractURLPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinURLPath(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		elems []string
		want  string
	}{
		{
			name:  "append segments",
			base:  "/v1/videos",
			elems: []string{"vid_123", "content"},
			want:  "/v1/videos/vid_123/content",
		},
		{
			name:  "trim duplicate slashes",
			base:  "/v1/videos/",
			elems: []string{"/vid_123/", "/content/"},
			want:  "/v1/videos/vid_123/content",
		},
		{
			name:  "skip empty segments",
			base:  "/v1/videos",
			elems: []string{"", "vid_123"},
			want:  "/v1/videos/vid_123",
		},
		{
			name:  "empty base",
			base:  "",
			elems: []string{"vid_123"},
			want:  "/vid_123",
		},
		{
			name:  "empty result",
			base:  "",
			elems: []string{"", "/"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinURLPath(tt.base, tt.elems...); got != tt.want {
				t.Errorf("JoinURLPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "https host without port",
			target: "https://csgbot.example.com",
			want:   "csgbot.example.com",
		},
		{
			name:   "https host with port",
			target: "https://csgbot.example.com:8070",
			want:   "csgbot.example.com",
		},
		{
			name:   "plain host",
			target: "csgbot.internal",
			want:   "csgbot.internal",
		},
		{
			name:   "host with port and path",
			target: "csgbot.example.com:8070/api/v1/chat",
			want:   "csgbot.example.com",
		},
		{
			name:   "host with path",
			target: "csgbot.internal/chat",
			want:   "csgbot.internal",
		},
		{
			name:   "localhost with port",
			target: "localhost:8070",
			want:   "localhost",
		},
		{
			name:   "empty host",
			target: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractHostname(tt.target); got != tt.want {
				t.Errorf("ExtractHostname() = %q, want %q", got, tt.want)
			}
		})
	}
}
