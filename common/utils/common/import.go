package common

import (
	"fmt"
	"net/url"
)

func ConvertURLWithAuth(baseURL, username, password string) (string, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	parsedURL.User = url.UserPassword(username, password)

	return parsedURL.String(), nil
}
