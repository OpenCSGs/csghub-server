package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accessTokenStoreImpl struct {
	db *DB
}

type AccessTokenStore interface {
	Create(ctx context.Context, token *AccessToken, quota []AccountAccessTokenQuota) error
	// Refresh will disable existing access token, and then generate new one
	Refresh(ctx context.Context, token *AccessToken, newTokenValue string, newExpiredAt time.Time) (*AccessToken, error)
	FindByID(ctx context.Context, id int64) (token *AccessToken, err error)
	// FindByIDs returns the tokens for the given ids, keyed by id; unknown ids
	// are simply absent from the map.
	FindByIDs(ctx context.Context, ids []int64) (map[int64]AccessToken, error)
	Delete(ctx context.Context, username, tkName, app string) (err error)
	IsExist(ctx context.Context, username, tkName, app string) (exists bool, err error)
	FindByUID(ctx context.Context, uid int64) (token *AccessToken, err error)
	GetUserGitToken(ctx context.Context, username string) (*AccessToken, error)
	FindByToken(ctx context.Context, tokenValue, app string) (*AccessToken, error)
	FindByTokenName(ctx context.Context, username, tokenName, app string) (*AccessToken, error)
	FindByUser(ctx context.Context, username, app string) ([]AccessToken, error)
	GetByID(ctx context.Context, id int64) (*AccessToken, error)
	IsExistByUUID(ctx context.Context, uuid string, tkName, app string) (exists bool, err error)
	// UpdateAPIKey updates a gateway API key by id, only works for app=apikey
	UpdateTokenAndQuota(ctx context.Context, key *AccessToken, quota *AccountAccessTokenQuota) (*AccessToken, error)
	// UpdateTokenAndQuotas updates a gateway API key and its multiple quota
	// records in a single transaction.
	UpdateTokenAndQuotas(ctx context.Context, key *AccessToken, quotas []*AccountAccessTokenQuota) (*AccessToken, error)
	// DeleteByID deletes a  API key by id
	DeleteByID(ctx context.Context, id int64) error
	// SetActiveByID enables or disables an API key by id without deleting it.
	SetActiveByID(ctx context.Context, id int64, active bool) error
	// FindByNsUUID finds gateway API keys by namespace uuid
	FindByNsUUID(ctx context.Context, nsUUID string, app string) ([]AccessToken, error)
	// FindBuiltinByNsUUID finds first builtin API key by namespace uuid
	FindBuiltinByNsUUID(ctx context.Context, nsUUID string, app string) (*AccessToken, error)
	// UpdateToken updates a gateway API key token value
	UpdateToken(ctx context.Context, token *AccessToken) error
}

func NewAccessTokenStoreWithDB(db *DB) AccessTokenStore {
	return &accessTokenStoreImpl{db: db}
}

func NewAccessTokenStore() AccessTokenStore {
	return &accessTokenStoreImpl{
		db: defaultDB,
	}
}

type AccessToken struct {
	ID    int64  `bun:",pk,autoincrement" json:"id"`
	GitID int64  `bun:",notnull" json:"git_id"`
	Name  string `bun:",notnull" json:"name"`
	// Token stays plaintext only in memory; EncryptedToken owns the database column.
	Token             string `bun:"-" json:"token"`
	EncryptedToken    string `bun:"column:token,notnull" json:"-"`
	TokenLookupHash   string `bun:",nullzero" json:"-"`
	CredentialVersion int    `bun:",notnull,default:0" json:"-"`
	CredentialKind    string `bun:",notnull,default:''" json:"-"`
	UserID            int64  `bun:",notnull" json:"user_id"`
	User              *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	//example: csghub, starship
	Application types.AccessTokenApp `bun:"column:app," json:"application"`
	Permission  string               `bun:"," json:"permission"`
	IsActive    bool                 `bun:",default:true" json:"is_active"`
	ExpiredAt   time.Time            `bun:",nullzero" json:"expired_at"`
	DeletedAt   time.Time            `bun:",soft_delete,nullzero"`
	// namespace uuid for gateway api key
	NsUUID    string                `bun:",nullzero" json:"ns_uuid"` // ns uuid
	TokenType types.AccessTokenType `bun:",notnull,default:''" json:"token_type"`
	times

	// CreationLimit is an optional, non-persisted namespace cap enforced atomically; nil leaves creation unrestricted.
	CreationLimit *uint `bun:"-" json:"-"`
}

