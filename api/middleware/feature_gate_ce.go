//go:build (!ee && !saas) || license_issuer

package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

// CE and license-issuing SaaS variants have no license gating: the feature
// gate middlewares are no-op passthroughs so route registration stays uniform
// across editions.

// NewFeatureGate returns a factory whose gates all pass through.
func NewFeatureGate(_ *config.Config) (func(types.FeatureDefinition) gin.HandlerFunc, error) {
	return func(types.FeatureDefinition) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Next()
		}
	}, nil
}

func FeatureGate(_ component.LicensePolicyComponent, _ types.FeatureDefinition) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
