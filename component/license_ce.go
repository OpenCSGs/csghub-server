//go:build !saas && !ee

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *licenseComponentImpl) ListLicense(ctx context.Context, req types.QueryLicenseReq) ([]database.License, int, error) {
	return nil, 0, nil
}

func (c *licenseComponentImpl) CreateLicense(ctx context.Context, req *types.CreateLicenseReq) (string, error) {
	return "", nil
}

func (c *licenseComponentImpl) GetLicenseByID(ctx context.Context, req types.GetLicenseReq) (*database.License, string, error) {
	return nil, "", nil
}

func (c *licenseComponentImpl) UpdateLicense(ctx context.Context, id int64, req *types.UpdateLicenseReq) (*database.License, error) {
	return nil, nil
}

func (c *licenseComponentImpl) DeleteLicenseByID(ctx context.Context, id int64, currentUser string) error {
	return nil
}

func (c *licenseComponentImpl) GetLicenseStatus(ctx context.Context, req types.LicenseStatusReq) (*types.LicenseStatusResp, error) {
	return nil, nil
}

func (c *licenseComponentImpl) GetLicenseStatusInternal(ctx context.Context) (*types.LicenseStatusResp, error) {
	return nil, nil
}

func (c *licenseComponentImpl) ImportLicense(ctx context.Context, req types.ImportLicenseReq) error {
	return nil
}

func (c *licenseComponentImpl) VerifyLicense(ctx context.Context, req types.ImportLicenseReq) (*types.RSAPayload, error) {
	return nil, nil
}