// Create enforces the optional namespace limit and encrypts the token and quota copies before atomic insertion.
// Read Committed ensures the count sees creations committed while waiting for the namespace lock.
func (s *accessTokenStoreImpl) Create(ctx context.Context, token *AccessToken, quotas []AccountAccessTokenQuota) error {
	if err := token.prepareCredentialKind(ctx, s.db.Core); err != nil {
		return err
	}
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx bun.Tx) error {
		if err := checkAccessTokenCreationLimit(ctx, tx, token); err != nil {
			return err
		}
		// Prepare ciphertext only after the creation limit admits this token.
		if err := token.encryptCredentials(); err != nil {
			return err
		}
		// Insert access token
		err := tx.NewInsert().Model(token).Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to create access token, err:%w", err)
		}

		// Insert quota records
		if len(quotas) > 0 {
			// token.ID is populated by the insert above; backfill it into
			// each quota so token_id is written instead of falling back to
			// the column DEFAULT 0.
			for i := range quotas {
				quotas[i].TokenID = token.ID
				if err := quotas[i].encryptCredentials(); err != nil {
					return err
				}
			}
			err = tx.NewInsert().Model(&quotas).Scan(ctx)
			if err != nil {
				return fmt.Errorf("failed to create access token quota, err:%w", err)
			}
		}

		return nil
	})
	if errors.Is(err, errorx.ErrAPIKeyLimitReached) {
		return errorx.ErrAPIKeyLimitReached
	}
	return errorx.HandleDBError(err, nil)
}

// checkAccessTokenCreationLimit serializes custom key creation on the namespace row.
// The lock is held through insertion and commit, including when no keys exist yet.
// Disabled and expired keys count; soft-deleted and builtin keys do not.
// Only keys in the requested namespace count, matching the namespace API key list.
// The inner query stops at the limit instead of counting every matching key.
func checkAccessTokenCreationLimit(ctx context.Context, tx bun.Tx, token *AccessToken) error {
	if token.CreationLimit == nil || token.Application != types.AccessTokenAppAIGateway || token.NsUUID == "" || token.TokenType == types.AccessTokenTypeBuiltIn {
		return nil
	}
	if *token.CreationLimit == 0 {
		return errorx.ErrAPIKeyLimitReached
	}
	var ns Namespace
	if err := tx.NewSelect().Model(&ns).Column("id").Where("uuid = ?", token.NsUUID).For("UPDATE").Scan(ctx); err != nil {
		return fmt.Errorf("lock API key namespace: %w", err)
	}
	keys := tx.NewSelect().Model((*AccessToken)(nil)).ColumnExpr("1").
		Where("app = ?", types.AccessTokenAppAIGateway).
		Where("token_type != ?", types.AccessTokenTypeBuiltIn).
		Where("ns_uuid = ?", token.NsUUID)
	count, err := tx.NewSelect().TableExpr("(? LIMIT ?) AS limited_keys", keys, *token.CreationLimit).Count(ctx)
	if err != nil {
		return fmt.Errorf("count namespace API keys: %w", err)
	}
	if uint(count) >= *token.CreationLimit {
		return errorx.ErrAPIKeyLimitReached
	}
	return nil
}

// Refresh will disable existing access token, and then generate new one
func (s *accessTokenStoreImpl) Refresh(ctx context.Context, token *AccessToken, newTokenValue string, newExpiredAt time.Time) (*AccessToken, error) {
	if err := token.prepareCredentialKind(ctx, s.db.Core); err != nil {
		return nil, err
	}
	var newToken *AccessToken
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().Model(token).
			Set("is_active = false").
			Where("user_id = ? and name = ? and app = ? and (is_active is null or is_active = true) ", token.UserID, token.Name, token.Application).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to disable old token, err:%w", err)
		}

		// make a copy
		newToken = &AccessToken{
			CredentialKind: token.CredentialKind,
			Name:           token.Name,
			Token:          newTokenValue,
			UserID:         token.UserID,
			Application:    token.Application,
			Permission:     token.Permission,
			IsActive:       true,
		}
		if newExpiredAt.After(time.Now()) {
			newToken.ExpiredAt = newExpiredAt
		} else {
			//don't change old key's expire time
			newToken.ExpiredAt = token.ExpiredAt
		}
		if err := newToken.encryptCredentials(); err != nil {
			return err
		}
		_, err = tx.NewInsert().Model(newToken).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed insert new token, err:%w", err)
		}

		return nil
	})

	newToken.User = token.User
	return newToken, errorx.HandleDBError(err, nil)
}

