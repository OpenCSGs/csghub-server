package lfssyncer

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/utils/common"
)

func TestLFSRedirectRevalidatesDestinationDNS(t *testing.T) {
	lookups := 0
	policy := common.PublicHTTPPolicy{TrustedDomains: []string{"source.example"}, LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}}
	client := newLFSSourceHTTPClient(policy)
	protectedTransport := client.Transport.(*http.Transport)
	defer protectedTransport.CloseIdleConnections()
	client.Transport = sourceRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "source.example" {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://rebound.example/file"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return protectedTransport.RoundTrip(r)
	})
	resp, err := client.Get("https://source.example/file")
	if resp != nil {
		resp.Body.Close()
	}
	require.ErrorContains(t, err, "non-public")
	require.Equal(t, 1, lookups)
}

type sourceRoundTripFunc func(*http.Request) (*http.Response, error)

func (f sourceRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLFSPublicRedirectsStripCrossOriginCredentials(t *testing.T) {
	client := newLFSSourceHTTPClient(common.PublicHTTPPolicy{})
	requests := 0
	client.Transport = sourceRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		require.Equal(t, "bytes=0-9", r.Header.Get("Range"))
		if requests > 1 {
			require.Empty(t, r.Header.Get("Authorization"))
			require.Empty(t, r.Header.Get("Cookie"))
			require.Empty(t, r.Header.Get("X-Custom-Secret"))
		}
		resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data")), Request: r}
		switch requests {
		case 1:
			resp.StatusCode = http.StatusTemporaryRedirect
			resp.Header.Set("Location", "https://cdn.example/file?signature=public-signed-link")
		case 2:
			require.Equal(t, "signature=public-signed-link", r.URL.RawQuery)
			resp.StatusCode = http.StatusFound
			resp.Header.Set("Location", "https://source.example/final")
		}
		return resp, nil
	})
	req, err := http.NewRequest(http.MethodGet, "https://source.example/file", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("X-Custom-Secret", "secret")
	req.Header.Set("Range", "bytes=0-9")
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	require.NoError(t, err)
	require.Equal(t, 3, requests)
}

func TestLFSRedirectPolicyRejectsUnsafeTargetsAndLoops(t *testing.T) {
	for _, target := range []string{"http://public.example/file", "https://127.0.0.1/file", "https://public.example:8080/file", "https://user:secret@public.example/file", "https://source.example/loop"} {
		t.Run(target, func(t *testing.T) {
			client := newLFSSourceHTTPClient(common.PublicHTTPPolicy{})
			requests := 0
			client.Transport = sourceRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {target}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})
			resp, err := client.Get("https://source.example/file")
			if resp != nil {
				resp.Body.Close()
			}
			require.Error(t, err)
			if strings.HasSuffix(target, "/loop") {
				require.Equal(t, 6, requests)
			} else {
				require.Equal(t, 1, requests)
			}
		})
	}
}

func TestLFSSameOriginRedirectPreservesAuthorization(t *testing.T) {
	client := newLFSSourceHTTPClient(common.PublicHTTPPolicy{})
	first, err := http.NewRequest(http.MethodGet, "https://source.example/file", nil)
	require.NoError(t, err)
	next, err := http.NewRequest(http.MethodGet, "https://source.example:443/next", nil)
	require.NoError(t, err)
	next.Header.Set("Authorization", "Bearer token")
	require.NoError(t, client.CheckRedirect(next, []*http.Request{first}))
	require.Equal(t, "Bearer token", next.Header.Get("Authorization"))
}
