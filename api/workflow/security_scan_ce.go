//go:build !ee && !saas

package workflow

import (
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/types"
)

func submitSecurityScanFromPush(_ workflow.Context, _ *types.GiteaCallbackPushReq) error { return nil }
