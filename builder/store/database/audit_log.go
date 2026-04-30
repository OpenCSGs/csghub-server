package database

import (
	"context"
	"encoding/json"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

type auditLogStoreImpl struct {
	db *DB
}

type AuditLogStore interface {
	Create(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, req types.QueryAuditLogReq) (logs []AuditLog, total int, err error)
}

func NewAuditLogStore() AuditLogStore {
	return &auditLogStoreImpl{db: defaultDB}
}

func NewAuditLogStoreWithDB(db *DB) AuditLogStore {
	return &auditLogStoreImpl{
		db: db,
	}
}

type AuditLog struct {
	ID          int64            `bun:"id,pk,autoincrement" json:"id"`
	TableName   string           `bun:"table_name,notnull" json:"table_name"`
	Action      enum.AuditAction `bun:"action,notnull" json:"action"`
	Operator    User             `bun:"rel:belongs-to,join:operator_id=uuid" json:"operator"`
	OperatorID  string           `bun:"operator_id,notnull" json:"operator_id"`
	Before      json.RawMessage  `bun:"before,type:jsonb" json:"before"`
	After       json.RawMessage  `bun:"after,type:jsonb" json:"after"`
	UserName    string           `bun:"user_name,nullzero" json:"user_name"`
	BearerToken string           `bun:"bearer_token,nullzero" json:"bearer_token"`
	IPAddress   string           `bun:"ip_address,nullzero" json:"ip_address"`
	AuthType    string           `bun:"auth_type,nullzero" json:"auth_type"`
	ResourceID  string           `bun:"resource_id,nullzero" json:"resource_id"`

	times
}

func (s *auditLogStoreImpl) Create(ctx context.Context, log *AuditLog) error {
	_, err := s.db.Operator.Core.NewInsert().Model(log).Exec(ctx)
	return err
}

func (s *auditLogStoreImpl) List(ctx context.Context, req types.QueryAuditLogReq) (logs []AuditLog, total int, err error) {
	query := s.db.Operator.Core.NewSelect().Model(&logs)
	countQuery := s.db.Operator.Core.NewSelect().Model((*AuditLog)(nil))

	if req.StartDate != nil {
		query = query.Where("created_at >= ?", *req.StartDate)
		countQuery = countQuery.Where("created_at >= ?", *req.StartDate)
	}
	if req.EndDate != nil {
		query = query.Where("created_at < ?", req.EndDate.AddDate(0, 0, 1))
		countQuery = countQuery.Where("created_at < ?", req.EndDate.AddDate(0, 0, 1))
	}
	if req.UserName != "" {
		query = query.Where("lower(user_name) LIKE lower(?)", "%"+req.UserName+"%")
		countQuery = countQuery.Where("lower(user_name) LIKE lower(?)", "%"+req.UserName+"%")
	}
	if req.Token != "" {
		query = query.Where("bearer_token = ?", req.Token)
		countQuery = countQuery.Where("bearer_token = ?", req.Token)
	}
	if req.Action != "" {
		query = query.Where("action = ?", req.Action)
		countQuery = countQuery.Where("action = ?", req.Action)
	}
	if req.TableName != "" {
		query = query.Where("table_name = ?", req.TableName)
		countQuery = countQuery.Where("table_name = ?", req.TableName)
	}
	if req.AuthType != "" {
		query = query.Where("auth_type = ?", req.AuthType)
		countQuery = countQuery.Where("auth_type = ?", req.AuthType)
	}

	total, err = countQuery.Count(ctx)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, 0, err
	}

	query = query.Order("id DESC")
	if req.Per > 0 {
		query = query.Limit(req.Per)
	}
	if req.Page > 0 && req.Per > 0 {
		query = query.Offset((req.Page - 1) * req.Per)
	}

	err = query.Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}
