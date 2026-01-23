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

type sensitiveComponentImpl struct {
	checker rpc.ModerationSvcClient
}

type SensitiveComponent interface {
	CheckText(ctx context.Context, scenario types.SensitiveScenario, text string) (bool, error)
	CheckImage(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (bool, error)
	CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error)
}

func NewSensitiveComponent(cfg *config.Config) (SensitiveComponent, error) {
	if !cfg.SensitiveCheck.Enable {
		return &sensitiveComponentNoOpImpl{}, nil
	}

	c := &sensitiveComponentImpl{}
	c.checker = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", cfg.Moderation.Host, cfg.Moderation.Port))
	return c, nil
}

func (c sensitiveComponentImpl) CheckText(ctx context.Context, scenario types.SensitiveScenario, text string) (bool, error) {
	result, err := c.checker.PassTextCheck(ctx, scenario, text)
	if err != nil {
		return false, err
	}

	return !result.IsSensitive, nil
}

func (c sensitiveComponentImpl) CheckImage(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (bool, error) {
	result, err := c.checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		return false, err
	}
	return !result.IsSensitive, nil
}

func (c sensitiveComponentImpl) CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error) {
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

// sensitiveComponentNoOpImpl this implementation provides a "no-op" (no operation) version of the SensitiveComponent interface,
// where all methods simply return a "not sensitive" result without performing any actual checks.
type sensitiveComponentNoOpImpl struct {
}

func (c *sensitiveComponentNoOpImpl) CheckText(ctx context.Context, scenario types.SensitiveScenario, text string) (bool, error) {
	return true, nil
}

func (c *sensitiveComponentNoOpImpl) CheckImage(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (bool, error) {
	return true, nil
}

// implements SensitiveComponent
func (c *sensitiveComponentNoOpImpl) CheckRequestV2(ctx context.Context, req types.SensitiveRequestV2) (bool, error) {
	return true, nil
}
