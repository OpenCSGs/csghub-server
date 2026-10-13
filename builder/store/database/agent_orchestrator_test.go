package database_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type blockAfterAdmissionLockHook struct {
	reached       chan struct{}
	secondReached chan struct{}
	release       chan struct{}
	once          sync.Once
	mu            sync.Mutex
	matches       int
}

func (h *blockAfterAdmissionLockHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if !isAdmissionLockQuery(event.Query) {
		return ctx
	}
	h.mu.Lock()
	h.matches++
	if h.matches == 2 {
		close(h.secondReached)
	}
	h.mu.Unlock()
	return ctx
}

func (h *blockAfterAdmissionLockHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	if !isAdmissionLockQuery(event.Query) {
		return
	}
	h.once.Do(func() {
		close(h.reached)
		<-h.release
	})
}

func isAdmissionLockQuery(query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(query, `from "agent_execution_admissions"`) && strings.Contains(query, "for update")
}

func TestAgentOrchestratorStoreReserveAllocationIsIdempotent(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	orchestratorStore := database.NewAgentOrchestratorStoreWithDB(db)

	owner := uuid.NewString()
	logical := newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString())
	logical.Metadata = map[string]any{
		"template_name": "owner/agent", "template_metadata": map[string]any{"agent_file": map[string]any{"name": "agent"}},
		"publisher_private": "must-not-copy",
	}
	logical = createPublicLogicalAgent(t, ctx, db, owner, logical.ContentID)
	logical.Metadata = map[string]any{
		"template_name": "owner/agent", "template_metadata": map[string]any{"agent_file": map[string]any{"name": "agent"}},
		"publisher_private": "must-not-copy",
	}
	_, err := db.Core.NewUpdate().Model(logical).Column("metadata").WherePK().Exec(ctx)
	require.NoError(t, err)

	actorID := uuid.NewString()
	input := database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID,
		InstanceQuota: 1, SessionUUID: uuid.NewString(),
	}
	results := make([]*database.ReserveAgentAllocationResult, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = orchestratorStore.ReserveAllocation(ctx, input)
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, results[0].Allocation.ID, results[1].Allocation.ID)
	require.Equal(t, results[0].Allocation.ResourceKey, results[1].Allocation.ResourceKey)
	require.NotEqual(t, results[0].Created, results[1].Created)
	require.True(t, strings.HasPrefix(results[0].Allocation.ResourceKey, types.AgentOrchestratedRuntimeResourcePrefix+"logical-"))
	require.LessOrEqual(t, len(results[0].Allocation.ResourceKey), 63)
	created := results[0]
	if !created.Created {
		created = results[1]
	}
	require.NotNil(t, created.Operation)
	require.Equal(t, "agent-allocation/"+created.Allocation.ResourceKey+"/1", created.Operation.OperationKey)
	var runtimeCount int
	require.NoError(t, db.Core.NewSelect().Model((*database.AgentInstance)(nil)).
		ColumnExpr("count(*)").Where("user_uuid = ? AND kind = ?", actorID, database.AgentInstanceKindRuntime).Scan(ctx, &runtimeCount))
	require.Zero(t, runtimeCount)

	input.InstanceQuota = 3
	_, err = orchestratorStore.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	counter := new(database.AgentCapacityCounter)
	require.NoError(t, db.Core.NewSelect().Model(counter).
		Where("scope_kind = 'user' AND scope_id = ? AND resource_kind = 'agent_runtime_instance'", actorID).Scan(ctx))
	// Reuse does not lock the capacity counter, so the creation-time limit remains.
	require.Equal(t, int64(1), counter.CapacityLimit)
}

func TestAgentOrchestratorStoreReservePrivateLogicalAgentForOwnerOnly(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	catalogStore := database.NewAgentCatalogStoreWithDB(db)
	orchestratorStore := database.NewAgentOrchestratorStoreWithDB(db)
	owner := uuid.NewString()
	logical, err := catalogStore.CreateLogicalAgent(ctx, newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString()))
	require.NoError(t, err)

	_, err = orchestratorStore.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: uuid.NewString(),
		RuntimePrincipal: "other", BillingOwner: "other", InstanceQuota: 1,
	})
	require.ErrorIs(t, err, errorx.ErrForbidden)

	result, err := orchestratorStore.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: owner,
		RuntimePrincipal: owner, BillingOwner: owner, InstanceQuota: 1,
	})
	require.NoError(t, err)
	require.Equal(t, owner, result.Allocation.ActorID)
}

func TestAgentOrchestratorStorePermanentFailureReleasesBindingAndQuota(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	input := database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID,
		InstanceQuota: 1,
		SessionUUID:   uuid.NewString(), SessionName: "provisional session",
	}
	reserved, err := store.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker-1", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker-1", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.FailProvisionOperation(ctx, claimed[0].ID, "invalid blueprint", true))

	retry, err := store.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	require.True(t, retry.Created)
	require.NotEqual(t, reserved.Allocation.ID, retry.Allocation.ID)
	require.NotEqual(t, reserved.Allocation.ResourceKey, retry.Allocation.ResourceKey)
	session := new(database.AgentAllocationSession)
	require.NoError(t, db.Core.NewSelect().Model(session).
		Where("allocation_id = ? AND session_uuid = ?", retry.Allocation.ID, input.SessionUUID).
		Scan(ctx))
	require.Equal(t, input.SessionName, session.SessionName)
}

