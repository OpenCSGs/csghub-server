package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

// AgentInstanceSessionShare stores a snapshot pointer for sharing a session read-only.
type AgentInstanceSessionShare struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	ShareUUID   string `bun:",notnull,unique" json:"share_uuid"`
	UserUUID    string `bun:",notnull" json:"user_uuid"`
	InstanceID  int64  `bun:",notnull" json:"instance_id"`
	SessionUUID string `bun:",notnull" json:"session_uuid"`
	MaxTurn     int64  `bun:",notnull" json:"max_turn"`
	ExpiresAt   int64  `bun:",notnull" json:"expires_at"`
	times
}

type AgentInstanceSessionShareStore interface {
	Create(ctx context.Context, share *AgentInstanceSessionShare) (*AgentInstanceSessionShare, error)
	FindByShareUUID(ctx context.Context, shareUUID string) (*AgentInstanceSessionShare, error)
}

type agentInstanceSessionShareStoreImpl struct {
	db *DB
}

func NewAgentInstanceSessionShareStore() AgentInstanceSessionShareStore {
	return &agentInstanceSessionShareStoreImpl{db: defaultDB}
}

func NewAgentInstanceSessionShareStoreWithDB(db *DB) AgentInstanceSessionShareStore {
	return &agentInstanceSessionShareStoreImpl{db: db}
}

func (s *agentInstanceSessionShareStoreImpl) Create(ctx context.Context, share *AgentInstanceSessionShare) (*AgentInstanceSessionShare, error) {
	res, err := s.db.Core.NewInsert().Model(share).Exec(ctx, share)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"share_uuid":   share.ShareUUID,
			"instance_id":  share.InstanceID,
			"session_uuid": share.SessionUUID,
			"user_uuid":    share.UserUUID,
			"operation":    "create",
		})
	}
	return share, nil
}

func (s *agentInstanceSessionShareStoreImpl) FindByShareUUID(ctx context.Context, shareUUID string) (*AgentInstanceSessionShare, error) {
	share := &AgentInstanceSessionShare{}
	err := s.db.Core.NewSelect().Model(share).Where("share_uuid = ?", shareUUID).Scan(ctx, share)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"share_uuid": shareUUID,
			"operation":  "find_by_share_uuid",
		})
	}
	return share, nil
}
