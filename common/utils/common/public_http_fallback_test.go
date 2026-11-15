package common

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublicHTTPDialFallsBackWithinValidatedAnswers(t *testing.T) {
	lookups := 0
	p := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		require.Equal(t, 1, lookups, "fallback must not resolve the hostname again")
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}, nil
	}}
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()
	var attempted []string
	conn, err := p.dialContext(context.Background(), "tcp", "example.com:443", func(_ context.Context, _, address string) (net.Conn, error) {
		attempted = append(attempted, address)
		if len(attempted) == 1 {
			return nil, errors.New("connection refused")
		}
		return client, nil
	})
	require.NoError(t, err)
	require.Same(t, client, conn)
	require.Equal(t, []string{"8.8.8.8:443", "1.1.1.1:443"}, attempted)
}

func TestPublicHTTPDialReservesTimeForFallback(t *testing.T) {
	p := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	parentDeadline, _ := ctx.Deadline()
	attempts := 0
	_, err := p.dialContext(ctx, "tcp", "example.com:443", func(attemptCtx context.Context, _, _ string) (net.Conn, error) {
		attempts++
		deadline, ok := attemptCtx.Deadline()
		require.True(t, ok)
		if attempts == 1 {
			require.True(t, deadline.Before(parentDeadline))
		}
		return nil, errors.New("dial failure")
	})
	require.Error(t, err)
	require.Equal(t, 2, attempts)
}

func TestPublicHTTPDialChecksAllAnswersBeforeAnyConnection(t *testing.T) {
	p := PublicHTTPPolicy{LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, nil
	}}
	_, err := p.dialContext(context.Background(), "tcp", "example.com:443", func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("no connection is permitted for a mixed DNS answer")
		return nil, nil
	})
	require.Error(t, err)
}
