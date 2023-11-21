package serverhost

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type ServerHost struct {
	// e.g: http[s]://example.com[:port]
	apiSchemeHost string
	// e.g: example.com[:port]
	apiHost string
	// e.g: http[s]://example.com[:port]
	// webhookSchemeHost string
	// e.g: http[s]://docs.example.com[:port]
	docsSchemaHost string
}

type Opt struct {
	// API external host, e.g: http(s)://example.com:port
	API string
	// webhook external host, e.g: http(s)://example.com:port
	// WebHook string
	// docs external host, e.g: http(s)://example.com:port
	DocsHost string
}

func NewServerHost(opt *Opt) (host *ServerHost, err error) {
	parsedAPIURL, err := url.Parse(opt.API)
	if err != nil {
		err = fmt.Errorf("parsing API: %w", err)
		return
	}
	if parsedAPIURL.Scheme == "" {
		err = fmt.Errorf("scheme of API %q is missing", opt.API)
		return
	}
	if parsedAPIURL.Host == "" {
		err = fmt.Errorf("host of API %q is missing", opt.API)
		return
	}

	// parsedWebhookURL, err := url.Parse(opt.WebHook)
	// if err != nil {
	// 	err = fmt.Errorf("parsing webhook: %w", err)
	// 	return
	// }
	// if parsedWebhookURL.Scheme == "" {
	// 	err = fmt.Errorf("scheme of webhook %q is missing", opt.WebHook)
	// 	return
	// }
	// if parsedWebhookURL.Host == "" {
	// 	err = fmt.Errorf("host of webhook %q is missing", opt.WebHook)
	// 	return
	// }

	parsedDocsURL, err := url.Parse(opt.DocsHost)
	if err != nil {
		err = fmt.Errorf("parsing docs: %w", err)
		return
	}

	host = &ServerHost{
		apiSchemeHost: fmt.Sprintf("%s://%s", parsedAPIURL.Scheme, parsedAPIURL.Host),
		apiHost:       parsedAPIURL.Host,
		// webhookSchemeHost: fmt.Sprintf("%s://%s", parsedWebhookURL.Scheme, parsedWebhookURL.Host),
		docsSchemaHost: fmt.Sprintf("%s://%s", parsedDocsURL.Scheme, parsedDocsURL.Host),
	}

	return
}

// Webhook returns webhook external host
// in the form http[s]://example.com[:port]
// func (m *ServerHost) Webhook() string {
// 	return m.webhookSchemeHost
// }

// API returns API external host
// in the form http[s]://example.com[:port]
func (m *ServerHost) API() string {
	return m.apiSchemeHost
}

func (m *ServerHost) Docs() string {
	return m.docsSchemaHost
}

// APIFullURL returns API returns external URL,
// which is a combination of API host and the specified path.
// path looks like /a/b/c
func (m *ServerHost) APIFullURL(path string) string {
	return fmt.Sprintf("%s/%s", m.apiSchemeHost, strings.TrimPrefix(path, "/"))
}

// WebhookFullURL returns Webhook returns external URL,
// which is a combination of Webhook host and the specified path.
// path looks like /a/b/c
// func (m *ServerHost) WebhookFullURL(path string) string {
// 	return fmt.Sprintf("%s/%s", m.webhookSchemeHost, strings.TrimPrefix(path, "/"))
// }

// IsInboundURL checks if a URL links to resources inside UltraFox.
// When err is nil, it extracts the URL root (e.g. https://example.com) for convenience.
func (m *ServerHost) IsInboundURL(URL string) (root string, err error) {
	if URL == "" {
		err = errors.New("URL is empty")
		return
	}
	parsed, err := url.Parse(URL)
	if err != nil {
		err = fmt.Errorf("parsing URL: %w", err)
		return
	}

	host := parsed.Hostname()
	if host == "" && parsed.Scheme == "" {
		// relative redirect URL
		return
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		// frontend guys are doing development
		scheme := parsed.Scheme
		if scheme == "" {
			scheme = "http"
		}
		root = fmt.Sprintf("%s://%s", scheme, parsed.Host)
		return
	}
	if parsed.Scheme != "https" {
		err = fmt.Errorf("unsafe scheme in redirectURL %q", URL)
		return
	}
	if parsed.Host != m.apiHost {
		err = fmt.Errorf("untrusted URL host %q, want %q", parsed.Host, m.apiHost)
		return
	}

	root = "https://" + parsed.Host
	return
}
