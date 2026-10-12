package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	AgentInstanceKindLogical = "logical"
	AgentInstanceKindRuntime = "runtime"

	AgentServingModeReadOnly  = "read_only"
	AgentServingModeReadWrite = "read_write"

	AgentStoragePolicyNone            = "none"
	AgentStoragePolicyActorPersistent = "actor_persistent"

	agentRuntimeResourceSuffixAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	agentRuntimeResourceSuffixLength   = 12
	agentRuntimeResourceNameMaxLength  = 63
)

var (
	ErrAgentExecutionRecoveryRequired = errors.New("agent runtime recovery required for unknown-fenced session")
	ErrAgentExecutionLeaseLost        = errors.New("agent execution lease was lost")
	errAgentAllocationReuseRetry      = errors.New("agent allocation reuse retry required")
)

type AgentAllocation struct {
	ID                 int64      `bun:",pk,autoincrement" json:"id"`
	LogicalInstanceID  int64      `bun:",notnull" json:"logical_instance_id"`
	ActorKind          string     `bun:",notnull" json:"actor_kind"`
	ActorID            string     `bun:",notnull" json:"actor_id"`
	RuntimePrincipal   *string    `bun:"runtime_principal" json:"runtime_principal,omitempty"`
	BillingOwner       *string    `bun:"billing_owner" json:"billing_owner,omitempty"`
	ResourceKey        string     `bun:",notnull" json:"resource_key"`
	DeployID           *int64     `bun:"deploy_id" json:"deploy_id,omitempty"`
	RuntimeGeneration  int64      `bun:",notnull" json:"runtime_generation"`
	AcceptedGeneration *int64     `bun:"accepted_generation" json:"accepted_generation,omitempty"`
	DesiredState       string     `bun:",notnull" json:"desired_state"`
	ObservedState      string     `bun:",notnull" json:"observed_state"`
	OperationSequence  int64      `bun:",notnull" json:"operation_sequence"`
	LastError          *string    `bun:"last_error" json:"last_error,omitempty"`
	IdleExpiresAt      *time.Time `bun:"idle_expires_at" json:"idle_expires_at,omitempty"`
	AbsoluteExpiresAt  *time.Time `bun:"absolute_expires_at" json:"absolute_expires_at,omitempty"`
	BindingEndedAt     *time.Time `bun:"binding_ended_at" json:"binding_ended_at,omitempty"`
	ResourceReleasedAt *time.Time `bun:"resource_released_at" json:"resource_released_at,omitempty"`
	times
}

type AgentAllocationOperation struct {
	ID                     int64          `bun:",pk,autoincrement" json:"id"`
	AllocationID           int64          `bun:",notnull" json:"allocation_id"`
	OperationSequence      int64          `bun:",notnull" json:"operation_sequence"`
	OperationKey           string         `bun:",notnull" json:"operation_key"`
	OperationType          string         `bun:",notnull" json:"operation_type"`
	Status                 string         `bun:",notnull" json:"status"`
	RuntimeGeneration      int64          `bun:",notnull" json:"runtime_generation"`
	Payload                map[string]any `bun:",type:jsonb,notnull" json:"payload"`
	WorkflowID             *string        `bun:"workflow_id" json:"workflow_id,omitempty"`
	DispatchAttempts       int            `bun:",notnull" json:"dispatch_attempts"`
	DispatchLeaseOwner     *string        `bun:"dispatch_lease_owner" json:"dispatch_lease_owner,omitempty"`
	DispatchLeaseExpiresAt *time.Time     `bun:"dispatch_lease_expires_at" json:"dispatch_lease_expires_at,omitempty"`
	DispatchedAt           *time.Time     `bun:"dispatched_at" json:"dispatched_at,omitempty"`
	CompletedAt            *time.Time     `bun:"completed_at" json:"completed_at,omitempty"`
	LastError              *string        `bun:"last_error" json:"last_error,omitempty"`
	times
}

type AgentCapacityCounter struct {
	ScopeKind     string `bun:",pk" json:"scope_kind"`
	ScopeID       string `bun:",pk" json:"scope_id"`
	ResourceKind  string `bun:",pk" json:"resource_kind"`
	CapacityLimit int64  `bun:",notnull" json:"capacity_limit"`
	Reserved      int64  `bun:",notnull" json:"reserved"`
	Consumed      int64  `bun:",notnull" json:"consumed"`
	times
}

type AgentCapacityReservation struct {
	ID           int64      `bun:",pk,autoincrement" json:"id"`
	ScopeKind    string     `bun:",notnull" json:"scope_kind"`
	ScopeID      string     `bun:",notnull" json:"scope_id"`
	ResourceKind string     `bun:",notnull" json:"resource_kind"`
	OperationKey string     `bun:",notnull" json:"operation_key"`
	AllocationID int64      `bun:",notnull" json:"allocation_id"`
	Amount       int64      `bun:",notnull" json:"amount"`
	State        string     `bun:",notnull" json:"state"`
	ReleasedAt   *time.Time `bun:"released_at" json:"released_at,omitempty"`
	times
}

// AgentAllocationSession is the orchestrator's execution-session registry.
// Actor identity is carried by the parent allocation. Product AgentInstanceSession
// rows are the chat-history projection and are not the admission registry.
type AgentAllocationSession struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	AllocationID int64  `bun:",notnull" json:"allocation_id"`
	SessionUUID  string `bun:",notnull" json:"session_uuid"`
	SessionName  string `bun:",nullzero" json:"session_name"`
	times
}

type AgentExecutionAdmission struct {
	ID                int64          `bun:",pk,autoincrement" json:"id"`
	AllocationID      int64          `bun:",notnull" json:"allocation_id"`
	SessionID         *int64         `bun:"session_id" json:"session_id,omitempty"`
	ActorKind         string         `bun:",notnull" json:"actor_kind"`
	ActorID           string         `bun:",notnull" json:"actor_id"`
	LogicalInstanceID int64          `bun:",notnull" json:"logical_instance_id"`
	RuntimeGeneration int64          `bun:",notnull" json:"runtime_generation"`
	ExecutionID       *string        `bun:"execution_id" json:"execution_id,omitempty"`
	UsageReservation  map[string]any `bun:",type:jsonb,notnull" json:"usage_reservation"`
	Status            string         `bun:",notnull" json:"status"`
	HeartbeatAt       *time.Time     `bun:"heartbeat_at" json:"heartbeat_at,omitempty"`
	CompletedAt       *time.Time     `bun:"completed_at" json:"completed_at,omitempty"`
	times
}

// AgentExecutionLease carries ephemeral execution concurrency and capacity
// state. Admissions remain durable history; leases are the only rows that
// expire or get reconciled.
type AgentExecutionLease struct {
	ID                int64      `bun:",pk,autoincrement" json:"id"`
	AdmissionID       int64      `bun:",notnull" json:"admission_id"`
	AllocationID      int64      `bun:",notnull" json:"allocation_id"`
	SessionID         *int64     `bun:"session_id" json:"session_id,omitempty"`
	HeartbeatAt       time.Time  `bun:",notnull" json:"heartbeat_at"`
	CapacityHoldUntil *time.Time `bun:"capacity_hold_until" json:"capacity_hold_until,omitempty"`
	EndedAt           *time.Time `bun:"ended_at" json:"ended_at,omitempty"`
	EndReason         *string    `bun:"end_reason" json:"end_reason,omitempty"`
	times
}

type ReserveAgentAllocationInput struct {
	LogicalInstanceID int64
	ActorKind         string
	ActorID           string
	RuntimePrincipal  string
	BillingOwner      string
	InstanceQuota     int64
	SessionUUID       string
	SessionName       string
}

type ReserveAgentAllocationResult struct {
	Allocation AgentAllocation
	Operation  *AgentAllocationOperation
	Created    bool
}

type BeginLogicalAgentDeletionResult struct {
	Logical    AgentInstance
	Operations []AgentAllocationOperation
	Pending    bool
}

type ClaimAgentAllocationOperationsInput struct {
	Owner       string
	Limit       int
	Lease       time.Duration
	OperationID int64
}

type AgentOrchestratorStore interface {
	BeginLogicalAgentDeletion(ctx context.Context, logicalInstanceID int64, ownerUUID string) (*BeginLogicalAgentDeletionResult, error)
	ReserveAllocation(ctx context.Context, input ReserveAgentAllocationInput) (*ReserveAgentAllocationResult, error)
	FindAllocationByID(ctx context.Context, allocationID int64) (*AgentAllocation, error)
	FindCallerAllocation(ctx context.Context, logicalInstanceID int64, actorKind, actorID string) (*AgentAllocation, error)
	FindLiveAllocationSession(ctx context.Context, logicalInstanceID int64, actorKind, actorID, sessionUUID string) (*AgentAllocationSession, error)
	FindLiveAllocationByResourceKey(ctx context.Context, resourceKey string) (*AgentAllocation, error)
	ClaimPendingOperations(ctx context.Context, input ClaimAgentAllocationOperationsInput) ([]AgentAllocationOperation, error)
	MarkOperationDispatched(ctx context.Context, operationID int64, owner, workflowID string) error
	FindOperationContext(ctx context.Context, operationID int64) (*AgentAllocationOperationContext, error)
	MarkProvisionOperationRunning(ctx context.Context, operationID int64) error
	CompleteProvisionOperation(ctx context.Context, operationID, deployID int64) error
	FailProvisionOperation(ctx context.Context, operationID int64, message string, releaseReservation bool) error
	MarkDeleteOperationRunning(ctx context.Context, operationID int64) error
	CompleteDeleteOperation(ctx context.Context, operationID, deployID int64) error
	FailDeleteOperation(ctx context.Context, operationID int64, message string) error
	AcquireExecutionAdmission(ctx context.Context, input AcquireAgentExecutionAdmissionInput) (*AgentExecutionAdmission, error)
	MarkExecutionAdmissionRunning(ctx context.Context, admissionID int64) error
	CompleteExecutionAdmission(ctx context.Context, admissionID int64, status string, unknownCapacityHold time.Duration) error
	HeartbeatExecutionAdmission(ctx context.Context, admissionID int64) error
	BeginAllocationRecovery(ctx context.Context, allocationID int64, actor commontypes.AgentActor) (*AgentAllocationOperation, error)
	MarkRestartOperationRunning(ctx context.Context, operationID int64) error
	CompleteRestartOperation(ctx context.Context, operationID, deployID int64) error
	FailRestartOperation(ctx context.Context, operationID int64, message string) error
	RecordAllocationSuspended(ctx context.Context, deployID int64) error
	BeginAllocationResume(ctx context.Context, allocationID int64, actor commontypes.AgentActor) (*AgentAllocationOperation, error)
	MarkResumeOperationRunning(ctx context.Context, operationID int64) error
	CompleteResumeOperation(ctx context.Context, operationID, deployID int64) error
	FailResumeOperation(ctx context.Context, operationID int64, message string) error
}

