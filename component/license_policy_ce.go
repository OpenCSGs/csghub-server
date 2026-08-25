//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// LicensePolicyComponent in CE is a no-op: there is no license gating, so
// every feature check is permissive and the OpenFeature machinery is not even
// compiled into CE builds.
type LicensePolicyComponent interface {
	CheckFeatureEnabled(ctx context.Context, feature types.FeatureDefinition) error
}

type licensePolicyComponentImpl struct{}

func NewLicensePolicyComponent(_ *config.Config) (LicensePolicyComponent, error) {
	return &licensePolicyComponentImpl{}, nil
}

func (c *licensePolicyComponentImpl) CheckFeatureEnabled(context.Context, types.FeatureDefinition) error {
	return nil
}
