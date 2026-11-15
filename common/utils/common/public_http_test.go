package common

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestPublicHTTPPolicyRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/a", "http://10.0.0.1/a", "http://169.254.169.254/a",
		"http://100.100.200.200/a", "http://0.0.0.0/a", "http://224.0.0.1/a",
		"http://192.0.2.1/a", "http://198.18.0.1/a", "http://240.0.0.1/a",
		"http://[::1]/a", "http://[fc00::1]/a", "http://[fe80::1]/a",
		"http://[::ffff:8.8.8.8]/a", "http://[64:ff9b::808:808]/a",
		"http://[2002:0808:0808::1]/a", "http://[2001:db8::1]/a",
		"http://2130706433/a", "http://0177.0.0.1/a", "http://127.1/a",
		"http://0x7f.0.0.1/a", "http://0x7f000001/a", "http://127.1./a",
		"http://localhost/a", "http://host.local/a", "http://[fe80::1%25eth0]/a",
		"http://example.com:8080/a", "http://example.com:/a", "file:///a",
		"http://user:secret@example.com/a", "http://example.com/a#fragment",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := ValidatePublicHTTPURL(raw)
			require.Error(t, err)
		})
	}
}

// Exercise the real Go DNS resolver rather than only injected address slices.
func TestPublicHTTPPolicyWithDNS(t *testing.T) {
	for _, privateAAAA := range []bool{false, true} {
		t.Run(fmt.Sprint(privateAAAA), func(t *testing.T) {
			server, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(t, err)
			defer server.Close()
			go func() {
				buf := make([]byte, 4096)
				for {
					n, peer, err := server.ReadFrom(buf)
					if err != nil {
						return
					}
					var message dnsmessage.Message
					if message.Unpack(buf[:n]) != nil || len(message.Questions) != 1 {
						continue
					}
					q := message.Questions[0]
					message.Header.Response = true
					message.Header.RecursionAvailable = true
					answer := dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: q.Type, Class: dnsmessage.ClassINET, TTL: 1}}
					switch q.Type {
					case dnsmessage.TypeA:
						answer.Body = &dnsmessage.AResource{A: [4]byte{8, 8, 8, 8}}
					case dnsmessage.TypeAAAA:
						if privateAAAA {
							answer.Body = &dnsmessage.AAAAResource{AAAA: netip.MustParseAddr("::ffff:127.0.0.1").As16()}
						}
					}
					if answer.Body != nil {
						message.Answers = []dnsmessage.Resource{answer}
					}
					payload, err := message.Pack()
					if err == nil {
						_, _ = server.WriteTo(payload, peer)
					}
				}
			}()
			resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "udp", server.LocalAddr().String())
			}}
			policy := PublicHTTPPolicy{LookupNetIP: resolver.LookupNetIP}
			ip, err := policy.ResolveURL(context.Background(), "https://source.example/ns/repo.git")
			if privateAAAA {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, "8.8.8.8", ip)
			}
		})
	}
}

func TestPublicHTTPTransportPreservesHostAndBlocksRedirect(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(fmt.Sprint(redirect), func(t *testing.T) {
			policy := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
			}}
			tr := policy.Transport()
			require.Nil(t, tr.Proxy, "environment proxies must not bypass pinning")
			defer tr.CloseIdleConnections()
			observedHost := make(chan string, 1)
			dials := 0
			tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return policy.dialContext(ctx, network, address, func(_ context.Context, _, pinned string) (net.Conn, error) {
					dials++
					if pinned != "8.8.8.8:80" {
						return nil, fmt.Errorf("unexpected destination %s", pinned)
					}
					client, server := net.Pipe()
					go func() {
						defer server.Close()
						req, err := http.ReadRequest(bufio.NewReader(server))
						if err != nil {
							observedHost <- err.Error()
							return
						}
						observedHost <- req.Host
						if redirect {
							_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: http://127.0.0.1/private\r\nContent-Length: 0\r\n\r\n")
						} else {
							_, _ = io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
						}
					}()
					return client, nil
				})
			}
			client := &http.Client{Transport: tr, Timeout: time.Second, CheckRedirect: RejectHTTPRedirect}
			resp, err := client.Get("http://example.com/ns/repo.git")
			if redirect {
				require.ErrorContains(t, err, "redirects are not allowed")
			} else {
				require.NoError(t, err)
			}
			if resp != nil {
				resp.Body.Close()
			}
			require.Equal(t, "example.com", <-observedHost)
			require.Equal(t, 1, dials)
		})
	}
}

func TestPublicHTTPPolicyDialRechecksAfterPreflight(t *testing.T) {
	lookups := 0
	policy := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		if lookups == 1 {
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}}
	_, err := policy.ResolveURL(context.Background(), "https://example.com/ns/repo.git")
	require.NoError(t, err)
	tr := policy.Transport()
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: time.Second, CheckRedirect: RejectHTTPRedirect}
	resp, err := client.Get("https://example.com/ns/repo.git")
	if resp != nil {
		resp.Body.Close()
	}
	require.ErrorContains(t, err, "non-public")
	require.Equal(t, 2, lookups)
}

func TestPublicHTTPPolicyResolution(t *testing.T) {
	for _, tc := range []struct {
		name      string
		addresses []netip.Addr
		err       error
		wantError bool
	}{
		{"public IPv4", []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil, false},
		{"public IPv6", []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111")}, nil, false},
		{"mixed answers", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, nil, true},
		{"mapped IPv6", []netip.Addr{netip.MustParseAddr("::ffff:8.8.8.8")}, nil, true},
		{"empty answers", nil, nil, true},
		{"DNS failure", nil, errors.New("DNS failed"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
				return tc.addresses, tc.err
			}}
			ip, err := policy.ResolveURL(context.Background(), "https://example.com/ns/repo.git")
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.addresses[0].String(), ip)
		})
	}
}

func TestPublicHTTPPolicyPinsDialAndRejectsRebinding(t *testing.T) {
	lookups := 0
	policy := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		if lookups > 1 {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}}
	var dialed string
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		require.Equal(t, "tcp", network)
		dialed = address
		return nil, errors.New("test dial")
	}
	_, err := policy.dialContext(context.Background(), "tcp", "example.com:443", dial)
	require.EqualError(t, err, "test dial")
	require.Equal(t, "8.8.8.8:443", dialed)
	dialed = ""
	_, err = policy.dialContext(context.Background(), "tcp", "example.com:443", dial)
	require.Error(t, err)
	require.Empty(t, dialed, "must reject before opening a socket")
}
