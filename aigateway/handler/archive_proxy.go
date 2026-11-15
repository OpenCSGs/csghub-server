package handler

import (
	"opencsg.com/csghub-server/aigateway/httparchive"
	"opencsg.com/csghub-server/builder/proxy"
)

// newArchiveProxy is shared by every Gateway protocol, including unknown future bodies.
// Capture runs after the proxy Director and before response transformations.
func newArchiveProxy(target string, options ...proxy.ReverseProxyOption) (proxy.ReverseProxy, error) {
	options = append(options, proxy.WithTransport(httparchive.WrapTransport(nil)))
	return proxy.NewReverseProxy(target, options...)
}
