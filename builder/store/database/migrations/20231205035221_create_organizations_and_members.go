package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type Organization struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	Nickname string `bun:"name,notnull" json:"name"`
	// unique name of the organization
	Name         string             `bun:"path,notnull" json:"path"`
	GitPath      string             `bun:",notnull" json:"git_path"`
	Description  string             `json:"description"`
	UserID       int64              `bun:",notnull" json:"user_id"`
	Homepage     string             `bun:"" json:"homepage,omitempty"`
	Logo         string             `bun:"" json:"logo,omitempty"`
	Verified     bool               `bun:"" json:"verified"`
	OrgType      string             `bun:"" json:"org_type"`
	User         *User              `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceID  int64              `bun:",notnull" json:"namespace_id"`
	Namespace    *Namespace         `bun:"rel:has-one,join:namespace_id=id" json:"namespace"`
	VerifyStatus types.VerifyStatus `bun:",notnull,default:'none'" json:"verify_status"` // none, pending, approved, rejected
	times
}

type Member struct {
	ID             int64         `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64         `bun:",pk" json:"organization_id"`
	UserID         int64         `bun:",pk" json:"user_id"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization"`
	User           *User         `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Role           string        `bun:",notnull" json:"role"`
	DeletedAt      time.Time     `bun:",soft_delete,nullzero"`
	times
}

var orgTables = []any{
	Organization{},
	Member{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, orgTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, orgTables...)
	})
}
