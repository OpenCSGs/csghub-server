package database

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
)

type AgentInstanceSession struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	UUID       string `bun:",notnull" json:"uuid"`
	Name       string `bun:",nullzero" json:"name"`
	InstanceID int64  `bun:",notnull" json:"instance_id"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	Type       string `bun:",notnull" json:"type"` // Possible values: langflow, agno, code, etc.
	times
}

type AgentInstanceSessionHistory struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	SessionID int64  `bun:",notnull" json:"session_id"`
	Request   bool   `bun:",notnull" json:"request"` // true: request, false: response
	Content   string `bun:",type:text" json:"content"`
	times
}

type AgentInstanceSessionStore interface {
	Create(ctx context.Context, session *AgentInstanceSession) (*AgentInstanceSession, error)
	FindByID(ctx context.Context, id int64) (*AgentInstanceSession, error)
	FindByUUID(ctx context.Context, uuid string) (*AgentInstanceSession, error)
	ListByInstanceID(ctx context.Context, instanceID int64) ([]AgentInstanceSession, int, error)
	Update(ctx context.Context, session *AgentInstanceSession) error
	Delete(ctx context.Context, id int64) error
}

type AgentInstanceSessionHistoryStore interface {
	Create(ctx context.Context, history *AgentInstanceSessionHistory) error
	FindByID(ctx context.Context, id int64) (*AgentInstanceSessionHistory, error)
	ListBySessionID(ctx context.Context, sessionID int64) ([]AgentInstanceSessionHistory, error)
	Update(ctx context.Context, history *AgentInstanceSessionHistory) error
	Delete(ctx context.Context, id int64) error
}

// agentInstanceSessionStoreImpl is the implementation of AgentInstanceSessionStore
type agentInstanceSessionStoreImpl struct {
	db *DB
}

// agentInstanceSessionHistoryStoreImpl is the implementation of AgentInstanceSessionHistoryStore
type agentInstanceSessionHistoryStoreImpl struct {
	db *DB
}

// NewAgentInstanceSessionStore creates a new AgentInstanceSessionStore
func NewAgentInstanceSessionStore() AgentInstanceSessionStore {
	return &agentInstanceSessionStoreImpl{
		db: defaultDB,
	}
}

// NewAgentInstanceSessionStoreWithDB creates a new AgentInstanceSessionStore with a specific DB
func NewAgentInstanceSessionStoreWithDB(db *DB) AgentInstanceSessionStore {
	return &agentInstanceSessionStoreImpl{
		db: db,
	}
}

// NewAgentInstanceSessionHistoryStore creates a new AgentInstanceSessionHistoryStore
func NewAgentInstanceSessionHistoryStore() AgentInstanceSessionHistoryStore {
	return &agentInstanceSessionHistoryStoreImpl{
		db: defaultDB,
	}
}

// NewAgentInstanceSessionHistoryStoreWithDB creates a new AgentInstanceSessionHistoryStore with a specific DB
func NewAgentInstanceSessionHistoryStoreWithDB(db *DB) AgentInstanceSessionHistoryStore {
	return &agentInstanceSessionHistoryStoreImpl{
		db: db,
	}
}

// Create inserts a new AgentInstanceSession into the database
func (s *agentInstanceSessionStoreImpl) Create(ctx context.Context, session *AgentInstanceSession) (*AgentInstanceSession, error) {
	res, err := s.db.Core.NewInsert().Model(session).Exec(ctx, session)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"session_uuid": session.UUID,
			"instance_id":  session.InstanceID,
			"user_uuid":    session.UserUUID,
		})
	}
	return session, nil
}

// FindByID retrieves an AgentInstanceSession by its ID
func (s *agentInstanceSessionStoreImpl) FindByID(ctx context.Context, id int64) (*AgentInstanceSession, error) {
	session := &AgentInstanceSession{}
	err := s.db.Core.NewSelect().Model(session).Where("id = ?", id).Scan(ctx, session)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"session_id": id,
			"operation":  "find_by_id",
		})
	}
	return session, nil
}

// FindByUUID retrieves an AgentInstanceSession by its UUID
func (s *agentInstanceSessionStoreImpl) FindByUUID(ctx context.Context, uuid string) (*AgentInstanceSession, error) {
	session := &AgentInstanceSession{}
	err := s.db.Core.NewSelect().Model(session).Where("uuid = ?", uuid).Scan(ctx, session)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"session_uuid": uuid,
			"operation":    "find_by_uuid",
		})
	}
	return session, nil
}

// ListByInstanceID retrieves all AgentInstanceSessions for a specific instance
func (s *agentInstanceSessionStoreImpl) ListByInstanceID(ctx context.Context, instanceID int64) ([]AgentInstanceSession, int, error) {
	var sessions []AgentInstanceSession
	count, err := s.db.Core.NewSelect().Model(&sessions).Where("instance_id = ?", instanceID).Order("updated_at DESC").ScanAndCount(ctx, &sessions)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"instance_id": instanceID,
			"operation":   "list_by_instance_id",
		})
	}
	return sessions, count, nil
}

// Update updates an existing AgentInstanceSession
func (s *agentInstanceSessionStoreImpl) Update(ctx context.Context, session *AgentInstanceSession) error {
	res, err := s.db.Core.NewUpdate().Model(session).Where("id = ?", session.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"session_id": session.ID,
			"operation":  "update",
		})
	}
	return nil
}

// Delete removes an AgentInstanceSession and its associated history from the database
func (s *agentInstanceSessionStoreImpl) Delete(ctx context.Context, id int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// First delete all session history records
		_, err := tx.NewDelete().Model((*AgentInstanceSessionHistory)(nil)).Where("session_id = ?", id).Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"session_id": id,
				"operation":  "delete_history",
			})
		}

		// Then delete the session itself
		res, err := tx.NewDelete().Model((*AgentInstanceSession)(nil)).Where("id = ?", id).Exec(ctx)
		if err = assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"session_id": id,
				"operation":  "delete_session",
			})
		}

		return nil
	})
}

// Create inserts a new AgentInstanceSessionHistory into the database
func (s *agentInstanceSessionHistoryStoreImpl) Create(ctx context.Context, history *AgentInstanceSessionHistory) error {
	res, err := s.db.Core.NewInsert().Model(history).Exec(ctx, history)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"session_id": history.SessionID,
			"operation":  "create_history",
		})
	}
	return nil
}

// FindByID retrieves an AgentInstanceSessionHistory by its ID
func (s *agentInstanceSessionHistoryStoreImpl) FindByID(ctx context.Context, id int64) (*AgentInstanceSessionHistory, error) {
	history := &AgentInstanceSessionHistory{}
	err := s.db.Core.NewSelect().Model(history).Where("id = ?", id).Scan(ctx, history)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"history_id": id,
			"operation":  "find_by_id",
		})
	}
	return history, nil
}

// ListBySessionID retrieves all AgentInstanceSessionHistory for a specific session
func (s *agentInstanceSessionHistoryStoreImpl) ListBySessionID(ctx context.Context, sessionID int64) ([]AgentInstanceSessionHistory, error) {
	var histories []AgentInstanceSessionHistory
	err := s.db.Core.NewSelect().Model(&histories).Where("session_id = ?", sessionID).Order("created_at ASC").Scan(ctx, &histories)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"session_id": sessionID,
			"operation":  "list_by_session_id",
		})
	}
	return histories, nil
}

// Update updates an existing AgentInstanceSessionHistory
func (s *agentInstanceSessionHistoryStoreImpl) Update(ctx context.Context, history *AgentInstanceSessionHistory) error {
	res, err := s.db.Core.NewUpdate().Model(history).Where("id = ?", history.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"history_id": history.ID,
			"operation":  "update",
		})
	}
	return nil
}

// Delete removes an AgentInstanceSessionHistory from the database
func (s *agentInstanceSessionHistoryStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentInstanceSessionHistory)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"history_id": id,
			"operation":  "delete_history",
		})
	}
	return nil
}