type AcquireAgentExecutionAdmissionInput struct {
	AllocationID               int64
	ActorID                    string
	ActorKind                  string
	SessionUUID                string
	SessionName                string
	ExecutionID                string
	ExpectedRuntimePrincipal   string
	PinnedRuntimeGeneration    int64
	MaxConcurrentPerAllocation int64
	ReservedTimeout            time.Duration
	HeartbeatTimeout           time.Duration
	UnknownCapacityHold        time.Duration
	GuestUsageMaxCount         *int64
}

type AgentAllocationOperationContext struct {
	Operation  AgentAllocationOperation
	Allocation AgentAllocation
	Logical    AgentInstance
}

type agentOrchestratorStoreImpl struct {
	db *DB
}

func NewAgentOrchestratorStore() AgentOrchestratorStore {
	return &agentOrchestratorStoreImpl{db: defaultDB}
}

func NewAgentOrchestratorStoreWithDB(db *DB) AgentOrchestratorStore {
	return &agentOrchestratorStoreImpl{db: db}
}

func (s *agentOrchestratorStoreImpl) BeginLogicalAgentDeletion(
	ctx context.Context,
	logicalInstanceID int64,
	ownerUUID string,
) (*BeginLogicalAgentDeletionResult, error) {
	if logicalInstanceID <= 0 || ownerUUID == "" {
		return nil, errorx.BadRequest(fmt.Errorf("invalid logical delete request"), nil)
	}
	result := new(BeginLogicalAgentDeletionResult)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		logical := new(AgentInstance)
		if err := tx.NewSelect().Model(logical).WhereAllWithDeleted().Where("id = ?", logicalInstanceID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if logical.UserUUID != ownerUUID {
			return errorx.ErrForbidden
		}
		if logical.Kind != AgentInstanceKindLogical {
			return errorx.BadRequest(fmt.Errorf("delete requires a logical agent"), nil)
		}
		if !logical.DeletedAt.IsZero() || logical.DeletionStartedAt != nil {
			result.Logical = *logical
			if !logical.DeletedAt.IsZero() {
				return nil
			}
			if err := loadLogicalDeleteOperations(ctx, tx, result, logical.ID); err != nil {
				return err
			}
			if !result.Pending {
				// All allocations are already cleaned up but the tombstone was
				// never written; repair it so retries converge. The row lock held
				// since the initial select keeps the soft delete single-row.
				res, err := tx.NewDelete().Model((*AgentInstance)(nil)).
					Where("id = ? AND kind = ?", logical.ID, AgentInstanceKindLogical).Exec(ctx)
				if err := assertAffectedOneRow(res, err); err != nil {
					return err
				}
				logical.DeletedAt = time.Now()
				result.Logical = *logical
			}
			return nil
		}

		allocations, err := lockLogicalAllocations(ctx, tx, logical.ID)
		if err != nil {
			return err
		}
		now := time.Now()
		logical.Public = false
		logical.DeletionStartedAt = &now
		logical.UpdatedAt = now
		logicalUpdate := tx.NewUpdate().Model(logical).
			Set("public = FALSE").Set("deletion_started_at = ?", now).
			Set("updated_at = ?", now)
		if len(allocations) == 0 {
			logical.DeletedAt = now
			logicalUpdate = logicalUpdate.Set("deleted_at = ?", now)
		}
		if err := assertAffectedOneRow(logicalUpdate.WherePK().Exec(ctx)); err != nil {
			return err
		}
		result.Logical = *logical
		result.Pending = len(allocations) != 0
		for i := range allocations {
			operation, err := beginAllocationDelete(ctx, tx, &allocations[i])
			if err != nil {
				return err
			}
			result.Operations = append(result.Operations, *operation)
		}
		return nil
	})
	return result, err
}

func lockLogicalAllocations(ctx context.Context, tx bun.Tx, logicalInstanceID int64) ([]AgentAllocation, error) {
	var allocations []AgentAllocation
	err := tx.NewSelect().Model(&allocations).
		Where("logical_instance_id = ? AND binding_ended_at IS NULL", logicalInstanceID).
		OrderExpr("agent_allocation.id ASC").For("UPDATE OF agent_allocation").Scan(ctx)
	return allocations, err
}

func beginAllocationDelete(ctx context.Context, tx bun.Tx, allocation *AgentAllocation) (*AgentAllocationOperation, error) {
	runtimeGeneration := allocation.RuntimeGeneration + 1
	operationSequence := allocation.OperationSequence + 1
	operation := &AgentAllocationOperation{
		AllocationID: allocation.ID, OperationSequence: operationSequence,
		OperationKey:  agentAllocationOperationKey(allocation.ResourceKey, operationSequence),
		OperationType: "delete", Status: "pending", RuntimeGeneration: runtimeGeneration,
		Payload: map[string]any{},
	}
	if _, err := tx.NewUpdate().Model((*AgentAllocationOperation)(nil)).
		Set("status = 'canceled'").Set("completed_at = CURRENT_TIMESTAMP").
		Where("allocation_id = ? AND status IN ('pending', 'dispatching', 'dispatched')", allocation.ID).Exec(ctx); err != nil {
		return nil, err
	}
	if _, err := tx.NewUpdate().Model(allocation).
		Set("desired_state = 'released'").Set("observed_state = 'deleting'").
		Set("operation_sequence = ?", operationSequence).Set("runtime_generation = ?", runtimeGeneration).
		Set("last_error = NULL").WherePK().Exec(ctx); err != nil {
		return nil, err
	}
	if _, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
		Set("status = 'canceled'").Set("completed_at = CURRENT_TIMESTAMP").
		Where("allocation_id = ? AND status IN ('reserved', 'running', 'unknown')", allocation.ID).Exec(ctx); err != nil {
		return nil, err
	}
	if _, err := tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
		Set("ended_at = CURRENT_TIMESTAMP").Set("end_reason = 'canceled'").Set("updated_at = CURRENT_TIMESTAMP").
		Where("allocation_id = ? AND ended_at IS NULL", allocation.ID).Exec(ctx); err != nil {
		return nil, err
	}
	if _, err := tx.NewInsert().Model(operation).Exec(ctx); err != nil {
		return nil, err
	}
	return operation, nil
}

func loadLogicalDeleteOperations(ctx context.Context, tx bun.Tx, result *BeginLogicalAgentDeletionResult, logicalInstanceID int64) error {
	if err := tx.NewSelect().Model(&result.Operations).
		Join("JOIN agent_allocations AS allocation ON allocation.id = agent_allocation_operation.allocation_id").
		Where("allocation.logical_instance_id = ? AND allocation.binding_ended_at IS NULL", logicalInstanceID).
		Where("agent_allocation_operation.operation_type = 'delete'").Scan(ctx); err != nil {
		return err
	}
	result.Pending = len(result.Operations) != 0
	return nil
}

func (s *agentOrchestratorStoreImpl) ReserveAllocation(ctx context.Context, input ReserveAgentAllocationInput) (*ReserveAgentAllocationResult, error) {
	if err := validateReserveAgentAllocationInput(input); err != nil {
		return nil, err
	}
	result, err := s.reserveAllocationAttempt(ctx, input)
	if errors.Is(err, errAgentAllocationReuseRetry) {
		return s.recoverConcurrentAllocation(ctx, input)
	}
	return result, err
}

func (s *agentOrchestratorStoreImpl) reserveAllocationAttempt(ctx context.Context, input ReserveAgentAllocationInput) (*ReserveAgentAllocationResult, error) {
	result := new(ReserveAgentAllocationResult)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		logical, err := lockUsableLogicalAgent(ctx, tx, input.LogicalInstanceID, input.ActorID)
		if err != nil {
			return err
		}
		existing, err := lockLiveAllocation(ctx, tx, input.LogicalInstanceID, input.ActorKind, input.ActorID)
		if err == nil {
			return loadReservedAllocationResult(ctx, tx, input, existing, result)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		counter, err := lockAgentRuntimeCounter(ctx, tx, input.RuntimePrincipal, input.InstanceQuota)
		if err != nil {
			return err
		}
		// Recheck without locking the allocation while the counter is held.
		if _, err := findLiveAllocation(ctx, tx, input.LogicalInstanceID, input.ActorKind, input.ActorID); err == nil {
			return errAgentAllocationReuseRetry
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if counter.Reserved+counter.Consumed >= input.InstanceQuota {
			return errorx.InstanceQuotaExceeded(nil, errorx.Ctx().Set("instance_count", counter.Reserved+counter.Consumed).Set("quota", input.InstanceQuota))
		}
		if err := insertReservedAllocation(ctx, tx, input, logical, counter, result); err != nil {
			if errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
				return errAgentAllocationReuseRetry
			}
			return err
		}
		return nil
	})
	return result, err
}

