package handler

import (
	"fmt"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type LicenseHandler struct {
	lc component.LicenseComponent
}

func NewLicenseHandler(config *config.Config) (*LicenseHandler, error) {
	lc, err := component.NewLicenseComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create license component, err: %w", err)
	}
	return &LicenseHandler{lc: lc}, nil
}
