package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

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

// imageHostValidator is the function used to validate the image URL host.
// It is a package-level variable so tests can replace it.
var imageHostValidator = validateImageHost

// imagePortValidator is the function used to validate the image URL port.
// It is a package-level variable so tests can replace it.
var imagePortValidator = validateImagePort

// imageHTTPClient is the HTTP client used by ValidateImageURL to fetch
// image URLs. It is a package-level variable so tests can replace it.
// Redirects are not followed to prevent SSRF via open redirects.
// The DialContext validates the actual connected IP to prevent DNS rebinding.
var imageHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
				if isPrivateIP(tcpAddr.IP) {
					conn.Close()
					return nil, fmt.Errorf("connection to %s is not allowed: private or internal IP address", tcpAddr.IP)
				}
			}
			return conn, nil
		},
	},
}

// ValidateImageURL checks that the URL is a valid absolute HTTP/HTTPS URL on
// port 80 or 443, does not resolve to a private/internal IP address, and that
// the remote resource is a PNG or JPEG image by fetching its Content-Type.
// The HTTP transport validates the actual connected IP to prevent DNS rebinding.
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

	if err := imageHostValidator(u.Hostname()); err != nil {
		return err
	}
	if err := imagePortValidator(u.Port()); err != nil {
		return err
	}

	resp, err := imageHTTPClient.Head(urlString)
	if err != nil {
		return fmt.Errorf("failed to fetch image url: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("image url returned redirect status %d, redirects are not allowed", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("image url returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("failed to parse content type: %w", err)
	}

	switch mediaType {
	case "image/png", "image/jpeg":
		return nil
	default:
		return fmt.Errorf("image url content type is %s, must be image/png or image/jpeg", mediaType)
	}
}

// validateImageHost checks that the host does not resolve to a private,
// loopback, link-local, or unspecified IP address.
func validateImageHost(host string) error {
	// Check if the host is already an IP address.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("image url must not be a private or internal IP address")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("image url resolves to a private or internal IP address")
		}
	}
	return nil
}

// validateImagePort checks that the port is 80 or 443 (or empty, meaning the
// default port for the scheme).
func validateImagePort(port string) error {
	if port == "" || port == "80" || port == "443" {
		return nil
	}
	return fmt.Errorf("image url port must be 80 or 443, got %s", port)
}

// isPrivateIP returns true if the IP is loopback, private, link-local,
// multicast, unspecified, or an IPv6 transition address whose embedded
// IPv4 destination falls into any of those categories. Transition
// mechanisms translate traffic to the embedded IPv4 address, so checking
// only the IPv6 literal would miss private destinations hidden inside
// NAT64, 6to4, and Teredo addresses.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// Plain IPv4 or IPv4-mapped IPv6 (::ffff:0:0/96).
		return isPrivateIPv4(v4)
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, v4 := range transitionEmbeddedIPv4(ip) {
		if isPrivateIPv4(v4) {
			return true
		}
	}
	return false
}

// isPrivateIPv4 returns true if the 4-byte IP is loopback, private,
// link-local, multicast, unspecified, in the shared address space used by
// some cloud metadata services, or in the "this network" / reserved
// ranges which are not globally routable.
func isPrivateIPv4(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	switch {
	case ip[0] == 0:
		// 0.0.0.0/8 ("this network").
		return true
	case ip[0] == 100 && ip[1]&0xc0 == 64:
		// 100.64.0.0/10 shared address space (RFC 6598), e.g. the
		// 100.100.200.200 metadata endpoint on Alibaba Cloud.
		return true
	case ip[0] >= 240:
		// 240.0.0.0/4 reserved range including broadcast.
		return true
	}
	return false
}

// transitionEmbeddedIPv4 returns the IPv4 destinations that traffic to the
// IPv6 transition address ip is translated to: the NAT64 well-known prefix
// (64:ff9b::/96) embeds it in the last 4 bytes, 6to4 (2002::/16) after the
// prefix, and Teredo (2001::/32) as the Teredo server IPv4 plus the
// obfuscated (bitwise NOT) client IPv4 in the last 4 bytes. It returns nil
// for addresses that do not embed an IPv4 destination.
func transitionEmbeddedIPv4(ip net.IP) []net.IP {
	ip16 := ip.To16()
	if ip16 == nil {
		return nil
	}
	switch {
	case ip16[0] == 0x00 && ip16[1] == 0x64 && ip16[2] == 0xff && ip16[3] == 0x9b:
		for _, b := range ip16[4:12] {
			if b != 0 {
				return nil
			}
		}
		return []net.IP{net.IP(ip16[12:16])}
	case ip16[0] == 0x20 && ip16[1] == 0x02:
		return []net.IP{net.IP(ip16[2:6])}
	case ip16[0] == 0x20 && ip16[1] == 0x01 && ip16[2] == 0x00 && ip16[3] == 0x00:
		client := make(net.IP, 4)
		for i := 0; i < 4; i++ {
			client[i] = ^ip16[12+i]
		}
		return []net.IP{net.IP(ip16[4:8]), client}
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
