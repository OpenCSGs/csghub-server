package sample

import (
	"fmt"
	"net/url"
	"strings"
)

func endpointMatchesRoute(endpoint, route string) bool {
	endpointURL, endpointSegments, err := parsedRouteSegments(endpoint)
	if err != nil || endpointURL.Scheme == "" || endpointURL.Host == "" {
		return false
	}
	_, routeSegments, err := parsedRouteSegments(route)
	if err != nil || len(routeSegments) == 0 || len(endpointSegments) < len(routeSegments) {
		return false
	}
	start := len(endpointSegments) - len(routeSegments)
	for index := range routeSegments {
		if endpointSegments[start+index] != routeSegments[index] {
			return false
		}
	}
	return true
}

func endpointBaseURL(endpoint, route string) (string, error) {
	parsed, endpointSegments, err := parsedRouteSegments(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse sample endpoint: %w", err)
	}
	_, routeSegments, err := parsedRouteSegments(route)
	if err != nil {
		return "", fmt.Errorf("parse sample route: %w", err)
	}
	if !endpointMatchesRoute(endpoint, route) {
		return "", fmt.Errorf("unsupported sample endpoint %q", endpoint)
	}

	baseSegments := endpointSegments[:len(endpointSegments)-len(routeSegments)]
	parsed.Path = "/" + strings.Join(baseSegments, "/")
	if len(baseSegments) == 0 {
		parsed.Path = ""
	}
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func parsedRouteSegments(rawURL string) (*url.URL, []string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return parsed, segments, nil
}
