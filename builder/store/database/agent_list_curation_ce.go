//go:build !ee && !saas

package database

import (
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

func applyAgentSkillCuration(q *bun.SelectQuery, _ string) *bun.SelectQuery { return q }
func orderAgentSkillCuration(q *bun.SelectQuery) *bun.SelectQuery {
	return q.OrderExpr("pin_pref.created_at DESC NULLS LAST, s.updated_at DESC")
}
func applyAgentPromptCuration(q *bun.SelectQuery) *bun.SelectQuery { return q }
func orderAgentPromptCuration(q *bun.SelectQuery) *bun.SelectQuery {
	return q.OrderExpr("pin_pref.created_at DESC NULLS LAST, p.updated_at DESC")
}
func applyAgentMCPCuration(q *bun.SelectQuery) *bun.SelectQuery { return q }
func orderAgentMCPCuration(q *bun.SelectQuery) *bun.SelectQuery {
	return q.OrderExpr("pin_pref.created_at DESC NULLS LAST, amsv.updated_at DESC")
}
func applyAgentKnowledgeCuration(q *bun.SelectQuery, _ string, _ []int64) *bun.SelectQuery { return q }
func orderAgentKnowledgeCuration(q *bun.SelectQuery) *bun.SelectQuery {
	return q.OrderExpr("(pin_pref.id IS NOT NULL) DESC, CASE akb.type WHEN ? THEN 0 WHEN ? THEN 1 ELSE 2 END, akb.updated_at DESC, akb.id DESC", types.AgentKnowledgeBaseTypeLLMWiki, types.AgentKnowledgeBaseTypeLangflow)
}
