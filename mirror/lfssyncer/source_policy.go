package lfssyncer

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/common/utils/common"
)

// Both LFS Batch URLs and download actions are controlled by the remote source.
// Each new connection must resolve, validate and pin its own destination, even
// after a successful Git fetch or a cached download action. Large downloads use
// the request context for cancellation rather than a whole-response timeout.
func newLFSSourceHTTPClient(policy common.PublicHTTPPolicy) *http.Client {
	return &http.Client{
		Transport:     policy.Transport(),
		CheckRedirect: checkLFSSourceRedirect,
	}
}

// Redirects retain the protected transport: every new connection validates all
// DNS answers and dials only numeric addresses. HTTPS may never downgrade.
func checkLFSSourceRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 5 {
		return fmt.Errorf("too many LFS redirects")
	}
	if _, err := common.ValidatePublicHTTPURL(req.URL.String()); err != nil {
		return err
	}
	if len(via) == 0 {
		return nil
	}
	if via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("LFS redirect must not downgrade HTTPS")
	}
	// net/http copies headers from the initial request on every hop, including
	// when returning to its origin. Once a chain crosses origins, strip all
	// source-provided credentials (including arbitrary LFS action headers).
	crossOrigin := !sameLFSOrigin(via[0].URL, req.URL)
	for _, previous := range via[1:] {
		crossOrigin = crossOrigin || !sameLFSOrigin(via[0].URL, previous.URL)
	}
	if crossOrigin {
		headers := make(http.Header)
		for _, key := range []string{"Accept", "Content-Type", "User-Agent", "Range"} {
			for _, value := range req.Header.Values(key) {
				headers.Add(key, value)
			}
		}
		req.Header = headers
		req.Host = ""
	}
	return nil
}

func sameLFSOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && lfsURLPort(a) == lfsURLPort(b)
}

func lfsURLPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
