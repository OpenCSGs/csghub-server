package migrations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/cryptox"
)

// credentialMigrationVersion freezes the format independently of runtime models.
const credentialMigrationVersion = 1

// credentialMigrationBatchSize limits memory without committing partial batches.
const credentialMigrationBatchSize = 250

// credentialMigrationSource names only the six credentials and the API-key copy
// already stored on quota records. All SQL fragments are fixed migration constants.
type credentialMigrationSource struct {
	table   string
	column  string
	purpose string
	columns string
}

// credentialMigrationSources preserves the existing v1 key scopes and bearer values.
var credentialMigrationSources = []credentialMigrationSource{
	{"access_tokens", "token", "access-token/", `id, token AS value,
 CAST(user_id AS TEXT) AS owner, COALESCE(ns_uuid, '') AS namespace,
 CASE WHEN credential_kind <> '' THEN credential_kind
 WHEN app = 'aigateway' THEN CASE WHEN EXISTS (
 SELECT 1 FROM namespaces n WHERE n.uuid = access_tokens.ns_uuid AND n.namespace_type = 'organization'
 ) THEN 'organization-api-key' ELSE 'personal-api-key' END
 ELSE COALESCE(app, '') END AS kind`},
	{"account_access_token_quota", "api_key", "api-key-quota", "id, api_key AS value, CAST(token_id AS TEXT) AS owner"},
	{"ai_gateway_upstreams", "auth_header", "upstream-api-key", "id, auth_header AS value, CAST(llm_config_id AS TEXT) AS owner"},
}

// credentialMigrationRow contains only the stored value and stable derivation scope.
type credentialMigrationRow struct {
	ID        int64
	Value     sql.NullString
	Owner     string
	Namespace string
	Kind      string
}

// credentialMigrationCipher holds the shared cipher and separate lookup HMAC key.
type credentialMigrationCipher struct {
	cipher *cryptox.Cipher
	lookup []byte
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return migrateCredentialStorage(ctx, db, true)
	}, func(ctx context.Context, db *bun.DB) error {
		return migrateCredentialStorage(ctx, db, false)
	})
}

// migrateCredentialStorage changes only credential fields and their required
// metadata. Each direction uses one transaction and never rotates bearer values.
func migrateCredentialStorage(ctx context.Context, db *bun.DB, encrypt bool) error {
	keys, err := newCredentialMigrationCipher()
	if err != nil {
		return err
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if encrypt {
			for _, statement := range []string{
				"ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS credential_version INTEGER NOT NULL DEFAULT 0",
				"ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS token_lookup_hash VARCHAR",
				"ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS credential_kind VARCHAR NOT NULL DEFAULT ''",
				"ALTER TABLE account_access_token_quota ADD COLUMN IF NOT EXISTS credential_version INTEGER NOT NULL DEFAULT 0",
				"ALTER TABLE ai_gateway_upstreams ADD COLUMN IF NOT EXISTS credential_version INTEGER NOT NULL DEFAULT 0",
			} {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
		}
		for _, source := range credentialMigrationSources {
			if err := rewriteMigrationCredentials(ctx, tx, keys, source, encrypt); err != nil {
				return err
			}
		}
		if encrypt {
			_, err := tx.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_access_tokens_lookup_hash ON access_tokens(token_lookup_hash)")
			return err
		}
		// Down retains metadata columns and the index for retry compatibility.
		return nil
	})
}

// newCredentialMigrationCipher freezes the v1 configuration while reusing cryptox
// for RFC 5869 HKDF and AES-256-GCM, without importing runtime database helpers.
func newCredentialMigrationCipher() (*credentialMigrationCipher, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	master, err := base64.StdEncoding.DecodeString(cfg.Credential.MasterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode credential migration master key: %w", err)
	}
	cipher, err := cryptox.NewCipher(cryptox.Config{
		MasterKey: master, Salt: []byte("csghub/credentials/storage/v1/salt"), InfoPrefix: "csghub/credentials/storage/",
	})
	if err != nil {
		return nil, err
	}
	h := hmac.New(sha256.New, master)
	_, _ = h.Write([]byte("csghub/credentials/lookup/v1"))
	return &credentialMigrationCipher{cipher: cipher, lookup: h.Sum(nil)}, nil
}

// digest supports deterministic token lookup independently of random GCM nonces.
func (c *credentialMigrationCipher) digest(value string) string {
	h := hmac.New(sha256.New, c.lookup)
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// rewriteMigrationCredentials reads bounded batches on the same transaction.
// A later error rolls back all earlier rows, schema changes and index creation.
func rewriteMigrationCredentials(ctx context.Context, tx bun.Tx, keys *credentialMigrationCipher, source credentialMigrationSource, encrypt bool) error {
	version := 0
	if !encrypt {
		version = credentialMigrationVersion
	}
	var cursor int64
	for {
		var rows []credentialMigrationRow
		query := "SELECT " + source.columns + " FROM " + source.table +
			" WHERE id > ? AND credential_version = ? ORDER BY id LIMIT ? FOR UPDATE"
		if err := tx.NewRaw(query, cursor, version, credentialMigrationBatchSize).Scan(ctx, &rows); err != nil {
			return fmt.Errorf("read %s credentials: %w", source.table, err)
		}
		for _, row := range rows {
			if err := rewriteMigrationCredentialRow(ctx, tx, keys, source, row, encrypt); err != nil {
				return fmt.Errorf("rewrite %s credential id %d: %w", source.table, row.ID, err)
			}
			cursor = row.ID
		}
		if len(rows) < credentialMigrationBatchSize {
			return nil
		}
	}
}

// rewriteMigrationCredentialRow preserves NULLs and updates only credential
// columns. Driver parameters keep plaintext out of Bun SQL traces and audit hooks.
func rewriteMigrationCredentialRow(ctx context.Context, tx bun.Tx, keys *credentialMigrationCipher, source credentialMigrationSource, row credentialMigrationRow, encrypt bool) error {
	scope := []string{source.purpose, row.Owner}
	if source.table == "access_tokens" {
		scope[0] += row.Kind
		scope = append(scope, row.Namespace)
	}
	value := row.Value
	var err error
	if value.String != "" {
		if encrypt {
			value.String, err = keys.cipher.EncryptString(value.String, scope...)
		} else {
			value.String, err = keys.cipher.DecryptString(value.String, scope...)
		}
		if err != nil {
			return err
		}
	}
	version := 0
	if encrypt {
		version = credentialMigrationVersion
	}
	if source.table == "access_tokens" {
		var digest any
		kind := ""
		if encrypt {
			digest, kind = keys.digest(row.Value.String), row.Kind
		}
		_, err = tx.Tx.ExecContext(ctx, "UPDATE access_tokens SET token=$1,token_lookup_hash=$2,credential_kind=$3,credential_version=$4 WHERE id=$5", value, digest, kind, version, row.ID)
	} else {
		_, err = tx.Tx.ExecContext(ctx, "UPDATE "+source.table+" SET "+source.column+"=$1,credential_version=$2 WHERE id=$3", value, version, row.ID)
	}
	return err
}
