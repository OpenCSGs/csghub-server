//go:build !ee && !saas

package plan

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// CE has no metrics middleware and no admission queue, so there is no queue
// wait to record.
func recordQueueWait(_ *gin.Context, _ *types.RequestMetadata) {}
