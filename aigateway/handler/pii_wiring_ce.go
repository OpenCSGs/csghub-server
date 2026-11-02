//go:build !ee && !saas

package handler

import (
	"opencsg.com/csghub-server/aigateway/component/pii"
	"opencsg.com/csghub-server/common/config"
)

// newPIIMasker returns the inert CE masker: PII masking ships in the
// EE/SAAS editions only.
func newPIIMasker(cfg *config.Config) pii.PIIMasker {
	return pii.NewPIIMasker(cfg, nil)
}
