package database

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/cryptox"
)

type times struct {
	CreatedAt time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

func (t *times) BeforeAppendModel(ctx context.Context, query schema.Query) error {
	switch query.(type) {
	case *bun.UpdateQuery:
		q := query.(*bun.UpdateQuery)
		m := q.GetModel().Value()
		//skip update for Repository, and related assets
		if _, ok := m.(*Repository); ok {
			return nil
		}
		if _, ok := m.(*Model); ok {
			return nil
		}
		if _, ok := m.(*Dataset); ok {
			return nil
		}
		if _, ok := m.(*Code); ok {
			return nil
		}
		if _, ok := m.(*Space); ok {
			return nil
		}
		t.UpdatedAt = time.Now()
	}

	return nil
}

func assertAffectedOneRow(result sql.Result, err error) error {
	return assertAffectedXRows(1, result, err)
}

func assertAffectedXRows(X int64, result sql.Result, err error) error {
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("retrieving affected row count: %w", err)
		return err
	}
	if affected != X {
		err = fmt.Errorf("affected %d row(s), want %d", affected, X)
		return err
	}

	return nil
}

// credentialEncryptionVersion marks rows encrypted by the shared cryptox envelope.
const credentialEncryptionVersion = 1

// credentialKeySet holds the domain-separated cipher and credential lookup key.
type credentialKeySet struct {
	cipher *cryptox.Cipher
	lookup []byte
}

// storageKeys initializes shared keys once per process, separately from FedAP scopes.
var storageKeys = sync.OnceValues(func() (*credentialKeySet, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	master, err := base64.StdEncoding.DecodeString(cfg.Credential.MasterKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode credential master key: %w", err)
	}
	c, err := cryptox.NewCipher(cryptox.Config{MasterKey: master, Salt: []byte("csghub/credentials/storage/v1/salt"), InfoPrefix: "csghub/credentials/storage/"})
	if err != nil {
		return nil, err
	}
	// A separate HMAC purpose prevents reuse of an AES key as a lookup key.
	h := hmac.New(sha256.New, master)
	h.Write([]byte("csghub/credentials/lookup/v1"))
	return &credentialKeySet{cipher: c, lookup: h.Sum(nil)}, nil
})

// storageCipher exposes the immutable initialized cipher and lookup key.
func storageCipher() (*cryptox.Cipher, []byte, error) {
	keys, err := storageKeys()
	if err != nil {
		return nil, nil, err
	}
	return keys.cipher, keys.lookup, nil
}

// CredentialLookupHash indexes an unchanged bearer credential without storing its plaintext.
// The index is independent of its owner and app because authentication starts with only a token.
func CredentialLookupHash(value string) (string, error) {
	_, key, err := storageCipher()
	if err != nil {
		return "", err
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// encryptStoredCredential encrypts a nonempty value under its purpose and stable owner scope.
func encryptStoredCredential(value, purpose string, scope ...string) (string, error) {
	if value == "" {
		return "", nil
	}
	c, _, err := storageCipher()
	if err != nil {
		return "", err
	}
	return c.EncryptString(value, append([]string{purpose}, scope...)...)
}

// decryptStoredCredential accepts legacy plaintext only on explicitly unversioned rows.
// Versioned rows fail closed on an unknown format, a wrong key, or authentication failure.
func decryptStoredCredential(value string, version int, purpose string, scope ...string) (string, error) {
	if version == 0 {
		return value, nil
	}
	if version != credentialEncryptionVersion {
		return "", fmt.Errorf("unsupported credential storage version %d", version)
	}
	if value == "" {
		return "", nil
	}
	c, _, err := storageCipher()
	if err != nil {
		return "", err
	}
	return c.DecryptString(value, append([]string{purpose}, scope...)...)
}

// credentialOwner formats a stable numeric identity without relying on editable display names.
func credentialOwner(id int64) string { return strconv.FormatInt(id, 10) }