func lockUsableLogicalAgent(ctx context.Context, tx bun.Tx, logicalInstanceID int64, actorID string) (*AgentInstance, error) {
	logical := new(AgentInstance)
	if err := tx.NewSelect().Model(logical).Where("id = ?", logicalInstanceID).For("UPDATE").Scan(ctx); err != nil {
		return nil, err
	}
	if logical.Kind != AgentInstanceKindLogical {
		return nil, errorx.BadRequest(fmt.Errorf("use requires a logical agent instance"), nil)
	}
	if logical.DeletionStartedAt != nil {
		return nil, errorx.AgentAllocationLifecycleConflict(fmt.Errorf("logical agent deletion is in progress"), nil)
	}
	if logical.UserUUID != actorID && !logical.Public {
		return nil, errorx.ErrForbidden
	}
	return logical, nil
}

func lockLiveAllocation(ctx context.Context, tx bun.Tx, logicalInstanceID int64, actorKind, actorID string) (*AgentAllocation, error) {
	allocation := new(AgentAllocation)
	err := tx.NewSelect().Model(allocation).
		Where("logical_instance_id = ? AND actor_kind = ? AND actor_id = ? AND binding_ended_at IS NULL", logicalInstanceID, actorKind, actorID).
		For("UPDATE").Scan(ctx)
	return allocation, err
}

func loadReservedAllocationResult(ctx context.Context, db bun.IDB, input ReserveAgentAllocationInput, allocation *AgentAllocation, result *ReserveAgentAllocationResult) error {
	if err := validateReusableAllocation(allocation); err != nil {
		return err
	}
	if allocation.ObservedState == "failed" {
		operation, err := insertProvisionRetryOperation(ctx, db, allocation)
		if err != nil {
			return err
		}
		result.Operation = operation
	}
	result.Allocation = *allocation
	if input.SessionUUID != "" {
		return reserveAllocationSession(ctx, db, input, allocation)
	}
	return nil
}

func validateReusableAllocation(allocation *AgentAllocation) error {
	if allocation == nil {
		return errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation is required"), nil)
	}
	if allocation.DesiredState != "running" || allocation.ObservedState == "deleting" {
		return errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation cannot be reused in state %s/%s", allocation.DesiredState, allocation.ObservedState), nil)
	}
	return nil
}

func insertProvisionRetryOperation(ctx context.Context, db bun.IDB, allocation *AgentAllocation) (*AgentAllocationOperation, error) {
	latest := new(AgentAllocationOperation)
	if err := db.NewSelect().Model(latest).
		Where("allocation_id = ? AND operation_type = 'provision'", allocation.ID).
		OrderExpr("operation_sequence DESC").Limit(1).Scan(ctx); err != nil {
		return nil, err
	}
	if latest.Status != "failed" {
		return nil, nil
	}
	sequence := allocation.OperationSequence + 1
	operation := &AgentAllocationOperation{
		AllocationID: allocation.ID, OperationSequence: sequence,
		OperationKey:  agentAllocationOperationKey(allocation.ResourceKey, sequence),
		OperationType: "provision", Status: "pending",
		// Target the allocation's current generation: a restart may have
		// advanced it past the last provision operation.
		RuntimeGeneration: allocation.RuntimeGeneration, Payload: map[string]any{},
	}
	if _, err := db.NewInsert().Model(operation).Exec(ctx); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{"operation": "retry_allocation_provision"})
	}
	if _, err := db.NewUpdate().Model(allocation).
		Set("operation_sequence = ?", sequence).Set("observed_state = 'pending'").Set("last_error = NULL").
		WherePK().Exec(ctx); err != nil {
		return nil, err
	}
	allocation.OperationSequence = sequence
	allocation.ObservedState = "pending"
	allocation.LastError = nil
	return operation, nil
}

func lockAgentRuntimeCounter(ctx context.Context, tx bun.Tx, actorID string, limit int64) (*AgentCapacityCounter, error) {
	counter := &AgentCapacityCounter{ScopeKind: "user", ScopeID: actorID, ResourceKind: "agent_runtime_instance", CapacityLimit: limit}
	if _, err := tx.NewInsert().Model(counter).Column("scope_kind", "scope_id", "resource_kind", "capacity_limit").
		On("CONFLICT (scope_kind, scope_id, resource_kind) DO NOTHING").Exec(ctx); err != nil {
		return nil, fmt.Errorf("initialize agent capacity counter: %w", err)
	}
	if err := tx.NewSelect().Model(counter).
		Where("scope_kind = ? AND scope_id = ? AND resource_kind = ?", counter.ScopeKind, counter.ScopeID, counter.ResourceKind).
		For("UPDATE").Scan(ctx); err != nil {
		return nil, fmt.Errorf("lock agent capacity counter: %w", err)
	}
	effectiveLimit := limit
	if usage := counter.Reserved + counter.Consumed; effectiveLimit < usage {
		effectiveLimit = usage
	}
	if counter.CapacityLimit != effectiveLimit {
		if _, err := tx.NewUpdate().Model(counter).Set("capacity_limit = ?", effectiveLimit).WherePK().Exec(ctx); err != nil {
			return nil, fmt.Errorf("refresh agent capacity counter limit: %w", err)
		}
		counter.CapacityLimit = effectiveLimit
	}
	return counter, nil
}

func insertReservedAllocation(ctx context.Context, tx bun.Tx, input ReserveAgentAllocationInput, logical *AgentInstance, counter *AgentCapacityCounter, result *ReserveAgentAllocationResult) error {
	allocationID, err := nextAgentAllocationID(ctx, tx)
	if err != nil {
		return err
	}
	resourceKey, err := newAgentRuntimeResourceKey(logical.ContentID)
	if err != nil {
		return err
	}
	principal, billingOwner := input.RuntimePrincipal, input.BillingOwner
	allocation := AgentAllocation{
		ID: allocationID, LogicalInstanceID: input.LogicalInstanceID, ActorKind: input.ActorKind, ActorID: input.ActorID,
		RuntimePrincipal: &principal, BillingOwner: &billingOwner,
		ResourceKey: resourceKey, DesiredState: "running", ObservedState: "pending", OperationSequence: 1,
	}
	if _, err := tx.NewInsert().Model(&allocation).Column(
		"id", "logical_instance_id", "actor_kind", "actor_id", "runtime_principal", "billing_owner",
		"resource_key", "desired_state", "observed_state", "operation_sequence",
	).Exec(ctx); err != nil {
		return errorx.HandleDBError(err, map[string]any{"operation": "reserve_allocation"})
	}
	operation, err := insertInitialProvisionOperation(ctx, tx, counter, &allocation)
	if err != nil {
		return err
	}
	if input.SessionUUID != "" {
		if err := reserveAllocationSession(ctx, tx, input, &allocation); err != nil {
			return err
		}
	}
	result.Allocation, result.Operation, result.Created = allocation, operation, true
	return nil
}

func nextAgentAllocationID(ctx context.Context, tx bun.Tx) (int64, error) {
	var allocationID int64
	if err := tx.NewRaw("SELECT nextval(pg_get_serial_sequence('agent_allocations', 'id'))").Scan(ctx, &allocationID); err != nil {
		return 0, fmt.Errorf("reserve allocation identity: %w", err)
	}
	return allocationID, nil
}

var agentRuntimeResourceInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)
var agentRuntimeResourceSuffixPattern = regexp.MustCompile(`^[a-z0-9]+$`)

func newAgentRuntimeResourceKey(logicalContentID string) (string, error) {
	suffix, err := gonanoid.Generate(
		agentRuntimeResourceSuffixAlphabet,
		agentRuntimeResourceSuffixLength,
	)
	if err != nil {
		return "", fmt.Errorf("generate agent runtime resource suffix: %w", err)
	}
	return formatAgentRuntimeResourceKey(logicalContentID, suffix)
}

func formatAgentRuntimeResourceKey(logicalContentID, suffix string) (string, error) {
	if len(suffix) != agentRuntimeResourceSuffixLength || !agentRuntimeResourceSuffixPattern.MatchString(suffix) {
		return "", fmt.Errorf("invalid agent runtime resource suffix")
	}
	prefix := agentRuntimeResourceInvalidChars.ReplaceAllString(strings.ToLower(logicalContentID), "-")
	prefix = strings.Trim(prefix, "-")
	maxPrefixLength := agentRuntimeResourceNameMaxLength -
		len(commontypes.AgentOrchestratedRuntimeResourcePrefix) - len(suffix) - 1
	if len(prefix) > maxPrefixLength {
		prefix = strings.Trim(prefix[:maxPrefixLength], "-")
	}
	if prefix == "" {
		prefix = "agent"
	}
	return commontypes.AgentOrchestratedRuntimeResourcePrefix + prefix + "-" + suffix, nil
}

func agentAllocationOperationKey(resourceKey string, sequence int64) string {
	return fmt.Sprintf("agent-allocation/%s/%d", resourceKey, sequence)
}

func insertInitialProvisionOperation(ctx context.Context, tx bun.Tx, counter *AgentCapacityCounter, allocation *AgentAllocation) (*AgentAllocationOperation, error) {
	operationKey := agentAllocationOperationKey(allocation.ResourceKey, 1)
	reservation := AgentCapacityReservation{
		ScopeKind: counter.ScopeKind, ScopeID: counter.ScopeID, ResourceKind: counter.ResourceKind,
		OperationKey: operationKey, AllocationID: allocation.ID, Amount: 1, State: "reserved",
	}
	if _, err := tx.NewInsert().Model(&reservation).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create agent capacity reservation: %w", err)
	}
	if _, err := tx.NewUpdate().Model(counter).Set("reserved = reserved + 1").WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("increment agent capacity counter: %w", err)
	}
	operation := &AgentAllocationOperation{
		AllocationID: allocation.ID, OperationSequence: 1, OperationKey: operationKey,
		OperationType: "provision", Status: "pending", RuntimeGeneration: allocation.RuntimeGeneration, Payload: map[string]any{},
	}
	if _, err := tx.NewInsert().Model(operation).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create allocation operation: %w", err)
	}
	return operation, nil
}