func TestAgentOrchestratorStoreRetryableFailureCreatesRepairOperation(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	input := database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID,
		InstanceQuota: 1,
	}
	reserved, err := store.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker-1", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker-1", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.FailProvisionOperation(ctx, claimed[0].ID, "runner unavailable", false))

	retry, err := store.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	require.False(t, retry.Created)
	require.Equal(t, reserved.Allocation.ID, retry.Allocation.ID)
	require.NotNil(t, retry.Operation)
	require.Equal(t, int64(2), retry.Operation.OperationSequence)
	require.Equal(t, "pending", retry.Operation.Status)
	require.Equal(t, "agent-allocation/"+reserved.Allocation.ResourceKey+"/2", retry.Operation.OperationKey)

	var reservationCount int
	require.NoError(t, db.Core.NewSelect().Table("agent_capacity_reservations").
		ColumnExpr("count(*)").Where("allocation_id = ?", reserved.Allocation.ID).Scan(ctx, &reservationCount))
	require.Equal(t, 1, reservationCount)
}

func TestAgentOrchestratorStoreReclaimsUnacknowledgedDispatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	_, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID, BillingOwner: actorID, InstanceQuota: 1})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker-1", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker-1", claimed[0].OperationKey))
	_, err = db.Core.NewUpdate().Model((*database.AgentAllocationOperation)(nil)).
		Set("dispatch_lease_expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second'").Where("id = ?", claimed[0].ID).Exec(ctx)
	require.NoError(t, err)

	reclaimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker-2", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, claimed[0].ID, reclaimed[0].ID)
	require.Equal(t, 2, reclaimed[0].DispatchAttempts)
}

func TestAgentOrchestratorStoreClaimsRequestedOperation(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())

	reserve := func(actorID string) *database.ReserveAgentAllocationResult {
		result, reserveErr := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
			LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
			BillingOwner: actorID, InstanceQuota: 1,
		})
		require.NoError(t, reserveErr)
		return result
	}
	first := reserve(uuid.NewString())
	second := reserve(uuid.NewString())
	require.NotEqual(t, first.Allocation.ResourceKey, second.Allocation.ResourceKey)

	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{
		Owner: "immediate", Limit: 1, Lease: time.Minute, OperationID: second.Operation.ID,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, second.Operation.ID, claimed[0].ID)

	claimed, err = store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{
		Owner: "cron", Limit: 10, Lease: time.Minute,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, first.Operation.ID, claimed[0].ID)

	claimed, err = store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{
		Owner: "duplicate", Limit: 1, Lease: time.Minute, OperationID: second.Operation.ID,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)

	_, err = db.Core.NewUpdate().Model((*database.AgentAllocationOperation)(nil)).
		Set("dispatch_lease_expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second'").
		Where("id = ?", second.Operation.ID).Exec(ctx)
	require.NoError(t, err)
	claimed, err = store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{
		Owner: "repair", Limit: 1, Lease: time.Minute, OperationID: second.Operation.ID,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, second.Operation.ID, claimed[0].ID)
	require.Equal(t, 2, claimed[0].DispatchAttempts)
}

func TestAgentOrchestratorStoreAllowsActorRuntimesForDifferentLogicalAgents(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	owner := uuid.NewString()
	firstLogical := createPublicLogicalAgent(t, ctx, db, owner, "logical-"+uuid.NewString())
	secondLogical := createPublicLogicalAgent(t, ctx, db, owner, "logical-"+uuid.NewString())

	actorID := uuid.NewString()
	input := database.ReserveAgentAllocationInput{
		LogicalInstanceID: firstLogical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID,
		InstanceQuota: 2,
	}
	first, err := store.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	input.LogicalInstanceID = secondLogical.ID
	second, err := store.ReserveAllocation(ctx, input)
	require.NoError(t, err)
	require.NotEqual(t, first.Allocation.ResourceKey, second.Allocation.ResourceKey)
}

func TestAgentOrchestratorStoreExecutionAdmissionIsSessionScoped(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	actor := &database.User{GitID: time.Now().UnixNano(), NickName: "actor", Username: "actor-" + uuid.NewString(), Password: "unused", UUID: actorID}
	_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
	require.NoError(t, err)
	deploy := &database.Deploy{UserID: actor.ID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu"}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
		BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID))

	sessionUUID := uuid.NewString()
	input := executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID)
	first, err := store.AcquireExecutionAdmission(ctx, input)
	require.NoError(t, err)
	secondInput := executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID)
	_, err = store.AcquireExecutionAdmission(ctx, secondInput)
	require.ErrorIs(t, err, errorx.ErrAgentExecutionSessionBusy)

	// A different session on the same allocation is admitted concurrently.
	other, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NotEqual(t, first.ID, other.ID)

	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, first.ID))
	require.NoError(t, store.CompleteExecutionAdmission(ctx, first.ID, "completed", time.Hour))
	second, err := store.AcquireExecutionAdmission(ctx, secondInput)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
}

