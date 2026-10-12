package database_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentOrchestratorStoreGuestAllocationReuseAndIsolation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	creator := uuid.NewString()
	logical := createPublicLogicalAgent(t, ctx, db, creator, "logical-"+uuid.NewString())
	insertGuestUsers(t, ctx, db, creator)

	guestA := "guest-" + uuid.NewString()
	guestB := "guest-" + uuid.NewString()
	first, err := store.ReserveAllocation(ctx, guestReserveInput(logical.ID, guestA, creator, 2))
	require.NoError(t, err)
	again, err := store.ReserveAllocation(ctx, guestReserveInput(logical.ID, guestA, creator, 2))
	require.NoError(t, err)
	require.Equal(t, first.Allocation.ID, again.Allocation.ID)

	other, err := store.ReserveAllocation(ctx, guestReserveInput(logical.ID, guestB, creator, 2))
	require.NoError(t, err)
	require.NotEqual(t, first.Allocation.ID, other.Allocation.ID)
	require.Equal(t, "guest", first.Allocation.ActorKind)
	require.Equal(t, creator, *first.Allocation.RuntimePrincipal)
	require.Equal(t, creator, *first.Allocation.BillingOwner)
}

func TestAgentOrchestratorStoreGuestChargesCreatorQuota(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	creator := uuid.NewString()
	logical := createPublicLogicalAgent(t, ctx, db, creator, "logical-"+uuid.NewString())
	insertGuestUsers(t, ctx, db, creator)

	_, err := store.ReserveAllocation(ctx, guestReserveInput(logical.ID, "guest-a", creator, 1))
	require.NoError(t, err)
	_, err = store.ReserveAllocation(ctx, guestReserveInput(logical.ID, "guest-b", creator, 1))
	require.ErrorIs(t, err, errorx.ErrInstanceQuotaExceeded)
}

func TestAgentOrchestratorStoreGuestSessionHiddenFromUserAPIs(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	creator := uuid.NewString()
	logical := createPublicLogicalAgent(t, ctx, db, creator, "logical-"+uuid.NewString())
	insertGuestUsers(t, ctx, db, creator)
	reserved, deploy := provisionReadyGuestAllocation(t, ctx, db, store, logical.ID, "guest-actor", creator)
	_ = deploy

	input := guestAdmissionInput(reserved.Allocation.ID, "guest-actor", uuid.NewString())
	input.PinnedRuntimeGeneration = reserved.Allocation.RuntimeGeneration
	admission, err := store.AcquireExecutionAdmission(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "guest", admission.ActorKind)

	_, err = sessionStore.FindByUUID(ctx, input.SessionUUID)
	require.Error(t, err)
	sessions, _, err := sessionStore.ListByInstanceID(ctx, logical.ID)
	require.NoError(t, err)
	require.Empty(t, sessions)

	allocSession := new(database.AgentAllocationSession)
	require.NoError(t, db.Core.NewSelect().Model(allocSession).
		Where("allocation_id = ? AND session_uuid = ?", reserved.Allocation.ID, input.SessionUUID).
		Scan(ctx))
	require.NotZero(t, allocSession.ID)
}

func TestAgentOrchestratorStoreGuestUsageLimitAtomic(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	creator := uuid.NewString()
	logical := createPublicLogicalAgent(t, ctx, db, creator, "logical-"+uuid.NewString())
	insertGuestUsers(t, ctx, db, creator)
	reserved, _ := provisionReadyGuestAllocation(t, ctx, db, store, logical.ID, "guest-limit", creator)
	maxCount := int64(1)

	first := guestAdmissionInput(reserved.Allocation.ID, "guest-limit", uuid.NewString())
	first.PinnedRuntimeGeneration = reserved.Allocation.RuntimeGeneration
	first.GuestUsageMaxCount = &maxCount
	_, err := store.AcquireExecutionAdmission(ctx, first)
	require.NoError(t, err)

	second := guestAdmissionInput(reserved.Allocation.ID, "guest-limit", uuid.NewString())
	second.PinnedRuntimeGeneration = reserved.Allocation.RuntimeGeneration
	second.GuestUsageMaxCount = &maxCount
	_, err = store.AcquireExecutionAdmission(ctx, second)
	require.ErrorIs(t, err, errorx.ErrAgentGuestUsageLimitExceeded)

	var useCount int64
	require.NoError(t, db.Core.NewSelect().
		TableExpr("agent_guest_usage_counters").
		Column("use_count").
		Where("logical_instance_id = ? AND guest_actor_id = ?", logical.ID, "guest-limit").
		Scan(ctx, &useCount))
	require.Equal(t, int64(1), useCount)
}

func TestAgentOrchestratorStoreGuestUsageUnlimitedBypassesCounter(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	creator := uuid.NewString()
	logical := createPublicLogicalAgent(t, ctx, db, creator, "logical-"+uuid.NewString())
	insertGuestUsers(t, ctx, db, creator)
	reserved, _ := provisionReadyGuestAllocation(t, ctx, db, store, logical.ID, "guest-unlim", creator)

	limited := guestAdmissionInput(reserved.Allocation.ID, "guest-unlim", uuid.NewString())
	limited.PinnedRuntimeGeneration = reserved.Allocation.RuntimeGeneration
	maxCount := int64(1)
	limited.GuestUsageMaxCount = &maxCount
	_, err := store.AcquireExecutionAdmission(ctx, limited)
	require.NoError(t, err)

	unlimited := guestAdmissionInput(reserved.Allocation.ID, "guest-unlim", uuid.NewString())
	unlimited.PinnedRuntimeGeneration = reserved.Allocation.RuntimeGeneration
	_, err = store.AcquireExecutionAdmission(ctx, unlimited)
	require.NoError(t, err)

	var useCount int64
	require.NoError(t, db.Core.NewSelect().
		TableExpr("agent_guest_usage_counters").
		Column("use_count").
		Where("logical_instance_id = ? AND guest_actor_id = ?", logical.ID, "guest-unlim").
		Scan(ctx, &useCount))
	require.Equal(t, int64(1), useCount)
}

