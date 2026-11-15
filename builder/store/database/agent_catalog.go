package database

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
	"opencsg.com/csghub-server/common/errorx"
)

type UpdateLogicalAgentInput struct {
	LogicalInstanceID   int64
	OwnerUUID           string
	Name                *string
	Description         *string
	Metadata            *map[string]any
	Public              *bool
	ResolvedIdleTimeout *int
	// RelayAgentID persists the CSGRelay agent id reported for this logical
	// agent. Nil leaves the stored value unchanged.
	RelayAgentID *string
}

type AgentCatalogStore interface {
	CreateLogicalAgent(ctx context.Context, instance *AgentInstance) (*AgentInstance, error)
	UpdateLogical(ctx context.Context, input UpdateLogicalAgentInput) (*AgentInstance, error)
}

type agentCatalogStoreImpl struct {
	db *DB
}

func NewAgentCatalogStore() AgentCatalogStore {
	return &agentCatalogStoreImpl{db: defaultDB}
}

func NewAgentCatalogStoreWithDB(db *DB) AgentCatalogStore {
	return &agentCatalogStoreImpl{db: db}
}

func (s *agentCatalogStoreImpl) CreateLogicalAgent(ctx context.Context, instance *AgentInstance) (*AgentInstance, error) {
	if _, err := s.db.Core.NewInsert().Model(instance).Exec(ctx); err != nil {
		// Classify the owner/kind/name uniqueness violation here so callers get an
		// identifiable agent-domain error instead of matching on index names.
		if agentInstanceNameConflict(err) {
			return nil, errorx.InstanceNameAlreadyExists(err, errorx.Ctx().Set("instance_name", instance.Name))
		}
		return nil, errorx.HandleDBError(err, map[string]any{"operation": "create_logical_instance"})
	}
	return instance, nil
}

// agentInstanceNameConflict reports whether err is a violation of the live
// owner/kind/name uniqueness guarantee for agent instances.
func agentInstanceNameConflict(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) &&
		pgErr.Field('C') == "23505" &&
		pgErr.Field('n') == "agent_instances_live_owner_kind_name"
}

func (s *agentCatalogStoreImpl) UpdateLogical(ctx context.Context, input UpdateLogicalAgentInput) (*AgentInstance, error) {
	result := new(AgentInstance)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		logical, err := lockLogicalAgent(ctx, tx, input.LogicalInstanceID, input.OwnerUUID)
		if err != nil {
			return err
		}
		if err := checkAgentPolicyUpdate(ctx, tx, logical, input.Metadata); err != nil {
			return err
		}
		query := tx.NewUpdate().Model(logical)
		if input.Name != nil {
			query = query.Set("name = ?", *input.Name)
		}
		if input.Description != nil {
			query = query.Set("description = ?", *input.Description)
		}
		if input.Metadata != nil {
			query = query.Set("metadata = ?", *input.Metadata)
		}
		if input.Public != nil {
			query = query.Set("public = ?", *input.Public)
		}
		if input.RelayAgentID != nil {
			query = query.Set("relay_agent_id = ?", *input.RelayAgentID)
		}
		if err := query.WherePK().Returning("*").Scan(ctx, result); err != nil {
			return err
		}
		return updateLogicalAgentDeployTimeouts(ctx, tx, input.LogicalInstanceID, input.ResolvedIdleTimeout)
	})
	if err != nil && !errors.Is(err, errorx.ErrForbidden) && !errors.Is(err, errorx.ErrBadRequest) && !errors.Is(err, errorx.ErrAgentAllocationLifecycleConflict) {
		err = errorx.HandleDBError(err, map[string]any{"operation": "update_logical_instance", "instance_id": input.LogicalInstanceID})
	}
	return result, err
}

func lockLogicalAgent(ctx context.Context, tx bun.Tx, logicalInstanceID int64, ownerUUID string) (*AgentInstance, error) {
	logical := new(AgentInstance)
	if err := tx.NewSelect().Model(logical).Where("id = ?", logicalInstanceID).For("UPDATE").Scan(ctx); err != nil {
		return nil, err
	}
	if logical.UserUUID != ownerUUID {
		return nil, errorx.ErrForbidden
	}
	if logical.Kind != AgentInstanceKindLogical {
		return nil, errorx.BadRequest(errors.New("operation requires a logical agent"), nil)
	}
	if logical.DeletionStartedAt != nil {
		return nil, errorx.BadRequest(errors.New("logical agent deletion is in progress"), nil)
	}
	return logical, nil
}