func TestAgentOrchestratorStoreDurablyDeletesLogicalAllocations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())

	actorID := uuid.NewString()
	actor := &database.User{GitID: time.Now().UnixNano(), NickName: "actor", Username: "actor-" + uuid.NewString(), Password: "unused", UUID: actorID}
	_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
	require.NoError(t, err)
	deploy := &database.Deploy{
		UserID: actor.ID, UserUUID: actorID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu",
		Env: `{"CSGHUB_USER_TOKEN":"secret"}`, Secret: "secret", Type: types.SandboxType,
	}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	sessionUUID := uuid.NewString()
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
		BillingOwner: actorID, InstanceQuota: 1,
		SessionUUID: sessionUUID, SessionName: "chat",
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "provision", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "provision", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID))
	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID))
	require.NoError(t, err)
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "completed", time.Hour))

	deletion, err := store.BeginLogicalAgentDeletion(ctx, logical.ID, logical.UserUUID)
	require.NoError(t, err)
	require.True(t, deletion.Pending)
	require.Len(t, deletion.Operations, 1)
	require.Equal(t, "delete", deletion.Operations[0].OperationType)
	require.Equal(t, "agent-allocation/"+reserved.Allocation.ResourceKey+"/2", deletion.Operations[0].OperationKey)
	require.NotNil(t, deletion.Logical.DeletionStartedAt)
	require.False(t, deletion.Logical.Public)

	claimed, err = store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "delete", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, deletion.Operations[0].ID, claimed[0].ID)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "delete", claimed[0].OperationKey))
	require.NoError(t, store.MarkDeleteOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteDeleteOperation(ctx, claimed[0].ID, deploy.ID))

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "released", allocation.ObservedState)
	require.NotNil(t, allocation.BindingEndedAt)
	reservation := new(database.AgentCapacityReservation)
	require.NoError(t, db.Core.NewSelect().Model(reservation).Where("allocation_id = ?", allocation.ID).Scan(ctx))
	require.Equal(t, "released", reservation.State)
	counter := new(database.AgentCapacityCounter)
	require.NoError(t, db.Core.NewSelect().Model(counter).
		Where("scope_kind = 'user' AND scope_id = ? AND resource_kind = 'agent_runtime_instance'", actorID).Scan(ctx))
	require.Zero(t, counter.Reserved)
	require.Zero(t, counter.Consumed)
	_, err = database.NewAgentInstanceStoreWithDB(db).FindByID(ctx, logical.ID)
	require.Error(t, err)
	_, err = database.NewAgentInstanceSessionStoreWithDB(db).FindByUUID(ctx, sessionUUID)
	require.Error(t, err)
	persistedAdmission := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persistedAdmission).Where("id = ?", admission.ID).Scan(ctx))
	require.Nil(t, persistedAdmission.SessionID)
	require.NoError(t, db.Core.NewSelect().Model(deploy).Where("id = ?", deploy.ID).Scan(ctx))
	require.Empty(t, deploy.Env)
	require.Empty(t, deploy.Secret)
}

func TestAgentOrchestratorStoreRejectsUseWhileDeleting(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)

	deletion, err := store.BeginLogicalAgentDeletion(ctx, logical.ID, logical.UserUUID)
	require.NoError(t, err)
	require.True(t, deletion.Pending)

	sessionUUID := uuid.NewString()
	_, err = store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID, InstanceQuota: 1,
		SessionUUID: sessionUUID, SessionName: "late",
	})
	require.ErrorIs(t, err, errorx.ErrAgentAllocationLifecycleConflict)
	_, err = database.NewAgentInstanceSessionStoreWithDB(db).FindByUUID(ctx, sessionUUID)
	require.Error(t, err)

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "deleting", allocation.ObservedState)
}

func TestAgentOrchestratorStoreFindCallerAllocationPrefersLiveThenNewestEnded(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()

	first, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.FailProvisionOperation(ctx, claimed[0].ID, "permanent", true))

	ended, err := store.FindCallerAllocation(ctx, logical.ID, "user", actorID)
	require.NoError(t, err)
	require.Equal(t, first.Allocation.ID, ended.ID)
	require.Equal(t, "failed", ended.ObservedState)
	require.NotNil(t, ended.BindingEndedAt)

	second, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	require.NotEqual(t, first.Allocation.ID, second.Allocation.ID)

	live, err := store.FindCallerAllocation(ctx, logical.ID, "user", actorID)
	require.NoError(t, err)
	require.Equal(t, second.Allocation.ID, live.ID)
	require.Nil(t, live.BindingEndedAt)
}

func TestAgentOrchestratorStoreAdmissionRejectsPinnedGenerationMismatch(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	actor := &database.User{GitID: time.Now().UnixNano(), NickName: "actor", Username: "actor-" + uuid.NewString(), Password: "unused", UUID: actorID}
	_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
	require.NoError(t, err)
	deploy := &database.Deploy{UserID: actor.ID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu"}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
		BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID))

	mismatch := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
	mismatch.ExpectedRuntimePrincipal = actorID
	mismatch.PinnedRuntimeGeneration = 9
	_, err = store.AcquireExecutionAdmission(ctx, mismatch)
	require.ErrorIs(t, err, errorx.ErrAgentAllocationLifecycleConflict)

	okInput := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
	okInput.ExpectedRuntimePrincipal = actorID
	okInput.PinnedRuntimeGeneration = 0
	ok, err := store.AcquireExecutionAdmission(ctx, okInput)
	require.NoError(t, err)
	require.NotZero(t, ok.ID)
}

