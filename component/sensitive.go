package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SensitiveChecker interface {
	CheckText(ctx context.Context, scenario, text string) (bool, error)
	CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error)
	CheckRequest(ctx context.Context, req types.SensitiveRequest) (bool, error)
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

func (cc *SensitiveComponent) CheckRequest(ctx context.Context, req types.SensitiveRequest) (bool, error) {
	if req.SensName() != "" {
		pass, err := cc.checker.PassTextCheck(ctx, sensitive.ScenarioNicknameDetection, req.SensName())
		if err != nil {
			slog.Error("fail to check name sensitivity", slog.String("name", req.SensName()), slog.Any("error", err))
			return false, fmt.Errorf("fail to check name sensitivity, error: %w", err)
		}
		if !pass {
			slog.Error("found sensitive words in name", slog.String("name", req.SensName()))
			return false, fmt.Errorf("found sensitive words in name")
		}
	}
	if req.SensNickName() != "" {
		pass, err := cc.checker.PassTextCheck(ctx, sensitive.ScenarioNicknameDetection, req.SensNickName())
		if err != nil {
			slog.Error("fail to check nick name sensitivity", slog.String("nick_name", req.SensNickName()), slog.Any("error", err))
			return false, fmt.Errorf("fail to check nick name sensitivity, error: %w", err)
		}
		if !pass {
			slog.Error("found sensitive words in nick name", slog.String("nick_name", req.SensNickName()))
			return false, fmt.Errorf("found sensitive words in nick name")
		}
	}
	if req.SensDescription() != "" {
		pass, err := cc.checker.PassTextCheck(ctx, sensitive.ScenarioCommentDetection, req.SensDescription())
		if err != nil {
			slog.Error("fail to check description sensitivity", slog.String("description", req.SensDescription()), slog.Any("error", err))
			return false, fmt.Errorf("fail to check description sensitivity, error: %w", err)
		}
		if !pass {
			slog.Error("found sensitive words in description", slog.String("description", req.SensDescription()))
			return false, errors.New("found sensitive words in description")
		}
	}
	if req.SensHomepage() != "" {
		pass, err := cc.checker.PassTextCheck(ctx, sensitive.ScenarioChatDetection, req.SensHomepage())
		if err != nil {
			slog.Error("fail to check homepage sensitivity", slog.String("homepage", req.SensHomepage()), slog.Any("error", err))
			return false, fmt.Errorf("fail to check homepage sensitivity, error: %w", err)
		}
		if !pass {
			slog.Error("found sensitive words in homepage", slog.String("homepage", req.SensHomepage()))
			return false, errors.New("found sensitive words in homepage")
		}
	}

	return true, nil
}

func (c NopSensitiveComponent) CheckText(ctx context.Context, scenario, text string) (bool, error) {
	return true, nil
}

func (c NopSensitiveComponent) CheckImage(ctx context.Context, scenario, ossBucketName, ossObjectName string) (bool, error) {
	return true, nil
}

func (c NopSensitiveComponent) CheckRequest(ctx context.Context, req types.SensitiveRequest) (bool, error) {
	return true, nil
}
