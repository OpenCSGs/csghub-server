//go:build ee || (saas && !license_issuer)

package middleware

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/feature"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

// NewFeatureGate builds the license-backed feature gate factory: it creates
// the edition's LicensePolicyComponent and returns a function that attaches a
// per-route gate for a given feature definition.
func NewFeatureGate(config *config.Config) (func(types.FeatureDefinition) gin.HandlerFunc, error) {
	policy, err := component.NewLicensePolicyComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating license policy: %w", err)
	}
	return func(def types.FeatureDefinition) gin.HandlerFunc {
		return FeatureGate(policy, def)
	}, nil
}

func withEvaluationContext(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	targetingKey := httpbase.GetCurrentUserUUID(c)
	if targetingKey == "" {
		targetingKey = username
	}
	ctx := feature.WithEvaluationContext(c.Request.Context(), targetingKey, map[string]any{
		"username": username,
		"authType": string(httpbase.GetAuthType(c)),
	})
	c.Request = c.Request.WithContext(ctx)
}

// FeatureGate returns a Gin middleware that rejects the request with 403
// Forbidden when the given feature is disabled by the license policy. Attach
// it per route, after auth/authorization middlewares, so unauthenticated or
// unauthorized callers cannot probe license entitlements through the
// response.
func FeatureGate(policy component.LicensePolicyComponent, def types.FeatureDefinition) gin.HandlerFunc {
	return func(c *gin.Context) {
		withEvaluationContext(c)
		if err := policy.CheckFeatureEnabled(c.Request.Context(), def); err != nil {
			if errors.Is(err, errorx.ErrLicenseFeatureDisabled) {
				httpbase.ForbiddenError(c, errorx.ErrLicenseFeatureDisabled)
			} else {
				slog.ErrorContext(c.Request.Context(), "failed to check feature policy", slog.String("flag", def.Key), slog.Any("error", err))
				httpbase.ServerError(c, err)
			}
			c.Abort()
			return
		}

		c.Next()
	}
}