func TestAgentOrchestratorStoreAdmissionRejectsPinnedGenerationZeroAgainstLaterGeneration(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	actor := &database.User{GitID: time.Now().UnixNano(), NickName: "actor", Username: "actor-" + uuid.NewString(), Password: "unused", UUID: actorID}
	_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
	require.NoError(t, err)
	deploy := &database.Deploy{UserID: actor.ID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu"}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
		BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID))

	_, err = db.Core.NewUpdate().Model((*database.AgentAllocation)(nil)).
		Set("runtime_generation = ?", 1).
		Set("accepted_generation = ?", 1).
		Where("id = ?", reserved.Allocation.ID).
		Exec(ctx)
	require.NoError(t, err)

	mismatch := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
	mismatch.ExpectedRuntimePrincipal = actorID
	mismatch.PinnedRuntimeGeneration = 0
	_, err = store.AcquireExecutionAdmission(ctx, mismatch)
	require.ErrorIs(t, err, errorx.ErrAgentAllocationLifecycleConflict)

	okInput := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
	okInput.ExpectedRuntimePrincipal = actorID
	okInput.PinnedRuntimeGeneration = 1
	ok, err := store.AcquireExecutionAdmission(ctx, okInput)
	require.NoError(t, err)
	require.NotZero(t, ok.ID)
}

func TestAgentOrchestratorStoreConcurrentReserveAndCompleteDoNotDeadlock(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	actor := &database.User{GitID: time.Now().UnixNano(), NickName: "actor", Username: "actor-" + uuid.NewString(), Password: "unused", UUID: actorID}
	_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
	require.NoError(t, err)
	deploy := &database.Deploy{UserID: actor.ID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu"}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
		BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))

	done := make(chan error, 2)
	go func() {
		done <- store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID)
	}()
	go func() {
		_, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
			LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
			BillingOwner: actorID, InstanceQuota: 1, SessionUUID: uuid.NewString(), SessionName: "reuse",
		})
		done <- err
	}()
	require.NoError(t, <-done)
	require.NoError(t, <-done)

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "ready", allocation.ObservedState)
}

func executionAdmissionInput(allocationID int64, actorID, sessionUUID string) database.AcquireAgentExecutionAdmissionInput {
	return database.AcquireAgentExecutionAdmissionInput{
		AllocationID: allocationID, ActorID: actorID, SessionUUID: sessionUUID,
		SessionName: "chat", ExecutionID: uuid.NewString(),
		MaxConcurrentPerAllocation: 4, ReservedTimeout: 30 * time.Second,
		HeartbeatTimeout: 90 * time.Second, UnknownCapacityHold: time.Hour,
	}
}

func provisionReadyAgentAllocation(t *testing.T, ctx context.Context, db *database.DB, store database.AgentOrchestratorStore, logicalID int64, actorID string) (*database.ReserveAgentAllocationResult, *database.Deploy) {
	t.Helper()
	actor := &database.User{GitID: time.Now().UnixNano(), NickName: "actor", Username: "actor-" + uuid.NewString(), Password: "unused", UUID: actorID}
	_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
	require.NoError(t, err)
	deploy := &database.Deploy{UserID: actor.ID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu"}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	reserved, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logicalID, ActorKind: "user", ActorID: actorID, RuntimePrincipal: actorID,
		BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, "worker", claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID))
	return reserved, deploy
}

func TestAgentOrchestratorStoreAdmissionEnforcesAllocationCapacity(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	input := func() database.AcquireAgentExecutionAdmissionInput {
		in := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
		in.MaxConcurrentPerAllocation = 2
		return in
	}
	first, err := store.AcquireExecutionAdmission(ctx, input())
	require.NoError(t, err)
	_, err = store.AcquireExecutionAdmission(ctx, input())
	require.NoError(t, err)
	_, err = store.AcquireExecutionAdmission(ctx, input())
	require.ErrorIs(t, err, errorx.ErrAgentExecutionCapacityExceeded)

	require.NoError(t, store.CompleteExecutionAdmission(ctx, first.ID, "completed", time.Hour))
	_, err = store.AcquireExecutionAdmission(ctx, input())
	require.NoError(t, err)
}