func (s *agentOrchestratorStoreImpl) recoverConcurrentAllocation(ctx context.Context, input ReserveAgentAllocationInput) (*ReserveAgentAllocationResult, error) {
	result := new(ReserveAgentAllocationResult)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := lockUsableLogicalAgent(ctx, tx, input.LogicalInstanceID, input.ActorID); err != nil {
			return err
		}
		existing, err := lockLiveAllocation(ctx, tx, input.LogicalInstanceID, input.ActorKind, input.ActorID)
		if errors.Is(err, sql.ErrNoRows) {
			return errorx.AgentAllocationLifecycleConflict(fmt.Errorf("live allocation disappeared during concurrent reservation"), nil)
		}
		if err != nil {
			return err
		}
		return loadReservedAllocationResult(ctx, tx, input, existing, result)
	})
	return result, err
}

func normalizedActorKind(kind string) string {
	if kind == "" {
		return string(commontypes.AgentActorKindUser)
	}
	return kind
}

func consumeGuestUsage(ctx context.Context, tx bun.IDB, logicalInstanceID int64, guestActorID string, maxCount *int64) error {
	if maxCount == nil {
		return nil
	}
	if *maxCount <= 0 {
		return errorx.AgentGuestUsageLimitExceeded(nil, errorx.Ctx().
			Set("logical_instance_id", logicalInstanceID).
			Set("max_count", *maxCount))
	}
	res, err := tx.NewRaw(`
		INSERT INTO agent_guest_usage_counters (logical_instance_id, guest_actor_id, use_count)
		VALUES (?, ?, 1)
		ON CONFLICT (logical_instance_id, guest_actor_id) DO UPDATE
		SET use_count = agent_guest_usage_counters.use_count + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE agent_guest_usage_counters.use_count < ?
	`, logicalInstanceID, guestActorID, *maxCount).Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errorx.AgentGuestUsageLimitExceeded(nil, errorx.Ctx().
			Set("logical_instance_id", logicalInstanceID).
			Set("max_count", *maxCount))
	}
	return nil
}

func validateReserveAgentAllocationInput(input ReserveAgentAllocationInput) error {
	if input.LogicalInstanceID <= 0 || input.ActorID == "" || input.RuntimePrincipal == "" || input.InstanceQuota <= 0 {
		return errorx.BadRequest(fmt.Errorf("invalid allocation reservation input"), nil)
	}
	if input.ActorKind != "user" && input.ActorKind != "guest" {
		return errorx.BadRequest(fmt.Errorf("unsupported allocation actor kind"), nil)
	}
	return nil
}

func findLiveAllocation(ctx context.Context, tx bun.IDB, logicalInstanceID int64, actorKind, actorID string) (*AgentAllocation, error) {
	allocation := new(AgentAllocation)
	err := tx.NewSelect().Model(allocation).
		Where("logical_instance_id = ? AND actor_kind = ? AND actor_id = ? AND binding_ended_at IS NULL", logicalInstanceID, actorKind, actorID).
		Scan(ctx)
	return allocation, err
}

func reserveAllocationSession(ctx context.Context, tx bun.IDB, input ReserveAgentAllocationInput, allocation *AgentAllocation) error {
	_, err := ensureAllocationSession(ctx, tx, allocation, input.SessionUUID, input.SessionName)
	return err
}

func tryRebindReleasedAllocationSession(
	ctx context.Context,
	tx bun.IDB,
	allocation *AgentAllocation,
	sessionUUID, sessionName string,
) (*AgentAllocationSession, bool, error) {
	session := new(AgentAllocationSession)
	err := tx.NewSelect().
		TableExpr("agent_allocation_sessions AS session").
		ColumnExpr("session.*").
		Join("JOIN agent_allocations AS allocation ON allocation.id = session.allocation_id").
		Where("session.session_uuid = ?", sessionUUID).
		Where("allocation.actor_kind = ? AND allocation.actor_id = ?", allocation.ActorKind, allocation.ActorID).
		Where("allocation.logical_instance_id = ?", allocation.LogicalInstanceID).
		Where("allocation.observed_state = 'failed'").
		Where("allocation.binding_ended_at IS NOT NULL").
		Where("allocation.resource_released_at IS NOT NULL").
		OrderExpr("session.id DESC").
		Limit(1).
		Scan(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	exists, err := tx.NewSelect().Model((*AgentExecutionAdmission)(nil)).Where("session_id = ?", session.ID).Exists(ctx)
	if err != nil {
		return nil, false, err
	}
	if exists {
		return nil, false, nil
	}
	update := tx.NewUpdate().Model(session).
		Set("allocation_id = ?", allocation.ID).
		Set("updated_at = CURRENT_TIMESTAMP").
		WherePK()
	if sessionName != "" {
		update = update.Set("session_name = ?", sessionName)
	}
	if err := assertAffectedOneRow(update.Exec(ctx)); err != nil {
		return nil, false, err
	}
	session.AllocationID = allocation.ID
	if sessionName != "" {
		session.SessionName = sessionName
	}
	return session, true, nil
}

// ensureAllocationSession binds sessionUUID to allocation. Callers must hold a
// lock on allocation (or otherwise serialize writers for that allocation row).
func ensureAllocationSession(
	ctx context.Context,
	tx bun.IDB,
	allocation *AgentAllocation,
	sessionUUID, sessionName string,
) (int64, error) {
	session := new(AgentAllocationSession)
	err := tx.NewSelect().Model(session).
		Where("allocation_id = ? AND session_uuid = ?", allocation.ID, sessionUUID).
		Scan(ctx)
	if err == nil {
		return session.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	rebound, ok, rebindErr := tryRebindReleasedAllocationSession(ctx, tx, allocation, sessionUUID, sessionName)
	if rebindErr != nil {
		return 0, rebindErr
	}
	if ok {
		return rebound.ID, nil
	}
	session = &AgentAllocationSession{
		AllocationID: allocation.ID,
		SessionUUID:  sessionUUID,
		SessionName:  sessionName,
	}
	if _, err := tx.NewInsert().Model(session).
		On("CONFLICT (allocation_id, session_uuid) DO NOTHING").
		Exec(ctx); err != nil {
		return 0, err
	}
	existing := new(AgentAllocationSession)
	if err := tx.NewSelect().Model(existing).
		Where("allocation_id = ? AND session_uuid = ?", allocation.ID, sessionUUID).
		Scan(ctx); err != nil {
		return 0, err
	}
	return existing.ID, nil
}

func ensureExecutionSession(ctx context.Context, tx bun.Tx, input AcquireAgentExecutionAdmissionInput, allocation *AgentAllocation) (int64, error) {
	return ensureAllocationSession(ctx, tx, allocation, input.SessionUUID, input.SessionName)
}

func (s *agentOrchestratorStoreImpl) FindAllocationByID(ctx context.Context, allocationID int64) (*AgentAllocation, error) {
	allocation := new(AgentAllocation)
	err := s.db.Core.NewSelect().Model(allocation).Where("id = ?", allocationID).Scan(ctx)
	return allocation, err
}

func (s *agentOrchestratorStoreImpl) FindCallerAllocation(ctx context.Context, logicalInstanceID int64, actorKind, actorID string) (*AgentAllocation, error) {
	if logicalInstanceID <= 0 || actorKind == "" || actorID == "" {
		return nil, errorx.BadRequest(fmt.Errorf("invalid caller allocation lookup"), nil)
	}
	live, err := findLiveAllocation(ctx, s.db.Core, logicalInstanceID, actorKind, actorID)
	if err == nil {
		return live, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	allocation := new(AgentAllocation)
	err = s.db.Core.NewSelect().Model(allocation).
		Where("logical_instance_id = ? AND actor_kind = ? AND actor_id = ? AND binding_ended_at IS NOT NULL", logicalInstanceID, actorKind, actorID).
		OrderExpr("binding_ended_at DESC, id DESC").Limit(1).Scan(ctx)
	return allocation, err
}

// FindLiveAllocationSession returns the caller's live allocation session.
// Ended bindings are excluded so chat history cannot attach to a released runtime.
func (s *agentOrchestratorStoreImpl) FindLiveAllocationSession(
	ctx context.Context,
	logicalInstanceID int64,
	actorKind, actorID, sessionUUID string,
) (*AgentAllocationSession, error) {
	if logicalInstanceID <= 0 || actorKind == "" || actorID == "" || sessionUUID == "" {
		return nil, errorx.BadRequest(fmt.Errorf("invalid live allocation session lookup"), nil)
	}
	session := new(AgentAllocationSession)
	err := s.db.Core.NewSelect().
		TableExpr("agent_allocation_sessions AS session").
		ColumnExpr("session.*").
		Join("JOIN agent_allocations AS allocation ON allocation.id = session.allocation_id").
		Where("allocation.logical_instance_id = ?", logicalInstanceID).
		Where("allocation.actor_kind = ? AND allocation.actor_id = ?", actorKind, actorID).
		Where("allocation.binding_ended_at IS NULL").
		Where("session.session_uuid = ?", sessionUUID).
		Limit(1).
		Scan(ctx, session)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"logical_instance_id": logicalInstanceID,
			"actor_kind":          actorKind,
			"session_uuid":        sessionUUID,
			"operation":           "find_live_allocation_session",
		})
	}
	return session, nil
}

func (s *agentOrchestratorStoreImpl) FindLiveAllocationByResourceKey(ctx context.Context, resourceKey string) (*AgentAllocation, error) {
	allocation := new(AgentAllocation)
	err := s.db.Core.NewSelect().Model(allocation).
		Where("resource_key = ? AND binding_ended_at IS NULL", resourceKey).Scan(ctx)
	return allocation, err
}

