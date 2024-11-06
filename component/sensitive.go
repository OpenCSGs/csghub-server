package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SensitiveComponent struct {
	checker rpc.ModerationSvcClient
	enable  bool
}

func NewSensitiveComponent(cfg *config.Config) (*SensitiveComponent, error) {
	c := &SensitiveComponent{}
	c.enable = cfg.SensitiveCheck.Enable

	if c.enable {
		c.checker = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", cfg.Moderation.Host, cfg.Moderation.Port))
	}
	return c, nil
}

func (c SensitiveComponent) CheckText(ctx context.Context, scenario, text string) (bool, error) {
	if !c.enable {
		return true, nil
	}

	result, err := c.checker.PassTextCheck(ctx, scenario, text)
	if err != nil {
		return false, err
	}

	return !result.IsSensitive, nil
}

func (c SensitiveComponent) CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error) {
	if !c.enable {
		return true, nil
	}

	result, err := c.checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		return false, err
	}
	return !result.IsSensitive, nil
}

func (c SensitiveComponent) CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error) {
	if !c.enable {
		return true, nil
	}

	fields := req.GetSensitiveFields()
	for _, field := range fields {
		if len(field.Value()) == 0 {
			continue
		}
		result, err := c.checker.PassTextCheck(ctx, field.Scenario, field.Value())
		if err != nil {
			slog.Error("fail to check request sensitivity", slog.String("field", field.Name), slog.Any("error", err))
			return false, fmt.Errorf("fail to check '%s' sensitivity, error: %w", field.Name, err)
		}
		if result.IsSensitive {
			slog.Error("found sensitive words in request", slog.String("field", field.Name))
			return false, errors.New("found sensitive words in field: " + field.Name)
		}
	}
	return true, nil
}
