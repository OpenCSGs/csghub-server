package common

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
)

var unsafeHTTPHeaders = map[string]struct{}{
	"connection":          {},
	"content-length":      {},
	"host":                {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"proxy-connection":    {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

// ValidateImageURL checks that the URL is a valid absolute URL and its path
// ends with an allowed image extension (.jpg, .jpeg, .png).
func ValidateImageURL(urlString string) error {
	if urlString == "" {
		return fmt.Errorf("url is empty")
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("url must have a host")
	}

	// Extract the path portion without query string and check extension
	path := strings.ToLower(u.Path)
	ext := ""
	for _, candidate := range []string{".jpeg", ".jpg", ".png"} {
		if strings.HasSuffix(path, candidate) {
			ext = candidate
			break
		}
	}
	if ext == "" {
		return fmt.Errorf("avatar url must end with .jpg, .jpeg, or .png")
	}

	return nil
}

func ValidateURLFormat(urlString string) error {
	if urlString == "" {
		return fmt.Errorf("url is empty")
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	if u.Scheme == "" {
		return fmt.Errorf("url must have a scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("url must have a host")
	}
	return nil
}

func ValidateHeader(key, value string) error {
	key = strings.TrimSpace(key)
	if !httpguts.ValidHeaderFieldName(key) {
		return errors.New("invalid header name")
	}
	if !httpguts.ValidHeaderFieldValue(value) {
		return errors.New("invalid header value")
	}
	if _, ok := unsafeHTTPHeaders[strings.ToLower(key)]; ok {
		return errors.New("unsafe header")
	}
	return nil
}

func ExtractURLPath(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return ""
	}
	return uri.Path
}

func JoinURLPath(base string, elems ...string) string {
	parts := make([]string, 0, len(elems)+1)
	if strings.Trim(base, "/") != "" {
		parts = append(parts, strings.Trim(base, "/"))
	}
	for _, elem := range elems {
		if strings.Trim(elem, "/") == "" {
			continue
		}
		parts = append(parts, strings.Trim(elem, "/"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "/" + strings.Join(parts, "/")
}

func ExtractHostname(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	candidate := target
	if !strings.Contains(candidate, "://") {
		candidate = "//" + candidate
	}

	parsedURL, err := url.Parse(candidate)
	if err != nil {
		return ""
	}

	if hostname := parsedURL.Hostname(); hostname != "" {
		return hostname
	}

	host := parsedURL.Host
	if host == "" {
		host = strings.Split(parsedURL.Path, "/")[0]
	}

	if host == "" {
		return ""
	}

	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			return parsedHost
		}
	}

	return host
}
