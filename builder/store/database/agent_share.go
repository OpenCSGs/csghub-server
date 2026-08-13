package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type AgentShare struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	ShareUUID  string `bun:",notnull,unique" json:"share_uuid"`
	ShareName  string `bun:",notnull,unique" json:"share_name"`
	Type       string `bun:",notnull" json:"type"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	InstanceID int64  `bun:",notnull" json:"instance_id"`
	times
}

type AgentShareStore interface {
	Create(ctx context.Context, share *AgentShare) (*AgentShare, error)
	FindByShareUUID(ctx context.Context, shareUUID string) (*AgentShare, error)
	FindByShareName(ctx context.Context, shareName string) (*AgentShare, error)
}

type agentShareStoreImpl struct {
	db *DB
}

func NewAgentShareStore() AgentShareStore {
	return &agentShareStoreImpl{db: defaultDB}
}

func NewAgentShareStoreWithDB(db *DB) AgentShareStore {
	return &agentShareStoreImpl{db: db}
}

func (s *agentShareStoreImpl) Create(ctx context.Context, share *AgentShare) (*AgentShare, error) {
	res, err := s.db.Core.NewInsert().Model(share).Exec(ctx, share)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"share_uuid":  share.ShareUUID,
			"share_name":  share.ShareName,
			"instance_id": share.InstanceID,
			"user_uuid":   share.UserUUID,
			"operation":   "create",
		})
	}
	return share, nil
}

func (s *agentShareStoreImpl) FindByShareUUID(ctx context.Context, shareUUID string) (*AgentShare, error) {
	return s.find(ctx, "share_uuid", shareUUID)
}

func (s *agentShareStoreImpl) FindByShareName(ctx context.Context, shareName string) (*AgentShare, error) {
	return s.find(ctx, "share_name", shareName)
}

func (s *agentShareStoreImpl) find(ctx context.Context, column string, value string) (*AgentShare, error) {
	share := &AgentShare{}
	err := s.db.Core.NewSelect().Model(share).Where(column+" = ?", value).Scan(ctx, share)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"operation": "find_by_" + column,
		})
	}
	return share, nil
}
