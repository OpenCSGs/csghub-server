package migrations

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/ssh"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return calculateSSHKeyFingerprint(ctx, db)
	}, func(ctx context.Context, db *bun.DB) error {
		return nil
	})
}

func calculateSSHKeyFingerprint(ctx context.Context, db *bun.DB) error {
	var sshKeys []database.SSHKey
	err := db.NewSelect().
		Model(&database.SSHKey{}).
		Where("fingerprint_sha256 is NULL").
		Scan(ctx, &sshKeys)
	if err != nil {
		return err
	}

	for _, sshKey := range sshKeys {
		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshKey.Content))
		if err != nil {
			return err
		}
		fingerPrint := ssh.FingerprintSHA256(parsedKey)
		fingerPrint = strings.Split(fingerPrint, ":")[1]
		sshKey.FingerprintSHA256 = fingerPrint
		_, err = db.NewUpdate().
			Model(&sshKey).
			WherePK().
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	return err
}