// FindByID explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) FindByID(ctx context.Context, id int64) (*AccessToken, error) {
	var token AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("access_token.id = ?", id).
		Scan(ctx)
	if err == nil {
		err = token.decryptCredentials()
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

// FindByIDs returns the tokens for the given ids, keyed by id; unknown ids are
// simply absent from the map.
func (s *accessTokenStoreImpl) FindByIDs(ctx context.Context, ids []int64) (map[int64]AccessToken, error) {
	result := make(map[int64]AccessToken, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var tokens []AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Where("access_token.id IN (?)", bun.In(ids)).
		Scan(ctx)
	if err == nil {
		err = decryptAccessTokens(tokens)
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	for _, token := range tokens {
		result[token.ID] = token
	}
	return result, nil
}

func (s *accessTokenStoreImpl) Delete(ctx context.Context, username, tkName, app string) (err error) {
	var token AccessToken
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&token).
		TableExpr("users AS u").
		Where("access_token.user_id = u.id").
		Where("u.username = ?", username).
		Where("access_token.name = ? and app = ?", tkName, app).
		ForceDelete().
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *accessTokenStoreImpl) IsExist(ctx context.Context, username, tkName, app string) (exists bool, err error) {
	var token AccessToken
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Join("JOIN users AS u ON u.id = access_token.user_id").
		Where("u.username = ?", username).
		Where("access_token.name = ? and app = ?", tkName, app).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

// FindByUID explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) FindByUID(ctx context.Context, uid int64) (token *AccessToken, err error) {
	var tokens []AccessToken
	err = s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Relation("User").
		Where("user_id = ?", uid).
		Where("app = ?", "git").
		Where("is_active = true and (expired_at is null or expired_at > ?)", time.Now()).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err == nil {
		err = decryptAccessTokens(tokens)
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	if len(tokens) == 0 {
		return nil, errorx.HandleDBError(sql.ErrNoRows, nil)
	}
	token = &tokens[0]
	return
}

// GetUserGitToken explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) GetUserGitToken(ctx context.Context, username string) (*AccessToken, error) {
	var token AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Join("JOIN users AS u ON u.id = access_token.user_id").
		Where("u.username = ?", username).
		Where("access_token.app = ?", "git").
		Where("is_active = true and (access_token.expired_at is null or access_token.expired_at > ?)", time.Now()).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err == nil {
		err = token.decryptCredentials()
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

const organizationAPIKeyPredicate = `app = ? AND (credential_kind = 'organization-api-key' OR EXISTS (
	SELECT 1 FROM namespaces AS n WHERE n.uuid = access_token.ns_uuid AND n.namespace_type = ?
))`

// FindByToken explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) FindByToken(ctx context.Context, tokenValue, app string) (*AccessToken, error) {
	digest, err := CredentialLookupHash(tokenValue)
	if err != nil {
		return nil, err
	}
	var token AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("(token_lookup_hash = ? OR (credential_version = 0 AND token = ?)) AND is_active = true", digest, tokenValue).
		Where(`"user".login_frozen IS NOT TRUE OR (`+organizationAPIKeyPredicate+")", types.AccessTokenAppAIGateway, OrgNamespace)
	if len(app) > 0 {
		q = q.Where("app = ?", app)
	}
	err = q.Scan(ctx)
	if err == nil {
		err = token.decryptCredentials()
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

// FindByTokenName explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) FindByTokenName(ctx context.Context, username, tokenName, app string) (*AccessToken, error) {
	var token AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("access_token.name = ? and app = ? and is_active = true and username = ?", tokenName, app, username)
	err := q.Scan(ctx)
	if err == nil {
		err = token.decryptCredentials()
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

// FindByUser explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) FindByUser(ctx context.Context, username, app string) ([]AccessToken, error) {
	var tokens []AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Relation("User").
		Where("is_active = true and username = ?", username).
		Order("created_at DESC")
	if len(app) > 0 {
		q = q.Where("app = ?", app)
	}
	err := q.Scan(ctx)
	if err == nil {
		err = decryptAccessTokens(tokens)
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return tokens, nil
}

// GetByID explicitly restores access token credentials after database reads.
func (s *accessTokenStoreImpl) GetByID(ctx context.Context, id int64) (*AccessToken, error) {
	token := &AccessToken{}
	err := s.db.Operator.Core.NewSelect().Model(token).WhereAllWithDeleted().Where("id = ?", id).Scan(ctx, token)
	if err == nil {
		err = token.decryptCredentials()
	}
	return token, errorx.HandleDBError(err, nil)
}

func (s *accessTokenStoreImpl) IsExistByUUID(ctx context.Context, uuid string, tkName, app string) (exists bool, err error) {
	var token AccessToken
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Where("ns_uuid = ?", uuid).
		Where("name = ? and app = ?", tkName, app).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

// UpdateTokenAndQuota updates a access token by id, only works for app=apikey
func (s *accessTokenStoreImpl) UpdateTokenAndQuota(ctx context.Context, key *AccessToken, quota *AccountAccessTokenQuota) (*AccessToken, error) {
	if err := key.prepareCredentialKind(ctx, s.db.Core); err != nil {
		return nil, err
	}
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := key.encryptCredentials(); err != nil {
			return err
		}
		// Update access token
		_, err := tx.NewUpdate().Model(key).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update api key, err:%w", err)
		}

		// Update quota record
		if quota != nil {
			if err := quota.encryptCredentials(); err != nil {
				return err
			}
			_, err = tx.NewUpdate().Model(quota).WherePK().Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to update api key quota, err:%w", err)
			}
		}

		return nil
	})

	return key, errorx.HandleDBError(err, nil)
}

// UpdateTokenAndQuotas updates an access token and multiple quota records in
// a single transaction. Each quota must carry its own primary key (ID).
func (s *accessTokenStoreImpl) UpdateTokenAndQuotas(ctx context.Context, key *AccessToken, quotas []*AccountAccessTokenQuota) (*AccessToken, error) {
	if err := key.prepareCredentialKind(ctx, s.db.Core); err != nil {
		return nil, err
	}
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := key.encryptCredentials(); err != nil {
			return err
		}
		// Update access token
		_, err := tx.NewUpdate().Model(key).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update api key, err:%w", err)
		}

		for _, quota := range quotas {
			if quota == nil {
				continue
			}
			if err := quota.encryptCredentials(); err != nil {
				return err
			}
			if quota.ID == 0 {
				if err := tx.NewInsert().Model(quota).Scan(ctx); err != nil {
					return fmt.Errorf("failed to create api key quota, err:%w", err)
				}
			} else {
				_, err = tx.NewUpdate().Model(quota).WherePK().Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to update api key quota, err:%w", err)
				}
			}
		}
		var tokenQuotas []AccountAccessTokenQuota
		err = tx.NewSelect().Model(&tokenQuotas).Where("token_id = ?", key.ID).Scan(ctx)
		if err == nil {
			err = decryptQuotaCredentials(tokenQuotas)
		}
		if err != nil {
			return fmt.Errorf("failed to get api key quotas, err:%w", err)
		}
		if len(tokenQuotas) < 1 {
			return nil
		}
		var quotaItems []types.UpdateAPIKeyQuotaItem
		for _, quota := range tokenQuotas {
			quotaItems = append(quotaItems, types.UpdateAPIKeyQuotaItem{
				QuotaType: types.AccountingQuotaType(quota.QuotaType),
				ValueType: types.AccountingQuotaValueType(quota.ValueType),
			})
		}
		if err := types.CheckQuotaSet(quotaItems); err != nil {
			return err
		}

		return nil
	})

	return key, errorx.HandleDBError(err, nil)
}

