package component

import (
	"context"
	"fmt"

	"opencsg.com/starhub-server/builder/sensitive"
	"opencsg.com/starhub-server/common/config"
)

type SensitiveChecker interface {
	CheckText(ctx context.Context, scenario, text string) (bool, error)
	CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error)
}

type SensitiveComponent struct {
	checker sensitive.SensitiveChecker
}

type NopSensitiveComponent struct{}

func NewSensitiveComponent(cfg *config.Config) SensitiveChecker {
	if cfg.SensitiveCheck.Enable {
		return &SensitiveComponent{
			checker: sensitive.NewAliyunGreenChecker(cfg),
		}
	} else {
		return &NopSensitiveComponent{}
	}
}

func (c SensitiveComponent) CheckText(ctx context.Context, scenario, text string) (bool, error) {
	var (
		s  sensitive.Scenario
		ok bool
	)
	if s, ok = s.FromString(scenario); !ok {
		return false, fmt.Errorf("invalid scenario: %s", scenario)
	}
	return c.checker.PassTextCheck(ctx, s, text)
}

func (c SensitiveComponent) CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error) {
	var (
		s  sensitive.Scenario
		ok bool
	)
	if s, ok = s.FromString(scenario); !ok {
		return false, fmt.Errorf("invalid scenario: %s", scenario)
	}
	return c.checker.PassImageCheck(ctx, s, ossBucketName, ossObjectName)
}

func (c NopSensitiveComponent) CheckText(ctx context.Context, scenario, text string) (bool, error) {
	return true, nil
}

func (c NopSensitiveComponent) CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error) {
	return true, nil
}