func TestAgentOrchestratorStoreAdmissionReconcilesStaleLeases(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	// A stale reserved admission is canceled and its lease closed.
	staleReserved, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	_, err = db.Core.NewUpdate().Model((*database.AgentExecutionLease)(nil)).
		Set("created_at = ?", time.Now().Add(-time.Hour)).Where("admission_id = ?", staleReserved.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", staleReserved.ID).Scan(ctx))
	require.Equal(t, "canceled", persisted.Status)
	staleLease := new(database.AgentExecutionLease)
	require.NoError(t, db.Core.NewSelect().Model(staleLease).Where("admission_id = ?", staleReserved.ID).Scan(ctx))
	require.NotNil(t, staleLease.EndedAt)
	require.Equal(t, "canceled", *staleLease.EndReason)

	// A stale running admission becomes unknown, keeps fencing its session,
	// and starts a bounded capacity hold.
	sessionB := uuid.NewString()
	staleRunning, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionB))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, staleRunning.ID))
	_, err = db.Core.NewUpdate().Model((*database.AgentExecutionLease)(nil)).
		Set("heartbeat_at = ?", time.Now().Add(-time.Hour)).Where("admission_id = ?", staleRunning.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", staleRunning.ID).Scan(ctx))
	require.Equal(t, "unknown", persisted.Status)
	runningLease := new(database.AgentExecutionLease)
	require.NoError(t, db.Core.NewSelect().Model(runningLease).Where("admission_id = ?", staleRunning.ID).Scan(ctx))
	require.Nil(t, runningLease.EndedAt)
	require.NotNil(t, runningLease.CapacityHoldUntil)

	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionB))
	require.ErrorIs(t, err, database.ErrAgentExecutionRecoveryRequired)
}

func TestAgentOrchestratorStoreAdmissionCapacityIgnoresExpiredUnknownHold(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	sessionA := uuid.NewString()
	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionA))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	// A negative hold simulates an already-expired capacity hold.
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "unknown", -time.Hour))

	capped := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
	capped.MaxConcurrentPerAllocation = 1
	_, err = store.AcquireExecutionAdmission(ctx, capped)
	require.NoError(t, err)

	// The unknown admission still fences its own session after the hold expires.
	fenced := executionAdmissionInput(reserved.Allocation.ID, actorID, sessionA)
	fenced.MaxConcurrentPerAllocation = 1
	_, err = store.AcquireExecutionAdmission(ctx, fenced)
	require.ErrorIs(t, err, database.ErrAgentExecutionRecoveryRequired)
}

func TestAgentOrchestratorStoreHeartbeatExecutionAdmission(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))

	lease := new(database.AgentExecutionLease)
	require.NoError(t, db.Core.NewSelect().Model(lease).Where("admission_id = ?", admission.ID).Scan(ctx))
	firstBeat := lease.HeartbeatAt
	require.NoError(t, store.HeartbeatExecutionAdmission(ctx, admission.ID))
	require.NoError(t, db.Core.NewSelect().Model(lease).Where("admission_id = ?", admission.ID).Scan(ctx))
	require.True(t, lease.HeartbeatAt.After(firstBeat) || lease.HeartbeatAt.Equal(firstBeat))

	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "completed", time.Hour))
	require.ErrorIs(t, store.HeartbeatExecutionAdmission(ctx, admission.ID), database.ErrAgentExecutionLeaseLost)
}

func TestAgentOrchestratorStoreHeartbeatWinsStaleReconciliation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	_, err = db.Core.NewUpdate().Model((*database.AgentExecutionLease)(nil)).
		Set("heartbeat_at = ?", time.Now().Add(-time.Hour)).
		Where("admission_id = ?", admission.ID).Exec(ctx)
	require.NoError(t, err)

	hook := &blockAfterAdmissionLockHook{
		reached: make(chan struct{}), secondReached: make(chan struct{}), release: make(chan struct{}),
	}
	db.BunDB.AddQueryHook(hook)
	heartbeatDone := make(chan error, 1)
	go func() {
		heartbeatDone <- store.HeartbeatExecutionAdmission(ctx, admission.ID)
	}()
	<-hook.reached

	acquireDone := make(chan error, 1)
	go func() {
		input := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
		input.HeartbeatTimeout = time.Minute
		_, err := store.AcquireExecutionAdmission(ctx, input)
		acquireDone <- err
	}()
	<-hook.secondReached
	close(hook.release)

	require.NoError(t, <-heartbeatDone)
	require.NoError(t, <-acquireDone)
	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", admission.ID).Scan(ctx))
	require.Equal(t, "running", persisted.Status)
}

func TestAgentOrchestratorStoreStaleReconciliationWinsHeartbeat(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	_, err = db.Core.NewUpdate().Model((*database.AgentExecutionLease)(nil)).
		Set("heartbeat_at = ?", time.Now().Add(-time.Hour)).
		Where("admission_id = ?", admission.ID).Exec(ctx)
	require.NoError(t, err)

	hook := &blockAfterAdmissionLockHook{
		reached: make(chan struct{}), secondReached: make(chan struct{}), release: make(chan struct{}),
	}
	db.BunDB.AddQueryHook(hook)
	acquireDone := make(chan error, 1)
	go func() {
		input := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
		input.HeartbeatTimeout = time.Minute
		_, err := store.AcquireExecutionAdmission(ctx, input)
		acquireDone <- err
	}()
	<-hook.reached

	heartbeatDone := make(chan error, 1)
	go func() {
		heartbeatDone <- store.HeartbeatExecutionAdmission(ctx, admission.ID)
	}()
	<-hook.secondReached
	close(hook.release)

	require.NoError(t, <-acquireDone)
	require.ErrorIs(t, <-heartbeatDone, database.ErrAgentExecutionLeaseLost)
	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", admission.ID).Scan(ctx))
	require.Equal(t, "unknown", persisted.Status)
}

