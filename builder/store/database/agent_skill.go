package database

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentSkillFilter for list filtering
type AgentSkillFilter struct {
	Search  string
	BuiltIn *bool // nil = all, true = built-in only, false = user-created only
}

type AgentSkillStore interface {
	// ListForAgent returns skills for the agent UI: user-created + platform (agentichub-skills), with pin ordering
	ListForAgent(ctx context.Context, userUUID string, username string, filter AgentSkillFilter, per, page int) ([]types.AgentSkillListItem, int, error)
}

type agentSkillStoreImpl struct {
	db *DB
}

func NewAgentSkillStore() AgentSkillStore {
	return &agentSkillStoreImpl{db: defaultDB}
}

func NewAgentSkillStoreWithDB(db *DB) AgentSkillStore {
	return &agentSkillStoreImpl{db: db}
}

// applyAgentSkillFilter applies filter conditions to the query
func applyAgentSkillFilter(query *bun.SelectQuery, username string, filter AgentSkillFilter) *bun.SelectQuery {
	// Add search filter if provided
	search := strings.TrimSpace(filter.Search)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("(LOWER(r.name) LIKE LOWER(?) OR LOWER(COALESCE(r.description, '')) LIKE LOWER(?))", searchPattern, searchPattern)
	}

	// Apply built_in filter
	agentichubExists := `EXISTS(
		SELECT 1 FROM repository_tags rt
		JOIN tags t ON rt.tag_id = t.id AND t.name = ? AND t.scope = ?
		WHERE rt.repository_id = s.repository_id AND rt.count > 0
	)`
	if filter.BuiltIn != nil {
		if *filter.BuiltIn {
			// Only built-in skills (platform skills: agentichub-skills tag, public, and NOT user's own)
			query = query.Where(agentichubExists+" AND r.private = false AND r.path NOT LIKE ?", types.AgentichubSkillsTagName, types.SkillTagScope, username+"/%")
		} else {
			// Only user-created skills (user's own skills, no tag required)
			query = query.Where("r.path LIKE ?", username+"/%")
		}
	} else {
		// All: user-created skills (path) OR platform skills (agentichub-skills tag + public)
		query = query.Where("("+agentichubExists+" AND r.private = false) OR r.path LIKE ?", types.AgentichubSkillsTagName, types.SkillTagScope, username+"/%")
	}

	return query
}

// ListForAgent returns user-created skills (with or without agentichub-skills tag) and platform skills (with agentichub-skills tag) for the agent UI
func (s *agentSkillStoreImpl) ListForAgent(ctx context.Context, userUUID string, username string, filter AgentSkillFilter, per, page int) ([]types.AgentSkillListItem, int, error) {
	var skills []types.AgentSkillListItem

	// Build the base query with pin join (select from existing "skills" table)
	query := s.db.Operator.Core.NewSelect().
		TableExpr("skills AS s").
		ColumnExpr("s.id AS id, s.created_at AS created_at, s.updated_at AS updated_at").
		ColumnExpr("(pin_pref.id IS NOT NULL) AS is_pinned").
		ColumnExpr("pin_pref.created_at AS pinned_at").
		ColumnExpr(`(EXISTS(
			SELECT 1 FROM repository_tags rt
			JOIN tags t ON rt.tag_id = t.id AND t.name = ? AND t.scope = ?
			WHERE rt.repository_id = s.repository_id AND rt.count > 0
		) AND r.path NOT LIKE ?) AS built_in`, types.AgentichubSkillsTagName, types.SkillTagScope, username+"/%").
		ColumnExpr("r.path AS path").
		ColumnExpr("r.name AS name").
		ColumnExpr("r.description AS description").
		ColumnExpr("r.private AS private").
		ColumnExpr("(NOT r.private) AS public").
		ColumnExpr("u.username AS owner").
		ColumnExpr("COALESCE(u.avatar, '') AS owner_avatar").
		Join("LEFT JOIN repositories AS r ON s.repository_id = r.id").
		Join("LEFT JOIN users AS u ON r.user_id = u.id").
		Join(`
			LEFT JOIN agent_user_preferences pin_pref
			ON pin_pref.user_uuid = ?
			AND pin_pref.action = ?
			AND pin_pref.entity_type = ?
			AND pin_pref.entity_id = CAST(s.id AS TEXT)
		`, userUUID, types.AgentUserPreferenceActionPin, types.AgentUserPreferenceEntityTypeAgentSkill).
		Where("r.id IS NOT NULL").
		Where("r.repository_type = ?", types.SkillRepo)

	// Apply filters
	query = applyAgentSkillFilter(query, username, filter)

	// Order: pinned first (by pin time desc), then by updated_at desc
	query = query.OrderExpr("pin_pref.created_at DESC NULLS LAST, s.updated_at DESC")

	// Get total count
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err,
			errorx.Ctx().
				Set("user_uuid", userUUID).
				Set("username", username),
		)
	}

	// Apply pagination
	query = query.Limit(per).Offset((page - 1) * per)

	// Execute query
	err = query.Scan(ctx, &skills)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err,
			errorx.Ctx().
				Set("user_uuid", userUUID).
				Set("username", username),
		)
	}

	return skills, total, nil
}
