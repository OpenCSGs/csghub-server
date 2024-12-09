package activity

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

func CalcRecomScore(ctx context.Context, config *config.Config) error {
	c, err := component.NewRecomComponent(config)
	if err != nil {
		slog.Error("failed to create recom component", "err", err)
		return err
	}
	c.CalculateRecomScore(context.Background())
	return nil
}
