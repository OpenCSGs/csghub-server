package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func requireRoute(t *testing.T, routes gin.RoutesInfo, method, path string) {
	t.Helper()

	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}

	require.Failf(t, "route not found", "expected route %s %s to be registered", method, path)
}

func assertNoRoute(t *testing.T, routes gin.RoutesInfo, method, path string) {
	t.Helper()

	for _, route := range routes {
		if route.Method == method && route.Path == path {
			require.Failf(t, "unexpected route found", "did not expect route %s %s to be registered", method, path)
		}
	}
}
