package common

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicHTTPTrustedDomainsSkipDNSPreflight(t *testing.T) {
	p := PublicHTTPPolicy{
		TrustedDomains: []string{" HuggingFace.COM. "},
		LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
			t.Fatal("trusted domains must be resolved by the executing service")
			return nil, nil
		},
	}
	for _, source := range []string{"https://huggingface.com/repo.git", "http://HUGGINGFACE.COM./repo.git"} {
		addresses, err := p.ResolveURLAddresses(context.Background(), source)
		require.NoError(t, err)
		require.Equal(t, []string{""}, addresses, "empty pin preserves Gitaly's DNS/hosts mapping")
		address, err := p.ResolveURL(context.Background(), source)
		require.NoError(t, err)
		require.Empty(t, address)
	}
}

func TestPublicHTTPTrustedDomainsAreExactDNSNames(t *testing.T) {
	for _, tc := range []struct{ domain, source string }{
		{"huggingface.com", "https://sub.huggingface.com/repo"},
		{"huggingface.com", "https://huggingface.com.evil.example/repo"},
		{"huggingface.com", "https://evilhuggingface.com/repo"},
		{"*.huggingface.com", "https://sub.huggingface.com/repo"},
		{"https://huggingface.com", "https://huggingface.com/repo"},
		{"huggingface.com:443", "https://huggingface.com/repo"},
		{"127.0.0.1", "http://127.0.0.1/repo"},
		{"2130706433", "http://2130706433/repo"},
		{"huggingface.com", "https://user@huggingface.com/repo"},
		{"huggingface.com", "https://huggingface.com:8080/repo"},
		{"huggingface.com", "ftp://huggingface.com/repo"},
	} {
		t.Run(tc.domain+"/"+tc.source, func(t *testing.T) {
			p := PublicHTTPPolicy{TrustedDomains: []string{tc.domain}, LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
			}}
			_, err := p.ResolveURLAddresses(context.Background(), tc.source)
			require.Error(t, err)
		})
	}
}

func TestPublicHTTPTrustedDialPreservesHostname(t *testing.T) {
	p := PublicHTTPPolicy{TrustedDomains: []string{"huggingface.com"}, LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("trusted dial must not perform a DNS preflight")
		return nil, nil
	}}
	wantErr := errors.New("dial result")
	_, err := p.dialContext(context.Background(), "tcp", "huggingface.com:443", func(ctx context.Context, network, address string) (net.Conn, error) {
		require.Equal(t, "tcp", network)
		require.Equal(t, "huggingface.com:443", address)
		_, hasDeadline := ctx.Deadline()
		require.True(t, hasDeadline)
		return nil, wantErr
	})
	require.ErrorIs(t, err, wantErr)
}

func TestPublicHTTPTrustedListDoesNotExemptOtherDialTargets(t *testing.T) {
	p := PublicHTTPPolicy{TrustedDomains: []string{"huggingface.com", "127.0.0.1"}, LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
	}}
	for _, target := range []string{"other.example:443", "sub.huggingface.com:443", "127.0.0.1:80", "huggingface.com:8080"} {
		_, err := p.dialContext(context.Background(), "tcp", target, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("nontrusted or nonstandard-port targets must be rejected before dialing")
			return nil, nil
		})
		require.Error(t, err)
	}
	addresses, err := (PublicHTTPPolicy{TrustedDomains: []string{"8.8.8.8"}}).ResolveURLAddresses(context.Background(), "https://8.8.8.8/repo")
	require.NoError(t, err)
	require.Equal(t, []string{"8.8.8.8"}, addresses, "public IP literals must remain pinned")
}
