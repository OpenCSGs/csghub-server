package imagebuilder

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type Builder interface {
	Build(context.Context, *types.ImageBuilderRequest) error
	Stop(context.Context, types.ImageBuildStopReq) error
}
