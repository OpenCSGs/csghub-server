//go:build !ee && !saas

package database

import (
	"context"
	"github.com/uptrace/bun"
)

func invalidateLatestLFSSnapshotTx(context.Context, bun.Tx, int64) error { return nil }
