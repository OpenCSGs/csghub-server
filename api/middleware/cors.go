package middleware

import (
	"log/slog"
	"net/url"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

// BuildCORS returns the shared gin CORS middleware driven by an origin
// whitelist. It replaces the previous global AllowAllOrigins+AllowCredentials
// combination (CWE-942, issue #1430): whitelisted origins get their exact
// origin echoed back in Access-Control-Allow-Origin together with
// Access-Control-Allow-Credentials, while any other foreign origin is rejected
// with 403 and receives no cross-origin read headers. Requests without an
// Origin header and same-origin browser requests are passed through untouched.
// When config.CORS.AllowedOrigins carries at least one valid entry the
// whitelist is exactly that list; when it is empty the fallback in
// collectAllowedOrigins applies instead.
func BuildCORS(config *config.Config) gin.HandlerFunc {
	origins := collectAllowedOrigins(config)
	return cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowOriginFunc: func(origin string) bool {
			return matchCORSOrigin(origins, origin)
		},
	})
}

// collectAllowedOrigins resolves the effective CORS whitelist. An explicit
// CORS.AllowedOrigins list with at least one valid entry wins outright and
// disables derivation, giving deployments full control (including narrowing).
// An empty or entirely invalid list falls back to the deployment's own
// addresses: the exact origins of the frontend (Frontend.URL) and the API
// (APIServer.PublicDomain), and a per-label subdomain wildcard over both
// schemes for Space.PublicRootDomain (space instances are exposed at
// <instance>.<PublicRootDomain>). Invalid entries are dropped with a warning;
// duplicates are removed. The resolved list is logged so the effective policy
// can be verified at startup.
func collectAllowedOrigins(config *config.Config) []string {
	add := func(origins []string, entry string) []string {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return origins
		}
		normalized := strings.TrimSuffix(strings.ToLower(entry), "/")
		if _, _, ok := splitOrigin(normalized); !ok {
			slog.Warn("ignoring invalid CORS allowed origin", "origin", entry)
			return origins
		}
		for _, existing := range origins {
			if existing == normalized {
				return origins
			}
		}
		return append(origins, normalized)
	}

	explicit := make([]string, 0, len(config.CORS.AllowedOrigins))
	for _, entry := range config.CORS.AllowedOrigins {
		explicit = add(explicit, entry)
	}
	if len(explicit) > 0 {
		slog.Info("CORS allowed origins from explicit config", "origins", explicit)
		return explicit
	}

	origins := make([]string, 0, 4)
	origins = add(origins, config.Frontend.URL)
	origins = add(origins, config.APIServer.PublicDomain)
	if root := strings.ToLower(strings.TrimSpace(config.Space.PublicRootDomain)); root != "" &&
		!strings.ContainsAny(root, "/:") {
		origins = add(origins, "https://*."+root)
		origins = add(origins, "http://*."+root)
	}
	slog.Info("CORS allowed origins derived from deployment config", "origins", origins)
	return origins
}

// matchCORSOrigin reports whether origin is covered by one of the whitelist
// entries. An entry that is not a valid origin never matches. The wildcard
// label "*" matches exactly one host label, following DNS wildcard semantics:
// "https://*.opencsg.com" matches "https://a.opencsg.com" but neither
// "https://a.b.opencsg.com" nor "https://evil-opencsg.com".
func matchCORSOrigin(patterns []string, origin string) bool {
	scheme, host, ok := splitOrigin(origin)
	if !ok {
		return false
	}
	for _, pattern := range patterns {
		pScheme, pHost, ok := splitOrigin(pattern)
		if !ok || pScheme != scheme {
			continue
		}
		if matchOriginHost(pHost, host) {
			return true
		}
	}
	return false
}

// matchOriginHost compares host labels pairwise; a "*" pattern label matches
// any single non-empty host label.
func matchOriginHost(pattern, host string) bool {
	patternLabels := strings.Split(pattern, ".")
	hostLabels := strings.Split(host, ".")
	if len(patternLabels) != len(hostLabels) {
		return false
	}
	for i, patternLabel := range patternLabels {
		if patternLabel == "*" {
			if hostLabels[i] == "" {
				return false
			}
			continue
		}
		if patternLabel != hostLabels[i] {
			return false
		}
	}
	return true
}

// splitOrigin normalizes an origin (lowercase scheme and host, no trailing
// slash) and validates that it carries a scheme and a host but no path.
func splitOrigin(value string) (scheme, host string, ok bool) {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), "/")
	if normalized == "" {
		return "", "", false
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" {
		return "", "", false
	}
	return parsed.Scheme, parsed.Host, true
}
