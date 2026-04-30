package common

import (
	"fmt"
	"net/url"
	"strings"
)

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
