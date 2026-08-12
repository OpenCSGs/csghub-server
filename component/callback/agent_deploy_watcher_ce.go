//go:build !(ee || saas)

package callback

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

// WatchAgentChange is a no-op in CE: agent sandbox deploy requires EE/saas components.
func (c *gitCallbackComponentImpl) WatchAgentChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	return nil
}
