package utils

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	invalidChar = regexp.MustCompile(`[^a-z0-9\-.]+`)
	trimEdge    = regexp.MustCompile(`^[^a-z0-9]+|[^a-z0-9]+$`)
)

func ValidUrl(endpoint string) bool {
	if endpoint == "" {
		return false
	}

	// If the endpoint doesn't have a scheme (like "http://"), url.Parse treats it
	// as a relative path. To handle inputs like "127.0.0.1:8080" correctly,
	// we prepend a default scheme. This forces url.Parse to parse the host part.
	parseURL := endpoint
	if !strings.Contains(parseURL, "://") {
		parseURL = "http://" + parseURL
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		return false
	}

	// We must use u.Hostname() instead of checking u.Host.
	// For an input like "http://:8082", u.Host is ":8082", which is not empty.
	if u.Hostname() == "" {
		return false
	}

	return true
}

// SafeName sanitizes the input string to be a valid Kubernetes resource name.
// It follows RFC 1123 subdomain rules:
// - contain only lowercase alphanumeric characters, '-' or '.'
// - start with an alphanumeric character
// - end with an alphanumeric character
func SafeName(name string) string {
	// 1. Convert to lower case
	name = strings.ToLower(name)

	// 2. Replace invalid characters with '-'
	name = invalidChar.ReplaceAllString(name, "-")

	// 3. Trim non-alphanumeric characters from start and end
	name = trimEdge.ReplaceAllString(name, "")

	// 4. Limit length to 253 characters (K8s RFC1123 max 253)
	if len(name) > 253 {
		name = name[:253]
	}

	// 5. Prevent empty string
	if name == "" {
		name = "default"
	}

	return name
}
