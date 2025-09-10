package common

import (
	"fmt"
	"net/url"
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
