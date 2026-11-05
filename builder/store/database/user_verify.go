package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type UserVerifyStoreImpl struct {
	db *DB
}

type UserVerifyStore interface {
	CreateUserVerify(ctx context.Context, user *UserVerify) (*UserVerify, error)
	UpdateUserVerify(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*UserVerify, error)
	GetUserVerify(ctx context.Context, uuid string) (*UserVerify, error)
}

func NewUserVerifyStore() UserVerifyStore {
	return &UserVerifyStoreImpl{
		db: defaultDB,
	}
}

func NewUserVerifyStoreWithDB(db *DB) UserVerifyStore {
	return &UserVerifyStoreImpl{
		db: db,
	}
}

// UserVerify stores the current manual or external identity verification for a user.
type UserVerify struct {
	Provider          string             `bun:",notnull,default:'manual'" json:"provider"`
	VerificationID    string             `bun:",nullzero" json:"-"`
	IDNumberEncrypted string             `bun:",nullzero,type:text" json:"-"`
	IDNumberHash      string             `bun:",nullzero" json:"-"`
	ID                int64              `bun:",pk,autoincrement" json:"id"`
	UUID              string             `bun:",unique,notnull" json:"uuid"`
	RealName          string             `bun:",notnull" json:"real_name"`
	Username          string             `bun:",notnull" json:"username"`
	IDCardFront       string             `bun:",nullzero" json:"id_card_front"`
	IDCardBack        string             `bun:",nullzero" json:"id_card_back"`
	Status            types.VerifyStatus `bun:",notnull,default:'pending'" json:"status"` // pending, approved, rejected
	// Reason is manual review text or a USER-ERR code for an external verification result.
	Reason string `bun:",nullzero" json:"reason,omitempty" i18n:"error"`
	// IDNumberMasked is a response-only value populated by SaaS verification queries, never persisted.
	IDNumberMasked string `bun:"-" json:"id_number,omitempty"`
	times
}

// MaskedRealName preserves the first Unicode character and masks the remaining name.
func (v UserVerify) MaskedRealName() string {
	name := []rune(v.RealName)
	if len(name) == 0 {
		return ""
	}
	return string(name[0]) + strings.Repeat("*", len(name)-1)
}

// MarshalJSON masks external identity names in legacy endpoints that serialize this model.
// Manual review retains its existing representation; sensitive new columns are always excluded.
func (v UserVerify) MarshalJSON() ([]byte, error) {
	type response UserVerify
	value := response(v)
	if v.Provider != "" && v.Provider != "manual" {
		value.RealName = v.MaskedRealName()
	}
	return json.Marshal(value)
}

// CreateUserVerify upserts manual material without replacing an external flow.
// Refresh the generation explicitly because PostgreSQL CURRENT_TIMESTAMP is fixed within a transaction.
func (uv *UserVerifyStoreImpl) CreateUserVerify(ctx context.Context, userVerify *UserVerify) (*UserVerify, error) {
	userVerify.UpdatedAt = time.Now().Truncate(time.Microsecond)
	result, err := uv.db.Operator.Core.NewInsert().
		Model(userVerify).
		On("CONFLICT (uuid) DO UPDATE").
		Set("real_name = EXCLUDED.real_name").
		Set("username = EXCLUDED.username").
		Set("id_card_front = EXCLUDED.id_card_front").
		Set("id_card_back = EXCLUDED.id_card_back").
		Set("status = EXCLUDED.status").
		Set("reason = EXCLUDED.reason").
		Set("updated_at = EXCLUDED.updated_at").
		Where("user_verify.provider = ?", "manual").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert or update user verify: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("retrieving affected row count: %w", err)
	}
	if affected == 0 {
		return nil, ErrIdentityConflict
	}
	if affected != 1 {
		return nil, fmt.Errorf("affected %d row(s), want 1", affected)
	}
	return userVerify, nil
}

// UpdateUserVerify completes one pending manual review and mirrors user status in the same transaction.
// Terminal, replaced and external records conflict so only one reviewer can dispatch completion effects.
func (uv *UserVerifyStoreImpl) UpdateUserVerify(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*UserVerify, error) {
	userVerify, err := uv.GetUserVerifyById(ctx, id)
	if errors.Is(err, errorx.ErrDatabaseNoRows) || errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIdentityConflict
	}
	if err != nil {
		return nil, err
	}
	if userVerify.Provider != "manual" {
		return nil, ErrIdentityConflict
	}
	return NewIdentityStoreWithDB(uv.db).complete(ctx, userVerify, status, reason)
}

func (uv *UserVerifyStoreImpl) GetUserVerifyById(ctx context.Context, id int64) (*UserVerify, error) {
	userVerify := new(UserVerify)
	err := uv.db.Operator.Core.NewSelect().
		Model(userVerify).
		Where("id = ?", id).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return userVerify, nil
}

func (uv *UserVerifyStoreImpl) GetUserVerify(ctx context.Context, uuid string) (*UserVerify, error) {
	userVerify := new(UserVerify)
	err := uv.db.Operator.Core.NewSelect().
		Model(userVerify).
		Where("uuid = ?", uuid).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get organization verify: %w", err)
	}
	return userVerify, nil
}

// ErrIdentityConflict identifies guarded manual writes, approved verification or a changed generation.
var ErrIdentityConflict = errors.New("identity verification already approved or generation changed")

// IdentityStore persists external verification with optimistic generation checks.
// UpdatedAt is the generation timestamp while pending; attaching an ID preserves it.
type IdentityStore struct{ db *DB }

// NewIdentityStore creates the production verification store.
func NewIdentityStore() *IdentityStore { return NewIdentityStoreWithDB(defaultDB) }