func TestAgentOrchestratorStoreBeginAllocationRecoveryInterruptsPreviousGeneration(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, deploy := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	sessionUUID := uuid.NewString()
	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "unknown", time.Hour))
	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID))
	require.ErrorIs(t, err, database.ErrAgentExecutionRecoveryRequired)

	operation, err := store.BeginAllocationRecovery(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	require.Equal(t, "restart", operation.OperationType)
	require.Equal(t, "pending", operation.Status)
	require.Equal(t, int64(1), operation.RuntimeGeneration)

	// Concurrent retries converge on the same operation.
	again, err := store.BeginAllocationRecovery(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	require.Equal(t, operation.ID, again.ID)

	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", admission.ID).Scan(ctx))
	require.Equal(t, "interrupted", persisted.Status)
	require.NotNil(t, persisted.CompletedAt)
	lease := new(database.AgentExecutionLease)
	require.NoError(t, db.Core.NewSelect().Model(lease).Where("admission_id = ?", admission.ID).Scan(ctx))
	require.NotNil(t, lease.EndedAt)
	require.Equal(t, "interrupted", *lease.EndReason)

	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute, OperationID: operation.ID})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, store.MarkOperationDispatched(ctx, operation.ID, "worker", operation.OperationKey))
	require.NoError(t, store.MarkRestartOperationRunning(ctx, operation.ID))
	require.NoError(t, store.CompleteRestartOperation(ctx, operation.ID, deploy.ID))

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "ready", allocation.ObservedState)
	require.Equal(t, int64(1), allocation.RuntimeGeneration)
	require.Equal(t, int64(1), *allocation.AcceptedGeneration)

	recovered := executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID)
	recovered.PinnedRuntimeGeneration = 1
	_, err = store.AcquireExecutionAdmission(ctx, recovered)
	require.NoError(t, err)
}

func TestAgentOrchestratorStoreFailedRestartDoesNotRedispatchCompletedProvision(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)
	actor := types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID}

	restart, err := store.BeginAllocationRecovery(ctx, reserved.Allocation.ID, actor)
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{
		Owner: "worker", Limit: 1, Lease: time.Minute, OperationID: restart.ID,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, store.MarkOperationDispatched(ctx, restart.ID, "worker", restart.OperationKey))
	require.NoError(t, store.MarkRestartOperationRunning(ctx, restart.ID))
	require.NoError(t, store.FailRestartOperation(ctx, restart.ID, "transient runtime error"))

	reused, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID, InstanceQuota: 1,
	})
	require.NoError(t, err)
	require.Equal(t, reserved.Allocation.ID, reused.Allocation.ID)
	require.Equal(t, "failed", reused.Allocation.ObservedState)
	require.Nil(t, reused.Operation)

	repair, err := store.BeginAllocationRecovery(ctx, reused.Allocation.ID, actor)
	require.NoError(t, err)
	require.Equal(t, "restart", repair.OperationType)
	require.Equal(t, "pending", repair.Status)
	require.Equal(t, restart.RuntimeGeneration+1, repair.RuntimeGeneration)
}

func TestAgentOrchestratorStoreConcurrentAdmissionsRespectCapacity(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	var admitted, rejected int64
	var wg sync.WaitGroup
	var mu sync.Mutex
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			input := executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString())
			input.MaxConcurrentPerAllocation = 2
			_, err := store.AcquireExecutionAdmission(ctx, input)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				admitted++
			} else {
				require.ErrorIs(t, err, errorx.ErrAgentExecutionCapacityExceeded)
				rejected++
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int64(2), admitted)
	require.Equal(t, int64(2), rejected)
}

func TestAgentOrchestratorStoreRecordAllocationSuspended(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, deploy := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	reservedAdmission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	runningAdmission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, runningAdmission.ID))
	unknownAdmission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, unknownAdmission.ID))
	require.NoError(t, store.CompleteExecutionAdmission(ctx, unknownAdmission.ID, "unknown", time.Hour))

	require.NoError(t, store.RecordAllocationSuspended(ctx, deploy.ID))

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "suspended", allocation.ObservedState)
	require.Equal(t, int64(0), allocation.RuntimeGeneration)

	assertAdmission := func(id int64, status string) {
		admission := new(database.AgentExecutionAdmission)
		require.NoError(t, db.Core.NewSelect().Model(admission).Where("id = ?", id).Scan(ctx))
		require.Equal(t, status, admission.Status)
	}
	assertAdmission(reservedAdmission.ID, "canceled")
	assertAdmission(runningAdmission.ID, "interrupted")
	assertAdmission(unknownAdmission.ID, "interrupted")

	leases := []database.AgentExecutionLease{}
	require.NoError(t, db.Core.NewSelect().Model(&leases).Where("allocation_id = ?", reserved.Allocation.ID).Scan(ctx))
	require.Len(t, leases, 3)
	for i := range leases {
		require.NotNil(t, leases[i].EndedAt)
		require.NotNil(t, leases[i].EndReason)
	}

	// Suspended allocations reject new admissions until resumed.
	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.ErrorIs(t, err, errorx.ErrAgentAllocationLifecycleConflict)

	// Non-orchestrated deploys are ignored.
	require.NoError(t, store.RecordAllocationSuspended(ctx, 999999))
}

