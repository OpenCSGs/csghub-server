package types

import (
	"time"

	"opencsg.com/csghub-server/common/types/enum"
)

type Rule struct {
	ID       int64         `json:"id"`
	Content  string        `json:"content"`
	RuleType enum.RuleType `json:"rule_type"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateRuleReq struct {
	ID      int64  `json:"id" validate:"required"`
	Content string `json:"content" validate:"required"`
}
