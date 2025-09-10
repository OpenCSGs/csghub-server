package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type OrganizationVerifyStoreImpl struct {
	db *DB
}

type OrganizationVerify struct {
	ID                 int64              `bun:",pk,autoincrement" json:"id"`
	Name               string             `bun:"column:path,unique,notnull" json:"path"`
	CompanyName        string             `bun:",notnull" json:"company_name"`
	UnifiedCreditCode  string             `bun:",notnull" json:"unified_credit_code"`
	Username           string             `bun:",notnull" json:"username"`
	ContactName        string             `bun:",notnull" json:"contact_name"`
	ContactEmail       string             `bun:",notnull" json:"contact_email"`
	BusinessLicenseImg string             `bun:",notnull" json:"business_license_img"`
	Status             types.VerifyStatus `bun:",notnull,default:'pending'" json:"status"` // pending, approved, rejected
	Reason             string             `bun:",nullzero" json:"reason,omitempty"`
	UserUUID           string             `bun:"," json:"user_uuid"`

	times
}

type OrganizationVerifyStore interface {
	CreateOrganizationVerify(ctx context.Context, orgVerify *OrganizationVerify) (*OrganizationVerify, error)
	UpdateOrganizationVerify(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*OrganizationVerify, error)
	GetOrganizationVerify(ctx context.Context, path string) (*OrganizationVerify, error)
}

func NewOrganizationVerifyStore() OrganizationVerifyStore {
	return &OrganizationVerifyStoreImpl{
		db: defaultDB,
	}
}

func NewOrganizationVerifyStoreWithDB(db *DB) OrganizationVerifyStore {
	return &OrganizationVerifyStoreImpl{
		db: db,
	}
}

func (ov *OrganizationVerifyStoreImpl) CreateOrganizationVerify(ctx context.Context, orgVerify *OrganizationVerify) (*OrganizationVerify, error) {
	_, err := ov.db.Operator.Core.NewInsert().
		Model(orgVerify).
		On("CONFLICT (path) DO UPDATE").
		Set("company_name = EXCLUDED.company_name").
		Set("unified_credit_code = EXCLUDED.unified_credit_code").
		Set("username = EXCLUDED.username").
		Set("contact_name = EXCLUDED.contact_name").
		Set("contact_email = EXCLUDED.contact_email").
		Set("business_license_img = EXCLUDED.business_license_img").
		Set("status = EXCLUDED.status").
		Set("reason = EXCLUDED.reason").
		Set("user_uuid = EXCLUDED.user_uuid").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert or update organization verify: %w", err)
	}
	return orgVerify, nil
}

func (ov *OrganizationVerifyStoreImpl) UpdateOrganizationVerify(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*OrganizationVerify, error) {
	orgVerify := &OrganizationVerify{
		ID:     id,
		Status: status,
		Reason: reason,
	}

	_, err := ov.db.Operator.Core.NewUpdate().
		Model(orgVerify).
		Column("status", "reason").
		WherePK().
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization verify: %w", err)
	}
	return ov.GetOrganizationVerifyById(ctx, id)
}

func (ov *OrganizationVerifyStoreImpl) GetOrganizationVerifyById(ctx context.Context, id int64) (*OrganizationVerify, error) {
	orgVerify := new(OrganizationVerify)
	err := ov.db.Operator.Core.NewSelect().
		Model(orgVerify).
		Where("id = ?", id).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return orgVerify, nil
}

func (ov *OrganizationVerifyStoreImpl) GetOrganizationVerify(ctx context.Context, path string) (*OrganizationVerify, error) {
	orgVerify := new(OrganizationVerify)
	err := ov.db.Operator.Core.NewSelect().
		Model(orgVerify).
		Where("path = ?", path).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get organization verify: %w", err)
	}
	return orgVerify, nil
}
