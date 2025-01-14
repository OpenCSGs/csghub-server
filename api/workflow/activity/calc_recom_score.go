package activity

import (
	"context"
)

func (a *Activities) CalcRecomScore(ctx context.Context) error {
	return a.recom.CalculateRecomScore(ctx, 0)
}
