package component

import (
	"context"

	"opencsg.com/csghub-server/builder/rsa"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type LicenseComponent interface {
	ListLicense(ctx context.Context, req types.QueryLicenseReq) ([]database.License, int, error)
	CreateLicense(ctx context.Context, req *types.CreateLicenseReq) (string, error)
	ImportLicense(ctx context.Context, req types.ImportLicenseReq) error
	GetLicenseByID(ctx context.Context, req types.GetLicenseReq) (*database.License, string, error)
	UpdateLicense(ctx context.Context, id int64, req *types.UpdateLicenseReq) (*database.License, error)
	GetLicenseStatus(ctx context.Context, req types.LicenseStatusReq) (*types.LicenseStatusResp, error)
	GetLicenseStatusInternal(ctx context.Context) (*types.LicenseStatusResp, error)
	DeleteLicenseByID(ctx context.Context, id int64, currentUser string) error
	VerifyLicense(ctx context.Context, req types.ImportLicenseReq) (*types.RSAPayload, error)
}

type licenseComponentImpl struct {
	publicKeyFile  string
	privateKeyFile string
	userStore      database.UserStore
	licenseStore   database.LicenseStore
	keysReader     rsa.KeysReader
}

var NewLicenseComponent = func(config *config.Config) (LicenseComponent, error) {
	lc := &licenseComponentImpl{
		publicKeyFile:  config.License.PublicKeyFile,
		privateKeyFile: config.License.PrivateKeyFile,
		userStore:      database.NewUserStore(),
		licenseStore:   database.NewLicenseStore(),
		keysReader:     rsa.NewKeysReader(),
	}
	return lc, nil
}
