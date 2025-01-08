package activity

import (
	"context"
)

func (a *Activities) CalcRecomScore(ctx context.Context) error {
	a.recom.CalculateRecomScore(context.Background())
	return nil
}
