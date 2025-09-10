package utils

import (
	"net/url"
	"strings"
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
