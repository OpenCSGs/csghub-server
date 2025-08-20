package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type OrganizationVerify struct {
	ID                 int64  `bun:",pk,autoincrement" json:"id"`
	Name               string `bun:"column:path,unique,notnull" json:"path"`
	CompanyName        string `bun:",notnull" json:"company_name"`
	UnifiedCreditCode  string `bun:",notnull" json:"unified_credit_code"`
	Username           string `bun:",notnull" json:"username"`
	ContactName        string `bun:",notnull" json:"contact_name"`
	ContactEmail       string `bun:",notnull" json:"contact_email"`
	BusinessLicenseImg string `bun:",notnull" json:"business_license_img"`
	Status             string `bun:",notnull,default:'pending'" json:"status"` // pending, approved, rejected
	Reason             string `bun:",nullzero" json:"reason,omitempty"`

	times
}

type UserVerify struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	UUID        string `bun:",unique,notnull" json:"uuid"`
	RealName    string `bun:",notnull" json:"real_name"`
	Username    string `bun:",notnull" json:"username"`
	IDCardFront string `bun:",notnull" json:"id_card_front"`
	IDCardBack  string `bun:",notnull" json:"id_card_back"`
	Status      string `bun:",notnull,default:'pending'" json:"status"` // pending, approved, rejected
	Reason      string `bun:",nullzero" json:"reason,omitempty"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, OrganizationVerify{}, UserVerify{})
		if err != nil {
			return fmt.Errorf("create tables failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*OrganizationVerify)(nil)).
			Index("idx_unique_organization_verify_path").
			Column("path").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index for OrganizationVerify failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*UserVerify)(nil)).
			Index("idx_unique_user_verify_uuid").
			Column("uuid").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index for UserVerify failed: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, OrganizationVerify{}, UserVerify{})
	})
}
