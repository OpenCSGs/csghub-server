package database

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// AgentUIOptionStore lists Spaces eligible to serve as an agent's UI.
type AgentUIOptionStore struct{ db *DB }

func NewAgentUIOptionStore() *AgentUIOptionStore { return &AgentUIOptionStore{db: defaultDB} }

func NewAgentUIOptionStoreWithDB(db *DB) *AgentUIOptionStore { return &AgentUIOptionStore{db: db} }

func (s *AgentUIOptionStore) HasAgentUITag(ctx context.Context, spaceID int64) (bool, error) {
	return s.db.Core.NewSelect().TableExpr("repository_tags AS rt").
		Join("JOIN tags AS t ON t.id = rt.tag_id").
		Join("JOIN spaces AS s ON s.repository_id = rt.repository_id").
		Where("s.id = ?", spaceID).
		Where("rt.count > 0 AND t.scope = ? AND t.category = ? AND t.name = ?", types.SpaceTagScope, "task", "agent-ui").
		Exists(ctx)
}

// List returns agent UI spaces visible under scope via the shared
// public-or-authorized contract (applyRepositoryAccessFilter): public spaces
// plus explicitly granted private ones. Binding a private space is allowed
// (its deployed page becomes reachable only through the agent's custom
// domain), so the response includes the private flag for the portal.
func (s *AgentUIOptionStore) List(ctx context.Context, scope RepositoryAccessScope, namespace string, per, page int) ([]types.AgentUIOption, int, error) {
	query := func() *bun.SelectQuery {
		q := s.db.Core.NewSelect().TableExpr("spaces AS s").
			Join("JOIN repositories AS r ON r.id = s.repository_id").
			Join("LEFT JOIN LATERAL (SELECT status, svc_name FROM deploys WHERE space_id = s.id ORDER BY created_at DESC LIMIT 1) AS d ON TRUE").
			Where("r.repository_type = ? AND r.deleted_at IS NULL", types.SpaceRepo).
			Where("EXISTS (SELECT 1 FROM repository_tags AS rt JOIN tags AS t ON t.id = rt.tag_id WHERE rt.repository_id = r.id AND rt.count > 0 AND t.scope = ? AND t.category = ? AND t.name = ?)", types.SpaceTagScope, "task", "agent-ui")
		if namespace != "" {
			q.Where("r.path LIKE ? ESCAPE '\\'", escapeLikePattern(namespace)+"/%")
		}
		applyRepositoryAccessFilter(q, "r", scope)
		return q
	}
	total, err := query().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	options := make([]types.AgentUIOption, 0)
	err = query().ColumnExpr("s.id AS space_id, r.path, r.name, r.private, s.sdk, d.svc_name, d.status AS deploy_status").
		OrderExpr("s.id DESC").Limit(per).Offset((page-1)*per).Scan(ctx, &options)
	if err != nil {
		return nil, 0, err
	}
	return options, total, nil
}
