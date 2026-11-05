package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type CustomerLicenseStore interface {
	GetCurrentForUser(ctx context.Context, userUUID, product string) (*types.LicenseDownloadData, error)
}

type customerLicenseStore struct{ db *DB }

func NewCustomerLicenseStore() CustomerLicenseStore {
	return NewCustomerLicenseStoreWithDB(defaultDB)
}

func NewCustomerLicenseStoreWithDB(db *DB) CustomerLicenseStore {
	return &customerLicenseStore{db: db}
}

func (s *customerLicenseStore) GetCurrentForUser(ctx context.Context, userUUID, product string) (*types.LicenseDownloadData, error) {
	var license License
	now := time.Now()
	err := s.db.Core.NewSelect().Model(&license).
		Where("user_uuid = ?", userUUID).Where("product = ?", product).
		Where("start_time <= ? AND expire_time > ?", now, now).
		Order("expire_time DESC", "id DESC").Limit(1).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &types.LicenseDownloadData{
		ID: license.ID, Key: license.Key, UserUUID: license.UserUUID,
		DataBody: types.DataBody{
			Company: license.Company, Email: license.Email, Product: license.Product, Edition: license.Edition,
			Version: license.Version, MaxUser: license.MaxUser, StartTime: license.StartTime, ExpireTime: license.ExpireTime, Extra: license.Extra,
		},
	}, nil
}