func TestAgentOrchestratorStoreResumeOperation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, deploy := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	// Resume is a no-op for a ready allocation.
	operation, err := store.BeginAllocationResume(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	require.Nil(t, operation)

	require.NoError(t, store.RecordAllocationSuspended(ctx, deploy.ID))

	operation, err = store.BeginAllocationResume(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	require.NotNil(t, operation)
	require.Equal(t, "resume", operation.OperationType)
	require.Equal(t, int64(0), operation.RuntimeGeneration)

	again, err := store.BeginAllocationResume(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	require.Equal(t, operation.ID, again.ID)

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "resuming", allocation.ObservedState)

	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute, OperationID: operation.ID})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, store.MarkOperationDispatched(ctx, operation.ID, "worker", operation.OperationKey))
	require.NoError(t, store.MarkResumeOperationRunning(ctx, operation.ID))
	require.NoError(t, store.CompleteResumeOperation(ctx, operation.ID, deploy.ID))

	allocation, err = store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "ready", allocation.ObservedState)
	require.Equal(t, int64(0), allocation.RuntimeGeneration)

	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
}

func TestAgentOrchestratorStoreFailResumeReturnsToSuspended(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, deploy := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)
	require.NoError(t, store.RecordAllocationSuspended(ctx, deploy.ID))

	operation, err := store.BeginAllocationResume(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: "worker", Limit: 1, Lease: time.Minute, OperationID: operation.ID})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, store.MarkOperationDispatched(ctx, operation.ID, "worker", operation.OperationKey))
	require.NoError(t, store.MarkResumeOperationRunning(ctx, operation.ID))
	require.NoError(t, store.FailResumeOperation(ctx, operation.ID, "runner unreachable"))

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "suspended", allocation.ObservedState)

	// The next chat starts a fresh resume operation.
	retry, err := store.BeginAllocationResume(ctx, reserved.Allocation.ID, types.AgentActor{Kind: types.AgentActorKindUser, ID: actorID})
	require.NoError(t, err)
	require.NotNil(t, retry)
	require.NotEqual(t, operation.ID, retry.ID)
}

func TestAgentOrchestratorStoreFailOperationsFenceCanceledAndStaleGenerations(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	_, err := db.Core.NewUpdate().Model((*database.AgentAllocation)(nil)).
		Set("runtime_generation = 1").
		Where("id = ?", reserved.Allocation.ID).
		Exec(ctx)
	require.NoError(t, err)

	tests := []struct {
		operationType string
		fail          func(int64) error
	}{
		{operationType: "provision", fail: func(operationID int64) error {
			return store.FailProvisionOperation(ctx, operationID, "stale provision", true)
		}},
		{operationType: "restart", fail: func(operationID int64) error {
			return store.FailRestartOperation(ctx, operationID, "stale restart")
		}},
		{operationType: "resume", fail: func(operationID int64) error {
			return store.FailResumeOperation(ctx, operationID, "stale resume")
		}},
		{operationType: "delete", fail: func(operationID int64) error {
			return store.FailDeleteOperation(ctx, operationID, "stale delete")
		}},
	}

	for i, tt := range tests {
		t.Run(tt.operationType+" stale", func(t *testing.T) {
			operation := &database.AgentAllocationOperation{
				AllocationID: reserved.Allocation.ID, OperationSequence: int64(100 + i),
				OperationKey: uuid.NewString(), OperationType: tt.operationType,
				Status: "running", RuntimeGeneration: 0, Payload: map[string]any{},
			}
			_, err := db.Core.NewInsert().Model(operation).Exec(ctx)
			require.NoError(t, err)

			err = tt.fail(operation.ID)
			require.ErrorContains(t, err, "is stale")

			persistedOperation := new(database.AgentAllocationOperation)
			require.NoError(t, db.Core.NewSelect().Model(persistedOperation).Where("id = ?", operation.ID).Scan(ctx))
			require.Equal(t, "running", persistedOperation.Status)
			allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
			require.NoError(t, err)
			require.Equal(t, "ready", allocation.ObservedState)
			require.Nil(t, allocation.BindingEndedAt)
			reservation := new(database.AgentCapacityReservation)
			require.NoError(t, db.Core.NewSelect().Model(reservation).Where("allocation_id = ?", reserved.Allocation.ID).Scan(ctx))
			require.Equal(t, "consumed", reservation.State)
		})

		t.Run(tt.operationType+" canceled", func(t *testing.T) {
			operation := &database.AgentAllocationOperation{
				AllocationID: reserved.Allocation.ID, OperationSequence: int64(200 + i),
				OperationKey: uuid.NewString(), OperationType: tt.operationType,
				Status: "canceled", RuntimeGeneration: 1, Payload: map[string]any{},
			}
			_, err := db.Core.NewInsert().Model(operation).Exec(ctx)
			require.NoError(t, err)
			require.NoError(t, tt.fail(operation.ID))

			persistedOperation := new(database.AgentAllocationOperation)
			require.NoError(t, db.Core.NewSelect().Model(persistedOperation).Where("id = ?", operation.ID).Scan(ctx))
			require.Equal(t, "canceled", persistedOperation.Status)
			allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
			require.NoError(t, err)
			require.Equal(t, "ready", allocation.ObservedState)
		})
	}
}