// DeleteByID soft deletes a access token by id
func (s *accessTokenStoreImpl) DeleteByID(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.
		NewUpdate().
		Model(&AccessToken{}).
		Set("is_active = false").
		Set("deleted_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// SetActiveByID enables (active=true) or disables (active=false) an API key by
// id. Disabled keys are kept but rejected by FindByToken, so they can be
// re-enabled later; this is distinct from DeleteByID, which soft-deletes the row.
func (s *accessTokenStoreImpl) SetActiveByID(ctx context.Context, id int64, active bool) error {
	_, err := s.db.Operator.Core.
		NewUpdate().
		Model(&AccessToken{}).
		Set("is_active = ?", active).
		Where("id = ?", id).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// FindAPIKeyByNsUUID finds gateway API keys by namespace uuid
func (s *accessTokenStoreImpl) FindByNsUUID(ctx context.Context, nsUUID string, app string) ([]AccessToken, error) {
	var tokens []AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Where("app = ? and ns_uuid = ?", app, nsUUID).
		Order("token_type ASC").
		Order("id DESC").
		Scan(ctx)
	if err == nil {
		err = decryptAccessTokens(tokens)
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return tokens, nil
}

// FindBuiltinByNsUUID finds first builtin API key by namespace uuid
func (s *accessTokenStoreImpl) FindBuiltinByNsUUID(ctx context.Context, nsUUID string, app string) (*AccessToken, error) {
	var token AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Where("app = ? and ns_uuid = ? and token_type = ? and is_active = true", app, nsUUID, types.AccessTokenTypeBuiltIn).
		Limit(1).
		Scan(ctx)
	if err == nil {
		err = token.decryptCredentials()
	}
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

// UpdateToken refreshes a gateway API key and its quota credential copies in
// one transaction. Quotas remain associated through the stable token_id; only
// credential columns are synchronized so concurrent usage, settings and timestamps
// cannot be overwritten by the quota snapshots read during refresh.
func (s *accessTokenStoreImpl) UpdateToken(ctx context.Context, token *AccessToken) error {
	if err := token.prepareCredentialKind(ctx, s.db.Core); err != nil {
		return err
	}
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := token.encryptCredentials(); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model(token).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update access token, err:%w", err)
		}

		// Sync the api_key on quota rows so they stay joined to the refreshed
		// key value. token_id is the stable join key and never changes here.
		var quotas []AccountAccessTokenQuota
		err = tx.NewSelect().Model(&quotas).Where("token_id = ?", token.ID).Scan(ctx)
		if err == nil {
			err = decryptQuotaCredentials(quotas)
		}
		if err == nil {
			for i := range quotas {
				quotas[i].APIKey = token.Token
				if err = quotas[i].encryptCredentials(); err != nil {
					break
				}
				// Persist only credentials, preserving concurrently updated quota
				// business fields and timestamps.
				_, err = tx.NewUpdate().Model(&quotas[i]).
					Column("api_key", "credential_version").
					WherePK().Exec(ctx)
				if err != nil {
					break
				}
			}
		}
		if err != nil {
			return fmt.Errorf("failed to sync api_key on access token quota, err:%w", err)
		}

		return nil
	})
	return errorx.HandleDBError(err, nil)
}

// prepareCredentialKind persists the personal/organization distinction so later
// membership or namespace changes cannot change the key needed for decryption.
func (t *AccessToken) prepareCredentialKind(ctx context.Context, db bun.IDB) error {
	if t.CredentialKind != "" {
		return nil
	}
	t.CredentialKind = string(t.Application)
	if t.Application != types.AccessTokenAppAIGateway {
		return nil
	}
	t.CredentialKind = "personal-api-key"
	if t.NsUUID == "" {
		return nil
	}
	var ns Namespace
	err := db.NewSelect().Model(&ns).WhereAllWithDeleted().Where("uuid = ?", t.NsUUID).Limit(1).Scan(ctx)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && ns.NamespaceType == OrgNamespace {
		t.CredentialKind = "organization-api-key"
	}
	return nil
}

// encryptCredentials prepares storage fields explicitly before a credential write.
// Plaintext remains available in memory for existing callers and create responses.
func (t *AccessToken) encryptCredentials() error {
	if t.Token == "" {
		return nil
	}
	if t.CredentialKind == "" {
		t.CredentialKind = string(t.Application)
	}
	var err error
	t.EncryptedToken, err = encryptStoredCredential(t.Token, "access-token/"+t.CredentialKind, credentialOwner(t.UserID), t.NsUUID)
	if err != nil {
		return err
	}
	t.TokenLookupHash, err = CredentialLookupHash(t.Token)
	if err != nil {
		return err
	}
	t.CredentialVersion = credentialEncryptionVersion
	return nil
}

// decryptCredentials restores stored values explicitly after a successful query.
func (t *AccessToken) decryptCredentials() error {
	var err error
	t.Token, err = decryptStoredCredential(t.EncryptedToken, t.CredentialVersion, "access-token/"+t.CredentialKind, credentialOwner(t.UserID), t.NsUUID)
	return err
}

// decryptAccessTokens restores credentials in the rows loaded by an explicit store query.
func decryptAccessTokens(rows []AccessToken) error {
	for i := range rows {
		if err := rows[i].decryptCredentials(); err != nil {
			return err
		}
	}
	return nil
}
