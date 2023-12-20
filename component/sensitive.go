package component

import (
	"context"
	"fmt"

	"opencsg.com/starhub-server/builder/sensitive"
	"opencsg.com/starhub-server/common/config"
)

type SensitiveComponent struct {
	checker sensitive.SensitiveChecker
}

func NewSensitiveComponent(cfg *config.Config) *SensitiveComponent {
	return &SensitiveComponent{
		checker: sensitive.NewAliyunGreenChecker(cfg),
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
