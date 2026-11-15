//go:build !ee && !saas

package lfssyncer

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (w *LfsSyncWorker) initLatestLFS() { w.recordLatestLFS = nil }
func (w *LfsSyncWorker) markLatestLFS(context.Context, *database.MirrorTask, []database.LfsMetaObject) error {
	return nil
}
func (w *LfsSyncWorker) persistLatestLFS(context.Context, *database.MirrorTask, []database.LfsMetaObject) error {
	return nil
}
func prioritizeLatestLFS([]database.LfsMetaObject, string) {}

func preservePointerOrder(_, resolved []*types.Pointer) []*types.Pointer { return resolved }
