package common

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// PublicHTTPPolicy restricts untrusted HTTP destinations to public addresses.
// Its zero value uses the system resolver. LookupNetIP permits a service to use
// its own resolver without changing the address restrictions.
type PublicHTTPPolicy struct {
	// TrustedDomains bypasses public-address checks for exact DNS names chosen
	// by the administrator. Execution-side DNS/hosts mappings remain effective.
	TrustedDomains []string
	LookupNetIP    func(context.Context, string, string) ([]netip.Addr, error)
}

var nonPublicHTTPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	// Special-purpose, documentation and transition prefixes. Reject transition
	// addresses entirely rather than relying on deployment-specific translation.
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func isPublicHTTPAddress(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" || ip.Is4In6() || !ip.IsGlobalUnicast() ||
		ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	// IPv6 public unicast currently resides in 2000::/3. This also excludes
	// local NAT64, mapped, compatible and other translation address forms.
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range nonPublicHTTPPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

// ValidatePublicHTTPURL performs syntax and literal-address checks without DNS.
// Call ResolveURL or use Transport before making a request; syntax checks alone
// cannot protect against private DNS answers or DNS rebinding.
func ValidatePublicHTTPURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid outbound URL")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("outbound URL must be HTTP(S) with a host and without credentials or fragment")
	}
	if strings.HasSuffix(u.Host, ":") || (u.Port() != "" && u.Port() != "80" && u.Port() != "443") {
		return nil, fmt.Errorf("outbound URL port must be 80 or 443")
	}
	if err := validatePublicHTTPHost(u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

func validatePublicHTTPHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !isPublicHTTPAddress(ip) {
			return fmt.Errorf("outbound address is not public")
		}
		return nil
	}
	// Require an ASCII DNS name whose final label starts with a letter.
	// This rejects inet_aton-style integer, octal, hexadecimal and shortened
	// IPv4 forms that curl might interpret differently from the Go resolver.
	host = strings.TrimSuffix(host, ".")
	labels := strings.Split(host, ".")
	last := labels[len(labels)-1]
	if len(labels) < 2 || len(host) > 253 || last == "" || !isASCIILetter(last[0]) {
		return fmt.Errorf("outbound host must be a public IP or DNS name")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("invalid outbound DNS name")
		}
		for i := range label {
			if !isASCIILetter(label[i]) && (label[i] < '0' || label[i] > '9') && label[i] != '-' {
				return fmt.Errorf("invalid outbound DNS name")
			}
		}
	}
	switch strings.ToLower(last) {
	case "localhost", "local", "internal", "home", "lan", "arpa":
		return fmt.Errorf("outbound host is not public")
	}
	return nil
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// ResolveURL validates every DNS answer and returns an IP to pin at connection
// time. A trusted domain returns an empty pin and is resolved by the executing
// service. Other hosts must not subsequently be re-resolved before connection.
func (p PublicHTTPPolicy) ResolveURL(ctx context.Context, raw string) (string, error) {
	addresses, err := p.ResolveURLAddresses(ctx, raw)
	if err != nil {
		return "", err
	}
	return addresses[0], nil
}

// ResolveURLAddresses returns a validated DNS snapshot for connection fallback.
// Every address must pass before any may be used. Callers must not re-resolve
// untrusted hosts between attempts or fall back to an unpinned connection.
// Trusted domains return a single empty string: Gitaly must leave ResolvedAddress
// empty so its own DNS/hosts mapping resolves the source, without a preflight.
func (p PublicHTTPPolicy) ResolveURLAddresses(ctx context.Context, raw string) ([]string, error) {
	u, err := ValidatePublicHTTPURL(raw)
	if err != nil {
		return nil, err
	}
	if p.isTrustedDomain(u.Hostname()) {
		return []string{""}, nil
	}
	return p.resolveHostAddresses(ctx, u.Hostname())
}

func (p PublicHTTPPolicy) isTrustedDomain(host string) bool {
	if _, err := netip.ParseAddr(host); err == nil || validatePublicHTTPHost(host) != nil {
		return false
	}
	host = strings.TrimSuffix(host, ".")
	for _, domain := range p.TrustedDomains {
		domain = strings.TrimSuffix(strings.TrimSpace(domain), ".")
		if strings.EqualFold(host, domain) {
			return true
		}
	}
	return false
}

func (p PublicHTTPPolicy) resolveHostAddresses(ctx context.Context, host string) ([]string, error) {
	if err := validatePublicHTTPHost(host); err != nil {
		return nil, err
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return []string{ip.String()}, nil
	}
	lookup := p.LookupNetIP
	if lookup == nil {
		lookup = net.DefaultResolver.LookupNetIP
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// A trailing dot prevents local DNS search suffixes from changing the name.
	ips, err := lookup(ctx, "ip", strings.TrimSuffix(host, ".")+".")
	if err != nil {
		return nil, fmt.Errorf("resolve outbound host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("outbound host has no addresses")
	}
	for _, ip := range ips {
		if !isPublicHTTPAddress(ip) {
			return nil, fmt.Errorf("outbound host resolves to a non-public address")
		}
	}
	addresses := make([]string, 0, len(ips))
	seen := make(map[netip.Addr]bool, len(ips))
	for _, ip := range ips {
		if !seen[ip] {
			addresses = append(addresses, ip.String())
			seen[ip] = true
		}
	}
	return addresses, nil
}

// Transport dials validated numeric addresses, or trusted domain names using
// local DNS/hosts, preserving HTTP Host and TLS SNI. Proxies remain disabled.
// The caller must reject redirects or validate every hop and strip credentials
// when crossing origins. Redirects must use this same protected transport.
func (p PublicHTTPPolicy) Transport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return p.dialContext(ctx, network, address, (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func (p PublicHTTPPolicy) dialContext(ctx context.Context, network, address string, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid outbound address")
	}
	if port != "80" && port != "443" {
		return nil, fmt.Errorf("outbound port must be 80 or 443")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if p.isTrustedDomain(host) {
		return dial(ctx, network, address)
	}
	addresses, err := p.resolveHostAddresses(ctx, host)
	if err != nil {
		return nil, err
	}
	var failures []error
	for i, ip := range addresses {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(append(failures, err)...)
		}
		// Reserve a share of the remaining overall budget for every address.
		deadline, _ := ctx.Deadline()
		attemptCtx, stop := context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(addresses)-i))
		conn, err := dial(attemptCtx, network, net.JoinHostPort(ip, port))
		stop()
		if err == nil {
			return conn, nil
		}
		failures = append(failures, err)
	}
	return nil, errors.Join(failures...)
}

// RejectHTTPRedirect prevents an approved origin from redirecting to another
// destination. Returning an error also prevents 3xx responses being accepted.
func RejectHTTPRedirect(_ *http.Request, _ []*http.Request) error {
	return fmt.Errorf("outbound HTTP redirects are not allowed")
}
