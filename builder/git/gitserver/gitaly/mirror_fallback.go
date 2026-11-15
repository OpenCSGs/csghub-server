package gitaly

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *Client) fetchMirrorRemote(ctx context.Context, req *gitalypb.FetchRemoteRequest, addresses []string) error {
	return retryMirrorAddresses(ctx, addresses, func(address string) error {
		req.RemoteParams.ResolvedAddress = address
		_, err := c.repoClient.FetchRemote(ctx, req)
		return err
	})
}

// Gitaly's clone/fetch RPCs accept one pinned address. Retry only a failure
// reported before a clone is published, or a fetch connection is established. Do not retry RPC
// transport errors, cancellation, timeouts or reference-commit failures: their
// outcome may be uncertain. Every attempt shares the original overall deadline.
func retryMirrorAddresses(ctx context.Context, addresses []string, call func(string) error) error {
	if len(addresses) == 0 {
		return fmt.Errorf("mirror source has no validated addresses")
	}
	var err error
	for _, address := range addresses {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = call(address)
		if err == nil || !isMirrorRemoteCommandFailure(err) {
			return err
		}
	}
	return err
}

func isMirrorRemoteCommandFailure(err error) bool {
	s, ok := status.FromError(err)
	if !ok || s.Code() != codes.Internal {
		return false
	}
	// Verified against 16.4.6 and 16.10.10: clone errors here precede
	// repoutil.Create's atomic rename. Older FetchRemote implementations are
	// not atomic, so the general "fetch remote:" prefix is NOT safe to retry.
	if strings.HasPrefix(s.Message(), "creating repository: cloning repository:") {
		return true
	}
	return isMirrorFetchConnectFailure(s.Message())
}

func isMirrorFetchConnectFailure(message string) bool {
	quotedStderr, ok := strings.CutPrefix(message, "fetch remote: ")
	if !ok {
		return false
	}
	quoted, err := strconv.QuotedPrefix(quotedStderr)
	if err != nil || strings.TrimPrefix(quotedStderr, quoted) != ": exit status 128" {
		return false
	}
	stderr, err := strconv.Unquote(quoted)
	if err != nil {
		return false
	}
	line := strings.TrimSpace(stderr)
	if strings.ContainsAny(line, "\r\n") || !strings.HasPrefix(line, "fatal: unable to access '") {
		return false
	}
	_, reason, ok := strings.Cut(line, "': ")
	// Match only Git/libcurl's initial connection failure, not resets, transfer
	// errors or ref-update failures which may follow partial publication.
	return ok && strings.HasPrefix(reason, "Failed to connect to ")
}