func (s *agentOrchestratorStoreImpl) AcquireExecutionAdmission(ctx context.Context, input AcquireAgentExecutionAdmissionInput) (*AgentExecutionAdmission, error) {
	if input.AllocationID <= 0 || input.ActorID == "" || input.SessionUUID == "" || input.ExecutionID == "" {
		return nil, errorx.BadRequest(fmt.Errorf("invalid execution admission request"), nil)
	}
	if input.MaxConcurrentPerAllocation <= 0 || input.ReservedTimeout <= 0 || input.HeartbeatTimeout <= 0 || input.UnknownCapacityHold <= 0 {
		return nil, errorx.BadRequest(fmt.Errorf("invalid execution admission limits"), nil)
	}
	admission := new(AgentExecutionAdmission)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		allocation, sessionID, err := loadExecutionAdmissionContext(ctx, tx, input)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := reconcileStaleExecutionLeases(ctx, tx, allocation.ID, input, now); err != nil {
			return err
		}
		if err := checkSessionExecutionLease(ctx, tx, allocation.ID, sessionID); err != nil {
			return err
		}
		if err := checkAllocationExecutionCapacity(ctx, tx, allocation.ID, input, now); err != nil {
			return err
		}
		if err := consumeGuestUsage(ctx, tx, allocation.LogicalInstanceID, input.ActorID, input.GuestUsageMaxCount); err != nil {
			return err
		}
		executionID := input.ExecutionID
		*admission = AgentExecutionAdmission{
			AllocationID: allocation.ID, SessionID: &sessionID,
			ActorKind: normalizedActorKind(input.ActorKind), ActorID: input.ActorID, LogicalInstanceID: allocation.LogicalInstanceID,
			RuntimeGeneration: allocation.RuntimeGeneration, ExecutionID: &executionID,
			UsageReservation: map[string]any{}, Status: "reserved",
		}
		if _, err := tx.NewInsert().Model(admission).Exec(ctx); err != nil {
			return err
		}
		lease := &AgentExecutionLease{
			AdmissionID: admission.ID, AllocationID: allocation.ID, SessionID: &sessionID,
			HeartbeatAt: now,
		}
		_, err = tx.NewInsert().Model(lease).Exec(ctx)
		return err
	})
	return admission, err
}

// reconcileStaleExecutionLeases closes leases whose admission reached a
// terminal status, cancels stale reserved admissions, and transitions stale
// running admissions to unknown with a bounded capacity hold. Callers must
// hold the allocation row lock.
//
// Lock order is admission row, then lease row. A stale candidate is re-read
// under those locks before it is changed. In PostgreSQL READ COMMITTED, a
// statement that waits on the admission row cannot safely use its original
// snapshot to judge a heartbeat stored on the separate lease row.
func reconcileStaleExecutionLeases(ctx context.Context, tx bun.Tx, allocationID int64, input AcquireAgentExecutionAdmissionInput, now time.Time) error {
	var openLeases []AgentExecutionLease
	if err := tx.NewSelect().Model(&openLeases).
		Where("allocation_id = ? AND ended_at IS NULL", allocationID).Scan(ctx); err != nil {
		return err
	}
	if len(openLeases) == 0 {
		return nil
	}
	admissionIDs := make([]int64, len(openLeases))
	for i := range openLeases {
		admissionIDs[i] = openLeases[i].AdmissionID
	}
	admissions := make(map[int64]string, len(openLeases))
	var rows []AgentExecutionAdmission
	if err := tx.NewSelect().Model(&rows).Where("id IN (?)", bun.In(admissionIDs)).Scan(ctx); err != nil {
		return err
	}
	for i := range rows {
		admissions[rows[i].ID] = rows[i].Status
	}
	for i := range openLeases {
		lease := &openLeases[i]
		status := admissions[lease.AdmissionID]
		if status == "unknown" || (status == "reserved" && now.Sub(lease.CreatedAt) <= input.ReservedTimeout) ||
			(status == "running" && now.Sub(lease.HeartbeatAt) <= input.HeartbeatTimeout) {
			continue
		}
		if err := reconcileStaleExecutionLease(ctx, tx, lease, input, now); err != nil {
			return err
		}
	}
	return nil
}

// reconcileStaleExecutionLease serializes reconciliation with heartbeat and
// completion writers. The lease is deliberately re-read in a statement issued
// after the admission lock has been acquired.
func reconcileStaleExecutionLease(ctx context.Context, tx bun.Tx, candidate *AgentExecutionLease, input AcquireAgentExecutionAdmissionInput, now time.Time) error {
	admission := new(AgentExecutionAdmission)
	if err := tx.NewSelect().Model(admission).Where("id = ?", candidate.AdmissionID).For("UPDATE").Scan(ctx); err != nil {
		return err
	}

	lease := new(AgentExecutionLease)
	err := tx.NewSelect().Model(lease).
		Where("id = ? AND ended_at IS NULL", candidate.ID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	switch admission.Status {
	case "reserved":
		if now.Sub(lease.CreatedAt) <= input.ReservedTimeout {
			return nil
		}
		if _, err := tx.NewUpdate().Model(admission).
			Set("status = 'canceled'").Set("completed_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			WherePK().Exec(ctx); err != nil {
			return err
		}
		return closeExecutionLease(ctx, tx, lease.ID, "canceled")
	case "running":
		if now.Sub(lease.HeartbeatAt) <= input.HeartbeatTimeout {
			return nil
		}
		if _, err := tx.NewUpdate().Model(admission).
			Set("status = 'unknown'").Set("updated_at = CURRENT_TIMESTAMP").WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model(lease).
			Set("capacity_hold_until = ?", now.Add(input.UnknownCapacityHold)).
			Set("updated_at = CURRENT_TIMESTAMP").WherePK().Exec(ctx)
		return err
	case "completed", "failed", "canceled", "interrupted":
		return closeExecutionLease(ctx, tx, lease.ID, admission.Status)
	default:
		return nil
	}
}

func closeExecutionLease(ctx context.Context, tx bun.Tx, leaseID int64, reason string) error {
	_, err := tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
		Set("ended_at = CURRENT_TIMESTAMP").Set("end_reason = ?", reason).Set("updated_at = CURRENT_TIMESTAMP").
		Where("id = ? AND ended_at IS NULL", leaseID).Exec(ctx)
	return err
}

func checkSessionExecutionLease(ctx context.Context, tx bun.Tx, allocationID, sessionID int64) error {
	lease := new(AgentExecutionLease)
	err := tx.NewSelect().Model(lease).
		Where("allocation_id = ? AND session_id = ? AND ended_at IS NULL", allocationID, sessionID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	admission := new(AgentExecutionAdmission)
	if err := tx.NewSelect().Model(admission).Where("id = ?", lease.AdmissionID).Scan(ctx); err != nil {
		return err
	}
	if admission.Status == "unknown" {
		return ErrAgentExecutionRecoveryRequired
	}
	return errorx.AgentExecutionSessionBusy(nil, nil)
}

// checkAllocationExecutionCapacity counts open leases directly so expired
// holds are excluded even when reconciliation has not touched them yet.
func checkAllocationExecutionCapacity(ctx context.Context, tx bun.Tx, allocationID int64, input AcquireAgentExecutionAdmissionInput, now time.Time) error {
	count, err := tx.NewSelect().Model((*AgentExecutionLease)(nil)).
		Join("JOIN agent_execution_admissions AS admission ON admission.id = agent_execution_lease.admission_id").
		Where("agent_execution_lease.allocation_id = ? AND agent_execution_lease.ended_at IS NULL", allocationID).
		Where(
			"(admission.status = 'reserved' AND agent_execution_lease.created_at >= ?) OR "+
				"(admission.status = 'running' AND agent_execution_lease.heartbeat_at >= ?) OR "+
				"(admission.status = 'unknown' AND agent_execution_lease.capacity_hold_until IS NOT NULL AND agent_execution_lease.capacity_hold_until > ?)",
			now.Add(-input.ReservedTimeout), now.Add(-input.HeartbeatTimeout), now,
		).
		Count(ctx)
	if err != nil {
		return err
	}
	if int64(count) >= input.MaxConcurrentPerAllocation {
		return errorx.AgentExecutionCapacityExceeded(nil, errorx.Ctx().
			Set("capacity", input.MaxConcurrentPerAllocation))
	}
	return nil
}

func loadExecutionAdmissionContext(ctx context.Context, tx bun.Tx, input AcquireAgentExecutionAdmissionInput) (*AgentAllocation, int64, error) {
	allocation := new(AgentAllocation)
	if err := tx.NewSelect().Model(allocation).
		Where("id = ?", input.AllocationID).For("UPDATE").Scan(ctx); err != nil {
		return nil, 0, err
	}
	if allocation.ActorKind != normalizedActorKind(input.ActorKind) || allocation.ActorID != input.ActorID {
		return nil, 0, errorx.ErrForbidden
	}
	if allocation.RuntimePrincipal == nil || *allocation.RuntimePrincipal == "" {
		return nil, 0, errorx.ErrForbidden
	}
	if input.ExpectedRuntimePrincipal != "" && *allocation.RuntimePrincipal != input.ExpectedRuntimePrincipal {
		return nil, 0, errorx.ErrForbidden
	}
	if allocation.BindingEndedAt != nil {
		return nil, 0, errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation binding has ended"), nil)
	}
	if allocation.DesiredState != "running" || allocation.ObservedState != "ready" || allocation.AcceptedGeneration == nil {
		return nil, 0, errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation is not ready for execution"), nil)
	}
	if allocation.RuntimeGeneration != *allocation.AcceptedGeneration {
		return nil, 0, errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation generation is not accepted"), nil)
	}
	if allocation.RuntimeGeneration != input.PinnedRuntimeGeneration {
		return nil, 0, errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation generation changed"), nil)
	}
	sessionID, err := ensureExecutionSession(ctx, tx, input, allocation)
	return allocation, sessionID, err
}

func (s *agentOrchestratorStoreImpl) MarkExecutionAdmissionRunning(ctx context.Context, admissionID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
			Set("status = 'running'").Set("heartbeat_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			Where("id = ? AND status = 'reserved'", admissionID).Exec(ctx)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}
		res, err = tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
			Set("heartbeat_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			Where("admission_id = ? AND ended_at IS NULL", admissionID).Exec(ctx)
		return assertAffectedOneRow(res, err)
	})
}

func (s *agentOrchestratorStoreImpl) CompleteExecutionAdmission(ctx context.Context, admissionID int64, status string, unknownCapacityHold time.Duration) error {
	if status != "completed" && status != "failed" && status != "canceled" && status != "unknown" {
		return errorx.BadRequest(fmt.Errorf("invalid execution admission terminal status"), nil)
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
			Set("status = ?", status).Set("heartbeat_at = CURRENT_TIMESTAMP").
			Set("completed_at = CASE WHEN ? = 'unknown' THEN completed_at ELSE CURRENT_TIMESTAMP END", status).
			Set("updated_at = CURRENT_TIMESTAMP").
			Where("id = ? AND status IN ('reserved', 'running')", admissionID).Exec(ctx)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 && (status == "completed" || status == "failed") {
			// Late completion of an admission that lazy reconciliation already
			// moved to unknown: resolve it only while its lease is still open
			// and the allocation still accepts the admission's generation.
			res, err = tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
				Set("status = ?", status).Set("heartbeat_at = CURRENT_TIMESTAMP").
				Set("completed_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
				Where("id = ? AND status = 'unknown'", admissionID).
				Where("EXISTS (SELECT 1 FROM agent_execution_leases l WHERE l.admission_id = agent_execution_admission.id AND l.ended_at IS NULL)").
				Where("EXISTS (SELECT 1 FROM agent_allocations al WHERE al.id = agent_execution_admission.allocation_id AND al.accepted_generation IS NOT NULL AND al.runtime_generation = al.accepted_generation AND al.runtime_generation = agent_execution_admission.runtime_generation)").
				Exec(ctx)
			if err != nil {
				return err
			}
			affected, err = res.RowsAffected()
			if err != nil {
				return err
			}
		}
		if affected != 1 {
			return fmt.Errorf("execution admission %d is not active", admissionID)
		}
		if status == "unknown" {
			// The lease keeps fencing its session; the bounded hold limits how
			// long the execution counts against allocation capacity.
			res, err = tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
				Set("capacity_hold_until = ?", time.Now().Add(unknownCapacityHold)).Set("updated_at = CURRENT_TIMESTAMP").
				Where("admission_id = ? AND ended_at IS NULL", admissionID).Exec(ctx)
		} else {
			res, err = tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
				Set("ended_at = CURRENT_TIMESTAMP").Set("end_reason = ?", status).Set("updated_at = CURRENT_TIMESTAMP").
				Where("admission_id = ? AND ended_at IS NULL", admissionID).Exec(ctx)
		}
		return assertAffectedOneRow(res, err)
	})
}

func (s *agentOrchestratorStoreImpl) HeartbeatExecutionAdmission(ctx context.Context, admissionID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Lock the admission row first (admission → lease order) so a heartbeat
		// cannot interleave with a stale-running reconciliation of the same
		// admission: whichever side commits first wins, and the other observes
		// the outcome instead of overwriting it.
		admission := new(AgentExecutionAdmission)
		if err := tx.NewSelect().Model(admission).Where("id = ?", admissionID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if admission.Status != "running" {
			return ErrAgentExecutionLeaseLost
		}
		res, err := tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
			Set("heartbeat_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			Where("admission_id = ? AND ended_at IS NULL", admissionID).Exec(ctx)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrAgentExecutionLeaseLost
		}
		return nil
	})
}

func (s *agentOrchestratorStoreImpl) ClaimPendingOperations(ctx context.Context, input ClaimAgentAllocationOperationsInput) ([]AgentAllocationOperation, error) {
	if input.Owner == "" || input.Limit <= 0 || input.Lease <= 0 || input.OperationID < 0 {
		return nil, errorx.BadRequest(fmt.Errorf("invalid operation claim"), nil)
	}
	var operations []AgentAllocationOperation
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		query := tx.NewSelect().Model(&operations).
			Where("(agent_allocation_operation.status = 'pending' OR (agent_allocation_operation.status IN ('dispatching', 'dispatched') AND agent_allocation_operation.dispatch_lease_expires_at < CURRENT_TIMESTAMP))").
			Where("NOT EXISTS (SELECT 1 FROM agent_allocation_operations AS previous WHERE previous.allocation_id = agent_allocation_operation.allocation_id AND previous.operation_sequence < agent_allocation_operation.operation_sequence AND previous.status IN ('pending', 'dispatching', 'dispatched', 'running'))")
		if input.OperationID > 0 {
			query = query.Where("id = ?", input.OperationID)
		}
		if err := query.OrderExpr("created_at ASC").For("UPDATE SKIP LOCKED").Limit(input.Limit).Scan(ctx); err != nil {
			return err
		}
		if len(operations) == 0 {
			return nil
		}
		ids := make([]int64, len(operations))
		expiresAt := time.Now().Add(input.Lease)
		for i := range operations {
			ids[i] = operations[i].ID
			operations[i].Status = "dispatching"
			operations[i].DispatchLeaseOwner = &input.Owner
			operations[i].DispatchLeaseExpiresAt = &expiresAt
			operations[i].DispatchAttempts++
		}
		_, err := tx.NewUpdate().Model((*AgentAllocationOperation)(nil)).
			Set("status = 'dispatching'").Set("dispatch_lease_owner = ?", input.Owner).
			Set("dispatch_lease_expires_at = ?", expiresAt).Set("dispatch_attempts = dispatch_attempts + 1").
			Where("id IN (?)", bun.In(ids)).Exec(ctx)
		return err
	})
	return operations, err
}

