//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type RuleComponent interface {
	MultiSync(ctx context.Context) (types.Rule, error)
	Update(ctx context.Context, req types.UpdateRuleReq) (types.Rule, error)
}

type ruleComponentImpl struct {
	ruleStore database.RuleStore
}

func NewRuleComponent(config *config.Config) RuleComponent {
	return &ruleComponentImpl{
		ruleStore: database.NewRuleStore(),
	}
}

func (r *ruleComponentImpl) MultiSync(ctx context.Context) (types.Rule, error) {
	return types.Rule{}, nil
}

func (r *ruleComponentImpl) Update(ctx context.Context, req types.UpdateRuleReq) (types.Rule, error) {
	return types.Rule{}, nil
}