func TestAgentOrchestratorStoreLateCompletionResolvesUnknown(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	sessionUUID := uuid.NewString()
	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "unknown", time.Hour))

	// The proxied request actually succeeded after reconciliation had marked
	// the admission unknown: the late completion resolves it and frees the
	// session.
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "completed", time.Hour))
	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", admission.ID).Scan(ctx))
	require.Equal(t, "completed", persisted.Status)
	require.NotNil(t, persisted.CompletedAt)
	lease := new(database.AgentExecutionLease)
	require.NoError(t, db.Core.NewSelect().Model(lease).Where("admission_id = ?", admission.ID).Scan(ctx))
	require.NotNil(t, lease.EndedAt)
	require.Equal(t, "completed", *lease.EndReason)

	_, err = store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, sessionUUID))
	require.NoError(t, err)
}

func TestAgentOrchestratorStoreLateCompletionRejectsStaleGeneration(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "unknown", time.Hour))

	// The allocation moved to a new generation before the late completion.
	_, err = db.Core.NewUpdate().Model((*database.AgentAllocation)(nil)).
		Set("runtime_generation = 1").Set("accepted_generation = 1").
		Where("id = ?", reserved.Allocation.ID).Exec(ctx)
	require.NoError(t, err)

	err = store.CompleteExecutionAdmission(ctx, admission.ID, "completed", time.Hour)
	require.ErrorContains(t, err, "not active")
	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", admission.ID).Scan(ctx))
	require.Equal(t, "unknown", persisted.Status)
}

func TestAgentOrchestratorStoreHeartbeatRejectsNonRunningAdmission(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, _ := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	// A reserved admission does not heartbeat yet.
	require.ErrorIs(t, store.HeartbeatExecutionAdmission(ctx, admission.ID), database.ErrAgentExecutionLeaseLost)

	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))
	require.NoError(t, store.HeartbeatExecutionAdmission(ctx, admission.ID))
	require.NoError(t, store.CompleteExecutionAdmission(ctx, admission.ID, "unknown", time.Hour))
	// An unknown admission no longer accepts heartbeats.
	require.ErrorIs(t, store.HeartbeatExecutionAdmission(ctx, admission.ID), database.ErrAgentExecutionLeaseLost)
}

func TestAgentOrchestratorStoreRecordAllocationSuspendedIsIdempotent(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	reserved, deploy := provisionReadyAgentAllocation(t, ctx, db, store, logical.ID, actorID)

	admission, err := store.AcquireExecutionAdmission(ctx, executionAdmissionInput(reserved.Allocation.ID, actorID, uuid.NewString()))
	require.NoError(t, err)
	require.NoError(t, store.MarkExecutionAdmissionRunning(ctx, admission.ID))

	require.NoError(t, store.RecordAllocationSuspended(ctx, deploy.ID))
	// A repeated Sleeping event (for example a webhook retry) is a no-op.
	require.NoError(t, store.RecordAllocationSuspended(ctx, deploy.ID))

	allocation, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	require.Equal(t, "suspended", allocation.ObservedState)
	persisted := new(database.AgentExecutionAdmission)
	require.NoError(t, db.Core.NewSelect().Model(persisted).Where("id = ?", admission.ID).Scan(ctx))
	require.Equal(t, "interrupted", persisted.Status)
}

func TestAgentOrchestratorStoreFindLiveAllocationSession(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	logical := createPublicLogicalAgent(t, ctx, db, uuid.NewString(), "logical-"+uuid.NewString())
	actorID := uuid.NewString()
	sessionUUID := uuid.NewString()
	_, err := store.ReserveAllocation(ctx, database.ReserveAgentAllocationInput{
		LogicalInstanceID: logical.ID, ActorKind: "user", ActorID: actorID,
		RuntimePrincipal: actorID, BillingOwner: actorID,
		InstanceQuota: 1, SessionUUID: sessionUUID, SessionName: "Review",
	})
	require.NoError(t, err)

	found, err := store.FindLiveAllocationSession(ctx, logical.ID, "user", actorID, sessionUUID)
	require.NoError(t, err)
	require.Equal(t, sessionUUID, found.SessionUUID)
	require.Equal(t, "Review", found.SessionName)

	_, err = store.FindLiveAllocationSession(ctx, logical.ID, "user", uuid.NewString(), sessionUUID)
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)

	_, err = db.Core.NewUpdate().Model((*database.AgentAllocation)(nil)).
		Set("binding_ended_at = CURRENT_TIMESTAMP").
		Where("logical_instance_id = ? AND actor_id = ?", logical.ID, actorID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = store.FindLiveAllocationSession(ctx, logical.ID, "user", actorID, sessionUUID)
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
}
