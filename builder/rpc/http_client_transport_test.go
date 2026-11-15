package rpc

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWrapTransportPreservesClientPolicy(t *testing.T) {
	base := &http.Transport{}
	redirects := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c := &HttpClient{hc: &http.Client{Transport: base, Timeout: 3 * time.Second, CheckRedirect: redirects}, retry: 2, retryDelay: time.Second}
	calls := 0
	returned := c.WrapTransport(func(rt http.RoundTripper) http.RoundTripper {
		require.Same(t, base, rt)
		return transportTestFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
		})
	})
	require.Same(t, c, returned)
	require.Equal(t, 3*time.Second, c.hc.Timeout)
	require.NotNil(t, c.hc.CheckRedirect)
	require.Equal(t, uint(2), c.retry)
	require.Equal(t, time.Second, c.retryDelay)
	req, err := http.NewRequest(http.MethodGet, "http://unused.invalid", nil)
	require.NoError(t, err)
	resp, err := c.hc.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, 1, calls)
	require.Same(t, c, c.WrapTransport(nil))
}

type transportTestFunc func(*http.Request) (*http.Response, error)

func (f transportTestFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
