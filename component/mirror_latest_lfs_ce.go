//go:build !ee && !saas

package component

import (
	"context"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (m *mirrorComponentImpl) initLatestLFSReady() {}
func (m *mirrorComponentImpl) populateLatestLFSReady(context.Context, int64, *database.Mirror, *types.Mirror) error {
	return nil
}
