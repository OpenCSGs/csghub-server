package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AgentInstanceSession struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	UUID       string `bun:",notnull,unique" json:"uuid"`
	Name       string `bun:",nullzero" json:"name"`
	InstanceID int64  `bun:",notnull" json:"instance_id"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	Type       string `bun:",notnull" json:"type"`      // Possible values: langflow, agno, code, etc.
	LastTurn   int64  `bun:",notnull" json:"last_turn"` // the last turn number of the session, used to prevent race conditions
	times
}

type AgentInstanceSessionHistory struct {
	ID          int64                             `bun:",pk,autoincrement" json:"id"`
	UUID        string                            `bun:",notnull,unique" json:"uuid"` // used to identify the history message in the frontend, used to update the history
	SessionID   int64                             `bun:",notnull" json:"session_id"`
	Request     bool                              `bun:",notnull" json:"request"` // true=request(from user), false=response(from assistant)
	Turn        int64                             `bun:",notnull" json:"turn"`    // use incremental turn number to indicate the response to its request, not just by time
	Content     string                            `bun:",type:text" json:"content"`
	Feedback    types.AgentSessionHistoryFeedback `bun:",notnull,default:'none'" json:"feedback"` // feedback options: none, like, dislike
	IsRewritten bool                              `bun:",notnull,default:false" json:"is_rewritten"`
	times
}

type AgentInstanceSessionStore interface {
	Create(ctx context.Context, session *AgentInstanceSession) (*AgentInstanceSession, error)
	FindByID(ctx context.Context, id int64) (*AgentInstanceSession, error)
	FindByUUID(ctx context.Context, uuid string) (*AgentInstanceSession, error)
	ListByInstanceID(ctx context.Context, instanceID int64) ([]AgentInstanceSession, int, error)
	List(ctx context.Context, userUUID string, filter types.AgentInstanceSessionFilter, per int, page int) ([]AgentInstanceSession, int, error)
	Update(ctx context.Context, session *AgentInstanceSession) error
	Delete(ctx context.Context, id int64) error
}

type AgentSessionHistoryFeedback struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	HistoryID int64  `bun:",notnull" json:"history_id"`
	UserUUID  string `bun:",notnull" json:"user_uuid"`
	Liked     bool   `bun:",notnull,default:false" json:"liked"`
	Disliked  bool   `bun:",notnull,default:false" json:"disliked"`
	Reason    string `bun:",nullzero" json:"reason"`
	times
}

type AgentInstanceSessionHistoryStore interface {
	Create(ctx context.Context, history *AgentInstanceSessionHistory) error
	FindByID(ctx context.Context, id int64) (*AgentInstanceSessionHistory, error)
	FindByUUID(ctx context.Context, uuid string) (*AgentInstanceSessionHistory, error)
	ListBySessionID(ctx context.Context, sessionID int64) ([]AgentInstanceSessionHistory, error)
	Update(ctx context.Context, history *AgentInstanceSessionHistory) error
	Delete(ctx context.Context, id int64) error
	Rewrite(ctx context.Context, originalMsgUUID string, history *AgentInstanceSessionHistory) error
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

// List retrieves all AgentInstanceSessions with pagination
func (s *agentInstanceSessionStoreImpl) List(ctx context.Context, userUUID string, filter types.AgentInstanceSessionFilter, per int, page int) ([]AgentInstanceSession, int, error) {
	var sessions []AgentInstanceSession
	query := s.db.Core.NewSelect().Model(&sessions)
	if filter.InstanceID != nil {
		query = query.Where("instance_id = ?", *filter.InstanceID)
	}

	if userUUID != "" {
		query = query.Where("user_uuid = ?", userUUID)
	}

	// Apply search filter
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?)", searchPattern)
	}

	// Count total before pagination
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"instance_id": filter.InstanceID,
			"operation":   "list",
		})
	}

	// Apply pagination and scan
	err = query.Order("updated_at DESC").Limit(per).Offset((page-1)*per).Scan(ctx, &sessions)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"instance_id": filter.InstanceID,
			"user_uuid":   userUUID,
			"operation":   "list",
		})
	}
	return sessions, total, nil
}

// Create inserts a new AgentInstanceSessionHistory into the database
func (s *agentInstanceSessionHistoryStoreImpl) Create(ctx context.Context, history *AgentInstanceSessionHistory) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Lock the session row to prevent concurrent updates
		var lastTurn int64
		err := tx.NewSelect().
			Model((*AgentInstanceSession)(nil)).
			ColumnExpr("last_turn").
			Where("id = ?", history.SessionID).
			For("UPDATE").
			Scan(ctx, &lastTurn)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"session_id": history.SessionID,
				"operation":  "select_last_turn",
			})
		}

		if history.Request {
			history.Turn = lastTurn + 1
		} else {
			// For responses, ensure the corresponding request has been processed first.
			// With multiple consumers on different hosts, messages can arrive out of order.
			// First check if there's already a response for the current last_turn.
			responseCount, err := tx.NewSelect().
				Model((*AgentInstanceSessionHistory)(nil)).
				Where("session_id = ?", history.SessionID).
				Where("request = ?", false).
				Where("turn = ?", lastTurn).
				Count(ctx)
			if err != nil {
				return errorx.HandleDBError(err, map[string]any{
					"session_id": history.SessionID,
					"operation":  "check_response_for_turn",
				})
			}

			if responseCount > 0 {
				return fmt.Errorf("response for turn %d already exists, waiting for next request, session_id: %d", lastTurn, history.SessionID)
			}

			// Then check if there's a request for the current last_turn.
			// This ensures the response corresponds to a request that has been processed.
			requestCount, err := tx.NewSelect().
				Model((*AgentInstanceSessionHistory)(nil)).
				Where("session_id = ?", history.SessionID).
				Where("request = ?", true).
				Where("turn = ?", lastTurn).
				Count(ctx)
			if err != nil {
				return errorx.HandleDBError(err, map[string]any{
					"session_id": history.SessionID,
					"operation":  "check_request_for_turn",
				})
			}

			if requestCount == 0 {
				return fmt.Errorf("response arrived before corresponding request for turn %d, session_id: %d", lastTurn, history.SessionID)
			}

			history.Turn = lastTurn
		}

		res, err := tx.NewInsert().Model(history).Exec(ctx)
		if err = assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"session_id": history.SessionID,
				"operation":  "insert_history",
			})
		}

		if history.Request {
			res, err = tx.NewUpdate().
				Model((*AgentInstanceSession)(nil)).
				Set("last_turn = ?", history.Turn).
				Where("id = ?", history.SessionID).
				Exec(ctx)
			if err = assertAffectedOneRow(res, err); err != nil {
				return errorx.HandleDBError(err, map[string]any{
					"session_id": history.SessionID,
					"operation":  "update_last_turn",
				})
			}
		} else {
			// Update session's updated_at to maintain row lock and avoid concurrency issues
			res, err = tx.NewUpdate().
				Model((*AgentInstanceSession)(nil)).
				Set("updated_at = now()").
				Where("id = ?", history.SessionID).
				Exec(ctx)
			if err = assertAffectedOneRow(res, err); err != nil {
				return errorx.HandleDBError(err, map[string]any{
					"session_id": history.SessionID,
					"operation":  "update_session_updated_at",
				})
			}
		}

		return nil
	})
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

// FindByUUID retrieves an AgentInstanceSessionHistory by its UUID
func (s *agentInstanceSessionHistoryStoreImpl) FindByUUID(ctx context.Context, uuid string) (*AgentInstanceSessionHistory, error) {
	history := &AgentInstanceSessionHistory{}
	err := s.db.Core.NewSelect().Model(history).Where("uuid = ?", uuid).Scan(ctx, history)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"history_uuid": uuid,
			"operation":    "find_by_uuid",
		})
	}
	return history, nil
}

// ListBySessionID retrieves all AgentInstanceSessionHistory for a specific session
func (s *agentInstanceSessionHistoryStoreImpl) ListBySessionID(ctx context.Context, sessionID int64) ([]AgentInstanceSessionHistory, error) {
	var histories []AgentInstanceSessionHistory
	err := s.db.Core.NewSelect().Model(&histories).Where("session_id = ?", sessionID).Where("is_rewritten = ?", false).Order("turn ASC", "request DESC").Scan(ctx, &histories)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

// Rewrite rewrites an existing AgentInstanceSessionHistory
func (s *agentInstanceSessionHistoryStoreImpl) Rewrite(ctx context.Context, originalUUID string, history *AgentInstanceSessionHistory) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// select original history for update
		originalHistory := &AgentInstanceSessionHistory{}
		err := tx.NewSelect().
			Model((*AgentInstanceSessionHistory)(nil)).
			Where("uuid = ?", originalUUID).
			Where("request = ?", false).
			Where("is_rewritten = ?", false).
			For("UPDATE").
			Scan(ctx, originalHistory)
		if err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"original_uuid": originalUUID,
				"operation":     "select_original_history_for_update",
			})
		}

		originalHistory.IsRewritten = true
		res, err := tx.NewUpdate().Model(originalHistory).Where("id = ?", originalHistory.ID).Exec(ctx)
		if err = assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"original_uuid": originalUUID,
				"operation":     "update_original_history",
			})
		}

		history.Turn = originalHistory.Turn
		res, err = tx.NewInsert().Model(history).Exec(ctx)
		if err = assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, map[string]any{
				"history_uuid": history.UUID,
				"operation":    "insert_history",
			})
		}
		return nil
	})
}
