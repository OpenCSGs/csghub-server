//go:build !ee && !saas

package httparchive

import (
	"net/http"
	"opencsg.com/csghub-server/common/config"
)

func WrapTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		return http.DefaultTransport
	}
	return base
}
func Init(_ *config.Config) (func(), error) { return func() {}, nil }

func RegisterAuthHeader(_ string) {}
