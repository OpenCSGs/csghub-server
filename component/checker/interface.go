package checker

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type GitCallbackChecker interface {
	// Check checks the given value and returns an error if the value is invalid.
	Check(ctx context.Context, req types.GitalyAllowedReq) (bool, error)
}
