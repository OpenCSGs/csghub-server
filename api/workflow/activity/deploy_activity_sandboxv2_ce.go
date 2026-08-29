//go:build !ee && !saas

package activity

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
)

// deploySandboxV2 is unavailable in CE builds: SandboxV2 depends on the agent-sandbox SDK (ee/saas only).
func (a *DeployActivity) deploySandboxV2(ctx context.Context, task *database.DeployTask) error {
	return fmt.Errorf("sandbox v2 is not supported in CE build")
}