func (s *agentOrchestratorStoreImpl) MarkOperationDispatched(ctx context.Context, operationID int64, owner, workflowID string) error {
	res, err := s.db.Core.NewUpdate().Model((*AgentAllocationOperation)(nil)).
		Set("status = 'dispatched'").Set("workflow_id = ?", workflowID).Set("dispatched_at = CURRENT_TIMESTAMP").
		Set("dispatch_lease_owner = NULL").
		Where("id = ? AND status = 'dispatching' AND dispatch_lease_owner = ?", operationID, owner).Exec(ctx)
	return assertAffectedOneRow(res, err)
}

func (s *agentOrchestratorStoreImpl) FindOperationContext(ctx context.Context, operationID int64) (*AgentAllocationOperationContext, error) {
	result := new(AgentAllocationOperationContext)
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewSelect().Model(&result.Operation).Where("id = ?", operationID).Scan(ctx); err != nil {
			return err
		}
		if err := tx.NewSelect().Model(&result.Allocation).Where("id = ?", result.Operation.AllocationID).Scan(ctx); err != nil {
			return err
		}
		return tx.NewSelect().Model(&result.Logical).Where("id = ?", result.Allocation.LogicalInstanceID).Scan(ctx)
	})
	return result, err
}

func (s *agentOrchestratorStoreImpl) MarkProvisionOperationRunning(ctx context.Context, operationID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockProvisionOperation(ctx, tx, operationID)
		if err != nil {
			return err
		}
		if operation.Status == "running" || operation.Status == "completed" {
			return nil
		}
		if operation.Status != "dispatched" {
			return fmt.Errorf("provision operation %d cannot run from status %s", operationID, operation.Status)
		}
		if allocation.DesiredState != "running" || operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("provision operation %d is stale", operationID)
		}
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'running'").
			Set("dispatch_lease_owner = NULL").Set("dispatch_lease_expires_at = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).Set("observed_state = 'provisioning'").Set("last_error = NULL").WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) CompleteProvisionOperation(ctx context.Context, operationID, deployID int64) error {
	if deployID <= 0 {
		return errorx.BadRequest(fmt.Errorf("deploy id is required"), nil)
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockProvisionOperation(ctx, tx, operationID)
		if err != nil {
			return err
		}
		if operation.Status == "completed" {
			if allocation.DeployID != nil && *allocation.DeployID == deployID {
				return nil
			}
			return fmt.Errorf("completed provision operation %d has another deploy", operationID)
		}
		if operation.Status != "running" {
			return fmt.Errorf("provision operation %d cannot complete from status %s", operationID, operation.Status)
		}
		if allocation.DesiredState != "running" || operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("provision operation %d is stale", operationID)
		}
		if err := consumeAgentRuntimeReservation(ctx, tx, allocation.ID); err != nil {
			return err
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'completed'").Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(allocation).
			Set("observed_state = 'ready'").Set("deploy_id = ?", deployID).
			Set("accepted_generation = ?", operation.RuntimeGeneration).Set("last_error = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (s *agentOrchestratorStoreImpl) FailProvisionOperation(ctx context.Context, operationID int64, message string, releaseReservation bool) error {
	if message == "" {
		message = "provisioning failed"
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockProvisionOperation(ctx, tx, operationID)
		if err != nil {
			return err
		}
		terminal, err := validateAllocationOperationFailure(operation, allocation)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'failed'").Set("last_error = ?", message).Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		query := tx.NewUpdate().Model(allocation)
		if allocation.DesiredState == "released" {
			_, err = query.Set("last_error = ?", message).WherePK().Exec(ctx)
			return err
		}
		query = query.Set("observed_state = 'failed'").Set("last_error = ?", message)
		if releaseReservation {
			if err := releaseAgentRuntimeReservation(ctx, tx, allocation.ID); err != nil {
				return err
			}
			query = query.Set("binding_ended_at = ?", now).Set("resource_released_at = ?", now)
		}
		_, err = query.WherePK().Exec(ctx)
		return err
	})
}

// BeginAllocationRecovery starts (or reuses) the idempotent restart operation
// that replaces an unknown-fenced runtime. It advances the runtime generation
// and interrupts all open admissions and leases from the previous generation.
// Sibling sessions are interrupted as well; that is acceptable because an
// allocation belongs to a single actor.
func (s *agentOrchestratorStoreImpl) BeginAllocationRecovery(ctx context.Context, allocationID int64, actor commontypes.AgentActor) (*AgentAllocationOperation, error) {
	if allocationID <= 0 || actor.ID == "" {
		return nil, errorx.BadRequest(fmt.Errorf("invalid allocation recovery request"), nil)
	}
	actorKind := normalizedActorKind(string(actor.Kind))
	operation := new(AgentAllocationOperation)
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		allocation := new(AgentAllocation)
		if err := tx.NewSelect().Model(allocation).Where("id = ?", allocationID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if allocation.ActorKind != actorKind || allocation.ActorID != actor.ID {
			return errorx.ErrForbidden
		}
		if allocation.BindingEndedAt != nil || allocation.DesiredState != "running" {
			return errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation cannot recover in its current lifecycle state"), nil)
		}
		existing := new(AgentAllocationOperation)
		err := tx.NewSelect().Model(existing).
			Where("allocation_id = ? AND operation_type = 'restart' AND status IN ('pending', 'dispatching', 'dispatched', 'running')", allocation.ID).
			Scan(ctx)
		if err == nil {
			*operation = *existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		runtimeGeneration := allocation.RuntimeGeneration + 1
		operationSequence := allocation.OperationSequence + 1
		if _, err := tx.NewUpdate().Model((*AgentAllocationOperation)(nil)).
			Set("status = 'canceled'").Set("completed_at = CURRENT_TIMESTAMP").
			Where("allocation_id = ? AND status IN ('pending', 'dispatching', 'dispatched')", allocation.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(allocation).
			Set("observed_state = 'pending'").Set("operation_sequence = ?", operationSequence).
			Set("runtime_generation = ?", runtimeGeneration).Set("last_error = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
			Set("status = 'interrupted'").Set("completed_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			Where("allocation_id = ? AND status IN ('reserved', 'running', 'unknown')", allocation.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
			Set("ended_at = CURRENT_TIMESTAMP").Set("end_reason = 'interrupted'").Set("updated_at = CURRENT_TIMESTAMP").
			Where("allocation_id = ? AND ended_at IS NULL", allocation.ID).Exec(ctx); err != nil {
			return err
		}
		*operation = AgentAllocationOperation{
			AllocationID: allocation.ID, OperationSequence: operationSequence,
			OperationKey:  agentAllocationOperationKey(allocation.ResourceKey, operationSequence),
			OperationType: "restart", Status: "pending", RuntimeGeneration: runtimeGeneration,
			Payload: map[string]any{},
		}
		_, err = tx.NewInsert().Model(operation).Exec(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return operation, nil
}

func (s *agentOrchestratorStoreImpl) MarkRestartOperationRunning(ctx context.Context, operationID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockAllocationOperation(ctx, tx, operationID, "restart")
		if err != nil {
			return err
		}
		if operation.Status == "running" || operation.Status == "completed" {
			return nil
		}
		if operation.Status != "dispatched" {
			return fmt.Errorf("restart operation %d cannot run from status %s", operationID, operation.Status)
		}
		if allocation.DesiredState != "running" || operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("restart operation %d is stale", operationID)
		}
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'running'").
			Set("dispatch_lease_owner = NULL").Set("dispatch_lease_expires_at = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).Set("observed_state = 'provisioning'").Set("last_error = NULL").WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) CompleteRestartOperation(ctx context.Context, operationID, deployID int64) error {
	if deployID <= 0 {
		return errorx.BadRequest(fmt.Errorf("deploy id is required"), nil)
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockAllocationOperation(ctx, tx, operationID, "restart")
		if err != nil {
			return err
		}
		if operation.Status == "completed" {
			if allocation.DeployID != nil && *allocation.DeployID == deployID {
				return nil
			}
			return fmt.Errorf("completed restart operation %d has another deploy", operationID)
		}
		if operation.Status != "running" {
			return fmt.Errorf("restart operation %d cannot complete from status %s", operationID, operation.Status)
		}
		if allocation.DesiredState != "running" || operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("restart operation %d is stale", operationID)
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'completed'").Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).
			Set("observed_state = 'ready'").Set("deploy_id = ?", deployID).
			Set("accepted_generation = ?", operation.RuntimeGeneration).Set("last_error = NULL").WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) FailRestartOperation(ctx context.Context, operationID int64, message string) error {
	if message == "" {
		message = "runtime recovery failed"
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockAllocationOperation(ctx, tx, operationID, "restart")
		if err != nil {
			return err
		}
		terminal, err := validateAllocationOperationFailure(operation, allocation)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'failed'").Set("last_error = ?", message).Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		if allocation.DesiredState == "released" {
			_, err = tx.NewUpdate().Model(allocation).Set("last_error = ?", message).WherePK().Exec(ctx)
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).Set("observed_state = 'failed'").Set("last_error = ?", message).WherePK().Exec(ctx)
		return err
	})
}

// RecordAllocationSuspended applies a confirmed sandbox suspension: the
// allocation keeps its identity and generation, while open executions are
// terminated (reserved → canceled, running/unknown → interrupted) and their
// leases closed. Non-orchestrated deploys have no allocation and are ignored.
func (s *agentOrchestratorStoreImpl) RecordAllocationSuspended(ctx context.Context, deployID int64) error {
	if deployID <= 0 {
		return nil
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		allocation := new(AgentAllocation)
		err := tx.NewSelect().Model(allocation).
			Where("deploy_id = ? AND binding_ended_at IS NULL", deployID).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		// Only a ready runtime can be suspended; provisioning, recovery, and
		// deletion states own the row and must not be overwritten.
		if allocation.DesiredState != "running" || allocation.ObservedState != "ready" {
			return nil
		}
		// Admissions first, then leases: every writer follows this lock order.
		if _, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
			Set("status = 'canceled'").Set("completed_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			Where("allocation_id = ? AND status = 'reserved'", allocation.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
			Set("status = 'interrupted'").Set("completed_at = CURRENT_TIMESTAMP").Set("updated_at = CURRENT_TIMESTAMP").
			Where("allocation_id = ? AND status IN ('running', 'unknown')", allocation.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
			Set("ended_at = CURRENT_TIMESTAMP").
			Set("end_reason = (SELECT a.status FROM agent_execution_admissions a WHERE a.id = agent_execution_lease.admission_id)").
			Set("updated_at = CURRENT_TIMESTAMP").
			Where("agent_execution_lease.allocation_id = ? AND agent_execution_lease.ended_at IS NULL", allocation.ID).
			Where("EXISTS (SELECT 1 FROM agent_execution_admissions a WHERE a.id = agent_execution_lease.admission_id AND a.status IN ('completed', 'failed', 'canceled', 'interrupted'))").
			Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).
			Set("observed_state = 'suspended'").Set("updated_at = CURRENT_TIMESTAMP").WherePK().Exec(ctx)
		return err
	})
}

// BeginAllocationResume starts (or reuses) the idempotent resume operation for
// a suspended allocation. The runtime generation is unchanged: suspension is a
// power state, not a reprovision. Returns nil when the allocation is not
// suspended and no resume is in flight.
func (s *agentOrchestratorStoreImpl) BeginAllocationResume(ctx context.Context, allocationID int64, actor commontypes.AgentActor) (*AgentAllocationOperation, error) {
	if allocationID <= 0 || actor.ID == "" {
		return nil, errorx.BadRequest(fmt.Errorf("invalid allocation resume request"), nil)
	}
	actorKind := normalizedActorKind(string(actor.Kind))
	var operation *AgentAllocationOperation
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		allocation := new(AgentAllocation)
		if err := tx.NewSelect().Model(allocation).Where("id = ?", allocationID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if allocation.ActorKind != actorKind || allocation.ActorID != actor.ID {
			return errorx.ErrForbidden
		}
		if allocation.BindingEndedAt != nil || allocation.DesiredState != "running" {
			return errorx.AgentAllocationLifecycleConflict(fmt.Errorf("allocation cannot resume in its current lifecycle state"), nil)
		}
		existing := new(AgentAllocationOperation)
		err := tx.NewSelect().Model(existing).
			Where("allocation_id = ? AND operation_type = 'resume' AND status IN ('pending', 'dispatching', 'dispatched', 'running')", allocation.ID).
			Scan(ctx)
		if err == nil {
			operation = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if allocation.ObservedState != "suspended" {
			return nil
		}
		operationSequence := allocation.OperationSequence + 1
		operation = &AgentAllocationOperation{
			AllocationID: allocation.ID, OperationSequence: operationSequence,
			OperationKey:  agentAllocationOperationKey(allocation.ResourceKey, operationSequence),
			OperationType: "resume", Status: "pending",
			RuntimeGeneration: allocation.RuntimeGeneration, Payload: map[string]any{},
		}
		if _, err := tx.NewUpdate().Model(allocation).
			Set("observed_state = 'resuming'").Set("operation_sequence = ?", operationSequence).
			Set("last_error = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewInsert().Model(operation).Exec(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return operation, nil
}

func (s *agentOrchestratorStoreImpl) MarkResumeOperationRunning(ctx context.Context, operationID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockAllocationOperation(ctx, tx, operationID, "resume")
		if err != nil {
			return err
		}
		if operation.Status == "running" || operation.Status == "completed" {
			return nil
		}
		if operation.Status != "dispatched" {
			return fmt.Errorf("resume operation %d cannot run from status %s", operationID, operation.Status)
		}
		if allocation.DesiredState != "running" || operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("resume operation %d is stale", operationID)
		}
		_, err = tx.NewUpdate().Model(operation).Set("status = 'running'").
			Set("dispatch_lease_owner = NULL").Set("dispatch_lease_expires_at = NULL").WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) CompleteResumeOperation(ctx context.Context, operationID, deployID int64) error {
	if deployID <= 0 {
		return errorx.BadRequest(fmt.Errorf("deploy id is required"), nil)
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockAllocationOperation(ctx, tx, operationID, "resume")
		if err != nil {
			return err
		}
		if operation.Status == "completed" {
			return nil
		}
		if operation.Status != "running" {
			return fmt.Errorf("resume operation %d cannot complete from status %s", operationID, operation.Status)
		}
		if allocation.DesiredState != "running" || operation.RuntimeGeneration != allocation.RuntimeGeneration ||
			allocation.DeployID == nil || *allocation.DeployID != deployID {
			return fmt.Errorf("resume operation %d is stale", operationID)
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'completed'").Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).
			Set("observed_state = 'ready'").Set("last_error = NULL").WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) FailResumeOperation(ctx context.Context, operationID int64, message string) error {
	if message == "" {
		message = "runtime resume failed"
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockAllocationOperation(ctx, tx, operationID, "resume")
		if err != nil {
			return err
		}
		terminal, err := validateAllocationOperationFailure(operation, allocation)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'failed'").Set("last_error = ?", message).Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		if allocation.DesiredState == "released" {
			_, err = tx.NewUpdate().Model(allocation).Set("last_error = ?", message).WherePK().Exec(ctx)
			return err
		}
		// Back to suspended so the next chat retries the resume.
		_, err = tx.NewUpdate().Model(allocation).Set("observed_state = 'suspended'").Set("last_error = ?", message).WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) MarkDeleteOperationRunning(ctx context.Context, operationID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockDeleteOperation(ctx, tx, operationID)
		if err != nil {
			return err
		}
		if operation.Status == "running" || operation.Status == "completed" {
			return nil
		}
		if operation.Status != "dispatched" || allocation.DesiredState != "released" ||
			operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("delete operation %d is not current", operationID)
		}
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'running'").
			Set("dispatch_lease_owner = NULL").Set("dispatch_lease_expires_at = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).Set("observed_state = 'deleting'").Set("last_error = NULL").WherePK().Exec(ctx)
		return err
	})
}

func (s *agentOrchestratorStoreImpl) CompleteDeleteOperation(ctx context.Context, operationID, deployID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockDeleteOperation(ctx, tx, operationID)
		if err != nil {
			return err
		}
		if operation.Status == "completed" {
			return nil
		}
		if operation.Status != "running" || allocation.DesiredState != "released" ||
			operation.RuntimeGeneration != allocation.RuntimeGeneration {
			return fmt.Errorf("delete operation %d is not current", operationID)
		}
		if err := cleanupAllocationSessions(ctx, tx, allocation.ID); err != nil {
			return err
		}
		if err := releaseAgentRuntimeReservation(ctx, tx, allocation.ID); err != nil {
			return err
		}
		now := time.Now()
		if deployID == 0 && allocation.DeployID != nil {
			deployID = *allocation.DeployID
		}
		if deployID > 0 {
			if _, err := tx.NewUpdate().Model((*Deploy)(nil)).Set("env = ''").Set("secret = ''").
				Set("updated_at = ?", now).Where("id = ?", deployID).Exec(ctx); err != nil {
				return err
			}
		}
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'completed'").Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(allocation).
			Set("observed_state = 'released'").Set("binding_ended_at = ?", now).
			Set("resource_released_at = ?", now).Set("last_error = NULL").WherePK().Exec(ctx); err != nil {
			return err
		}
		return finalizeLogicalDeletion(ctx, tx, allocation.LogicalInstanceID, now)
	})
}

func (s *agentOrchestratorStoreImpl) FailDeleteOperation(ctx context.Context, operationID int64, message string) error {
	if message == "" {
		message = "runtime deletion failed"
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operation, allocation, err := lockDeleteOperation(ctx, tx, operationID)
		if err != nil {
			return err
		}
		terminal, err := validateAllocationOperationFailure(operation, allocation)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model(operation).Set("status = 'failed'").Set("last_error = ?", message).
			Set("completed_at = ?", now).WherePK().Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model(allocation).Set("last_error = ?", message).WherePK().Exec(ctx)
		return err
	})
}

func lockDeleteOperation(ctx context.Context, tx bun.Tx, operationID int64) (*AgentAllocationOperation, *AgentAllocation, error) {
	return lockAllocationOperation(ctx, tx, operationID, "delete")
}

func validateAllocationOperationFailure(operation *AgentAllocationOperation, allocation *AgentAllocation) (bool, error) {
	switch operation.Status {
	case "completed", "failed", "canceled":
		return true, nil
	}
	if operation.RuntimeGeneration != allocation.RuntimeGeneration {
		return false, fmt.Errorf("%s operation %d is stale", operation.OperationType, operation.ID)
	}
	return false, nil
}

func lockAllocationOperation(ctx context.Context, tx bun.Tx, operationID int64, operationType string) (*AgentAllocationOperation, *AgentAllocation, error) {
	operation := new(AgentAllocationOperation)
	if err := tx.NewSelect().Model(operation).Where("id = ?", operationID).For("UPDATE").Scan(ctx); err != nil {
		return nil, nil, err
	}
	if operation.OperationType != operationType {
		return nil, nil, fmt.Errorf("operation %d is not %s", operationID, operationType)
	}
	allocation := new(AgentAllocation)
	if err := tx.NewSelect().Model(allocation).Where("id = ?", operation.AllocationID).For("UPDATE").Scan(ctx); err != nil {
		return nil, nil, err
	}
	return operation, allocation, nil
}

func cleanupAllocationSessions(ctx context.Context, tx bun.Tx, allocationID int64) error {
	var sessions []AgentAllocationSession
	if err := tx.NewSelect().Model(&sessions).Where("allocation_id = ?", allocationID).Scan(ctx); err != nil {
		return err
	}
	if len(sessions) == 0 {
		return nil
	}
	ids := make([]int64, len(sessions))
	for i := range sessions {
		ids[i] = sessions[i].ID
	}
	// Preserve execution audit rows while releasing their nullable session FK so
	// allocation cleanup can remove the orchestrator session registry rows.
	if _, err := tx.NewUpdate().Model((*AgentExecutionAdmission)(nil)).
		Set("session_id = NULL").Where("session_id IN (?)", bun.In(ids)).Exec(ctx); err != nil {
		return err
	}
	if _, err := tx.NewUpdate().Model((*AgentExecutionLease)(nil)).
		Set("session_id = NULL").Where("session_id IN (?)", bun.In(ids)).Exec(ctx); err != nil {
		return err
	}
	_, err := tx.NewDelete().Model((*AgentAllocationSession)(nil)).Where("id IN (?)", bun.In(ids)).Exec(ctx)
	return err
}

func finalizeLogicalDeletion(ctx context.Context, tx bun.Tx, logicalInstanceID int64, now time.Time) error {
	exists, err := tx.NewSelect().Model((*AgentAllocation)(nil)).
		Where("logical_instance_id = ? AND binding_ended_at IS NULL", logicalInstanceID).Exists(ctx)
	if err != nil || exists {
		return err
	}
	_, err = tx.NewUpdate().Model((*AgentInstance)(nil)).Set("deleted_at = ?", now).
		Where("id = ? AND kind = ? AND deletion_started_at IS NOT NULL AND deleted_at IS NULL", logicalInstanceID, AgentInstanceKindLogical).Exec(ctx)
	return err
}

func lockProvisionOperation(ctx context.Context, tx bun.Tx, operationID int64) (*AgentAllocationOperation, *AgentAllocation, error) {
	return lockAllocationOperation(ctx, tx, operationID, "provision")
}

func consumeAgentRuntimeReservation(ctx context.Context, tx bun.Tx, allocationID int64) error {
	reservation, counter, err := lockAgentRuntimeReservation(ctx, tx, allocationID)
	if err != nil {
		return err
	}
	if reservation.State == "consumed" {
		return nil
	}
	if reservation.State != "reserved" {
		return fmt.Errorf("reservation %d cannot be consumed from state %s", reservation.ID, reservation.State)
	}
	if _, err := tx.NewUpdate().Model(reservation).Set("state = 'consumed'").WherePK().Exec(ctx); err != nil {
		return err
	}
	_, err = tx.NewUpdate().Model(counter).Set("reserved = reserved - ?", reservation.Amount).
		Set("consumed = consumed + ?", reservation.Amount).WherePK().Exec(ctx)
	return err
}

func releaseAgentRuntimeReservation(ctx context.Context, tx bun.Tx, allocationID int64) error {
	reservation, counter, err := lockAgentRuntimeReservation(ctx, tx, allocationID)
	if err != nil {
		return err
	}
	if reservation.State == "released" {
		return nil
	}
	if reservation.State != "reserved" && reservation.State != "consumed" {
		return fmt.Errorf("reservation %d cannot be released from state %s", reservation.ID, reservation.State)
	}
	if _, err := tx.NewUpdate().Model(reservation).Set("state = 'released'").Set("released_at = CURRENT_TIMESTAMP").WherePK().Exec(ctx); err != nil {
		return err
	}
	query := tx.NewUpdate().Model(counter)
	switch reservation.State {
	case "reserved":
		query = query.Set("reserved = reserved - ?", reservation.Amount)
	case "consumed":
		query = query.Set("consumed = consumed - ?", reservation.Amount)
	}
	_, err = query.WherePK().Exec(ctx)
	return err
}

func lockAgentRuntimeReservation(ctx context.Context, tx bun.Tx, allocationID int64) (*AgentCapacityReservation, *AgentCapacityCounter, error) {
	reservation := new(AgentCapacityReservation)
	if err := tx.NewSelect().Model(reservation).Where("allocation_id = ? AND resource_kind = 'agent_runtime_instance'", allocationID).For("UPDATE").Scan(ctx); err != nil {
		return nil, nil, err
	}
	counter := new(AgentCapacityCounter)
	if err := tx.NewSelect().Model(counter).
		Where("scope_kind = ? AND scope_id = ? AND resource_kind = ?", reservation.ScopeKind, reservation.ScopeID, reservation.ResourceKind).
		For("UPDATE").Scan(ctx); err != nil {
		return nil, nil, err
	}
	return reservation, counter, nil
}