// NewIdentityStoreWithDB uses a caller-owned database, including test databases.
func NewIdentityStoreWithDB(db *DB) *IdentityStore { return &IdentityStore{db: db} }

// Get returns only the specified user's current verification.
func (s *IdentityStore) Get(ctx context.Context, uuid string) (*UserVerify, error) {
	return (&UserVerifyStoreImpl{db: s.db}).GetUserVerify(ctx, uuid)
}

// Begin reserves a generation under a user lock and synchronizes the user status.
// Every submission replaces an unfinished generation, including identical pending data.
func (s *IdentityStore) Begin(ctx context.Context, proposed *UserVerify) (*UserVerify, error) {
	var result *UserVerify
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		user := new(User)
		if err := tx.NewSelect().Model(user).Where("uuid = ?", proposed.UUID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if user.VerifyStatus == types.VerifyStatusApproved {
			return ErrIdentityConflict
		}
		existing := new(UserVerify)
		err := tx.NewSelect().Model(existing).Where("uuid = ?", proposed.UUID).For("UPDATE").Scan(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && existing.Status == types.VerifyStatusApproved {
			return ErrIdentityConflict
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		// Keep generations distinct even within one clock tick or after a clock adjustment.
		if !now.After(existing.UpdatedAt) {
			now = existing.UpdatedAt.Add(time.Microsecond)
		}
		proposed.Status = types.VerifyStatusPending
		proposed.VerificationID = ""
		proposed.Reason = ""
		proposed.IDCardFront, proposed.IDCardBack = "", ""
		proposed.Username = user.Username
		proposed.UpdatedAt = now
		if existing.ID == 0 {
			proposed.CreatedAt = now
			if _, err = tx.NewInsert().Model(proposed).Returning("*").Exec(ctx); err != nil {
				return err
			}
		} else {
			proposed.ID = existing.ID
			// Raw updates intentionally preserve the explicitly selected generation timestamp.
			_, err = tx.NewUpdate().Table("user_verifies").
				Set("provider = ?", proposed.Provider).Set("verification_id = NULL").
				Set("real_name = ?", proposed.RealName).Set("username = ?", proposed.Username).
				Set("id_number_encrypted = ?", proposed.IDNumberEncrypted).Set("id_number_hash = ?", proposed.IDNumberHash).
				Set("status = ?", proposed.Status).Set("reason = NULL").Set("id_card_front = NULL").Set("id_card_back = NULL").
				Set("updated_at = ?", now).Where("id = ?", existing.ID).Exec(ctx)
			if err != nil {
				return err
			}
			proposed.CreatedAt = existing.CreatedAt
		}
		if _, err := tx.NewUpdate().Table("users").Set("verify_status = ?", proposed.Status).Where("uuid = ?", proposed.UUID).Exec(ctx); err != nil {
			return err
		}
		result = proposed
		return nil
	})
	return result, err
}

// Attach records the provider ID only if initialization still owns the generation.
func (s *IdentityStore) Attach(ctx context.Context, record *UserVerify, id string) error {
	result, err := s.db.Core.NewUpdate().Table("user_verifies").Set("verification_id = ?", id).
		Where("id = ? AND uuid = ? AND provider = ?", record.ID, record.UUID, record.Provider).
		Where("status = ? AND updated_at = ?", types.VerifyStatusPending, record.UpdatedAt).
		Where("verification_id IS NULL OR verification_id = ? OR verification_id = ''", id).Exec(ctx)
	if err != nil {
		return err
	}
	if err := assertAffectedOneRow(result, nil); err != nil {
		return ErrIdentityConflict
	}
	return nil
}

// Finish commits an authoritative terminal result with a generation/provider/ID guard.
func (s *IdentityStore) Finish(ctx context.Context, record *UserVerify, status types.VerifyStatus, reason string) error {
	_, err := s.complete(ctx, record, status, reason)
	return err
}

// complete returns the committed snapshot only to the request that wins the pending-to-terminal transition.
// It does not mutate the supplied snapshot, which may also be used by concurrent or delayed requests.
func (s *IdentityStore) complete(ctx context.Context, record *UserVerify, status types.VerifyStatus, reason string) (*UserVerify, error) {
	if status != types.VerifyStatusApproved && status != types.VerifyStatusRejected {
		return nil, ErrIdentityConflict
	}
	completedAt := time.Now().Truncate(time.Microsecond)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Use the same lock order as Begin to serialize state changes across replicas.
		var id int64
		if err := tx.NewSelect().Table("users").Column("id").Where("uuid = ?", record.UUID).For("UPDATE").Scan(ctx, &id); err != nil {
			return err
		}
		result, err := tx.NewUpdate().Table("user_verifies").Set("status = ?", status).Set("reason = ?", reason).Set("updated_at = ?", completedAt).
			Where("id = ? AND uuid = ? AND provider = ?", record.ID, record.UUID, record.Provider).
			Where("status = ? AND updated_at = ?", types.VerifyStatusPending, record.UpdatedAt).
			Where("COALESCE(verification_id, '') = ?", record.VerificationID).Exec(ctx)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("retrieving affected row count: %w", err)
		}
		if affected == 0 {
			return ErrIdentityConflict
		}
		if affected != 1 {
			return fmt.Errorf("affected %d row(s), want 1", affected)
		}
		_, err = tx.NewUpdate().Table("users").Set("verify_status = ?", status).Where("uuid = ?", record.UUID).Exec(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	completed := *record
	completed.Status, completed.Reason, completed.UpdatedAt = status, reason, completedAt
	return &completed, nil
}
