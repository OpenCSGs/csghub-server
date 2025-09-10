//go:build !ee && !saas

package callback

import (
	"opencsg.com/csghub-server/common/types"
)

func (g *syncVersionGeneratorImpl) GenSyncVersion(req *types.GiteaCallbackPushReq) error {
	return nil
}