func TestAgentOrchestratorStoreGuestConcurrentReuse(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	creator := uuid.NewString()
	logical := createPublicLogicalAgent(t, ctx, db, creator, "logical-"+uuid.NewString())
	insertGuestUsers(t, ctx, db, creator)

	const workers = 8
	var wg sync.WaitGroup
	ids := make([]int64, workers)
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			reserved, err := store.ReserveAllocation(ctx, guestReserveInput(logical.ID, "guest-concurrent", creator, 1))
			errs[i] = err
			if err == nil {
				ids[i] = reserved.Allocation.ID
			}
		}(i)
	}
	wg.Wait()
	var first int64
	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		if first == 0 {
			first = ids[i]
		}
		require.Equal(t, first, ids[i])
	}
}

func TestAgentOrchestratorFoundationGuestSchemaPresent(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()

	for _, table := range []string{"agent_allocation_sessions", "agent_guest_usage_counters"} {
		exists, err := db.Core.NewSelect().TableExpr("information_schema.tables").
			Where("table_name = ?", table).Exists(ctx)
		require.NoError(t, err)
		require.True(t, exists, table)
	}

	absent, err := db.Core.NewSelect().TableExpr("information_schema.columns").
		Where("table_name = ? AND column_name = ?", "agent_instance_sessions", "actor_kind").Exists(ctx)
	require.NoError(t, err)
	require.False(t, absent)

	upPath := filepath.Join("builder", "store", "database", "migrations", "20260917151104_add_agent_orchestrator_foundation.up.sql")
	downPath := filepath.Join("builder", "store", "database", "migrations", "20260917151104_add_agent_orchestrator_foundation.down.sql")
	upSQL, err := os.ReadFile(upPath)
	require.NoError(t, err)
	downSQL, err := os.ReadFile(downPath)
	require.NoError(t, err)
	require.Contains(t, string(upSQL), "agent_allocation_sessions")
	require.Contains(t, string(upSQL), "agent_guest_usage_counters")
	require.NotContains(t, string(upSQL), "agent_instance_sessions_actor_idx")
	require.Contains(t, string(downSQL), "DROP TABLE IF EXISTS agent_allocation_sessions")
	require.Contains(t, string(downSQL), "DROP TABLE IF EXISTS agent_guest_usage_counters")
}

func guestReserveInput(logicalID int64, guestActorID, creator string, quota int64) database.ReserveAgentAllocationInput {
	return database.ReserveAgentAllocationInput{
		LogicalInstanceID: logicalID, ActorKind: "guest", ActorID: guestActorID,
		RuntimePrincipal: creator, BillingOwner: creator, InstanceQuota: quota,
	}
}

func guestAdmissionInput(allocationID int64, guestActorID, sessionUUID string) database.AcquireAgentExecutionAdmissionInput {
	return database.AcquireAgentExecutionAdmissionInput{
		AllocationID: allocationID, ActorID: guestActorID, ActorKind: "guest", SessionUUID: sessionUUID,
		SessionName: "guest-chat", ExecutionID: uuid.NewString(),
		MaxConcurrentPerAllocation: 4, ReservedTimeout: 30 * time.Second,
		HeartbeatTimeout: 90 * time.Second, UnknownCapacityHold: time.Hour,
	}
}

func insertGuestUsers(t *testing.T, ctx context.Context, db *database.DB, uuids ...string) {
	t.Helper()
	for _, id := range uuids {
		_, err := db.Core.NewInsert().Model(&database.User{
			GitID: time.Now().UnixNano(), NickName: "user", Username: "user-" + uuid.NewString(),
			Password: "unused", UUID: id,
		}).Exec(ctx)
		require.NoError(t, err)
	}
}

func provisionReadyGuestAllocation(t *testing.T, ctx context.Context, db *database.DB, store database.AgentOrchestratorStore, logicalID int64, guestActorID, creator string) (*database.ReserveAgentAllocationResult, *database.Deploy) {
	t.Helper()
	user := &database.User{}
	require.NoError(t, db.Core.NewSelect().Model(user).Where("uuid = ?", creator).Scan(ctx))
	deploy := &database.Deploy{UserID: user.ID, GitPath: "agent", GitBranch: "main", Template: "csgclaw", Hardware: "cpu"}
	require.NoError(t, database.NewDeployTaskStoreWithDB(db).CreateDeploy(ctx, deploy))
	reserved, err := store.ReserveAllocation(ctx, guestReserveInput(logicalID, guestActorID, creator, 1))
	require.NoError(t, err)
	owner := "worker-" + uuid.NewString()
	claimed, err := store.ClaimPendingOperations(ctx, database.ClaimAgentAllocationOperationsInput{Owner: owner, Limit: 1, Lease: time.Minute})
	require.NoError(t, err)
	require.NoError(t, store.MarkOperationDispatched(ctx, claimed[0].ID, owner, claimed[0].OperationKey))
	require.NoError(t, store.MarkProvisionOperationRunning(ctx, claimed[0].ID))
	require.NoError(t, store.CompleteProvisionOperation(ctx, claimed[0].ID, deploy.ID))
	ready, err := store.FindAllocationByID(ctx, reserved.Allocation.ID)
	require.NoError(t, err)
	reserved.Allocation = *ready
	return reserved, deploy
}
