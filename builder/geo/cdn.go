package geo

import (
	"fmt"
	"net/netip"
	"net/url"
)

// CDNUrl replace the host of originalUrl to the cdn by user client ip geo location
//
// if no matched cdn domain, return the original url
func CDNUrlString(clientIP string, originalUrl string) (string, error) {
	parsedUrl, err := url.Parse(originalUrl)
	if err != nil {
		return originalUrl, fmt.Errorf("failed to parse original url, %w", err)
	}
	cdnUrl, err := CDNUrl(clientIP, parsedUrl)
	if err != nil {
		return originalUrl, fmt.Errorf("failed to get cdn url, %w", err)
	}
	return cdnUrl.String(), nil
}

func CDNUrl(clientIP string, originalUrl *url.URL) (*url.URL, error) {
	if len(cityToCdnDomain) == 0 {
		return originalUrl, nil
	}

	cdnDomain, err := getCdnDomainByIp(clientIP)
	if err != nil {
		return originalUrl, fmt.Errorf("failed to get cdn domain by ip, %w", err)
	}
	if cdnDomain != "" {
		originalUrl.Host = cdnDomain
	}
	return originalUrl, nil
}

func getCdnDomainByIp(clientIP string) (string, error) {
	ip, err := netip.ParseAddr(clientIP)
	if err != nil {
		return "", fmt.Errorf("failed to parse client ip, %w", err)
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() {
		return "", nil
	}
	if ipLocator == nil {
		ipLocator = NewGaodeIPLocator(lbsServiceKey)
	}
	loc, err := ipLocator.GetIPLocation(clientIP)
	if err != nil {
		return "", fmt.Errorf("failed to get ip location, %w", err)
	}

	city := loc.City
	return getCdnDomainByCity(city), nil
}

func getCdnDomainByCity(city string) string {
	if cdnDomain, ok := cityToCdnDomain[city]; ok {
		return cdnDomain
	}
	return ""
}
