package database

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
)

// RiskRuleStore reads enabled contact risk rules from the database.
type RiskRuleStore struct{ db *DB }

// NewRiskRuleStore uses the default database.
func NewRiskRuleStore() *RiskRuleStore {
	return NewRiskRuleStoreWithDB(GetDB())
}

// NewRiskRuleStoreWithDB uses an explicit database.
func NewRiskRuleStoreWithDB(db *DB) *RiskRuleStore {
	return &RiskRuleStore{db: db}
}

// ListEmailDomains returns enabled email domain rules.
func (s *RiskRuleStore) ListEmailDomains(ctx context.Context) ([]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("risk rule database is not initialized")
	}
	var domains []string
	err := s.db.Core.NewSelect().TableExpr("risk_email_domains").Column("domain").Where("enabled = TRUE").Scan(ctx, &domains)
	if err != nil {
		return nil, fmt.Errorf("list email risk rules: %w", err)
	}
	return domains, nil
}

// ListPhonePrefixes returns enabled calling-code and prefix rules.
func (s *RiskRuleStore) ListPhonePrefixes(ctx context.Context) ([]types.RiskPhonePrefix, error) {
	if s.db == nil {
		return nil, fmt.Errorf("risk rule database is not initialized")
	}
	var prefixes []types.RiskPhonePrefix
	err := s.db.Core.NewSelect().TableExpr("risk_phone_prefixes").Column("phone_area", "prefix").Where("enabled = TRUE").Scan(ctx, &prefixes)
	if err != nil {
		return nil, fmt.Errorf("list phone risk rules: %w", err)
	}
	return prefixes, nil
}
