package component

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestEnsureNamespaceRelationshipWritesUserOwner verifies personal namespaces receive an owner tuple.
func TestEnsureNamespaceRelationshipWritesUserOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := ensureNamespaceRelationship(ctx, authorizer, database.Namespace{
		Path: "alice", UUID: "namespace-uuid", NamespaceType: database.UserNamespace,
	}, "user-uuid")
	require.NoError(t, err)
}

// TestEnsureNamespaceRelationshipWritesOrganization verifies organization namespaces receive an organization tuple.
func TestEnsureNamespaceRelationshipWritesOrganization(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, "organization-uuid"),
		Relation: rebac.RelationOrganization,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := ensureNamespaceRelationship(ctx, authorizer, database.Namespace{
		Path: "acme", UUID: "namespace-uuid", NamespaceType: database.OrgNamespace,
	}, "organization-uuid")
	require.NoError(t, err)
}

// TestEnsureUserObjectOwnerRelationshipWritesOwner verifies a user object receives its owner tuple.
func TestEnsureUserObjectOwnerRelationshipWritesOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject: rebac.UserSubject("user-uuid"), Relation: rebac.RelationOwner, Object: rebac.UserObject("user-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	require.NoError(t, ensureUserObjectOwnerRelationship(ctx, authorizer, "user-uuid"))
}

// TestDeleteUserObjectOwnerRelationshipDeletesOwner verifies a user object owner tuple is removed.
func TestDeleteUserObjectOwnerRelationshipDeletesOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject: rebac.UserSubject("user-uuid"), Relation: rebac.RelationOwner, Object: rebac.UserObject("user-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	require.NoError(t, deleteUserObjectOwnerRelationship(ctx, authorizer, relationship))
}

// TestLoadUserRepositoryRelationshipsResolvesPersonalAndOrganizationRepositories verifies bulk deletion preserves namespace ownership semantics.
func TestLoadUserRepositoryRelationshipsResolvesPersonalAndOrganizationRepositories(t *testing.T) {
	ctx := context.Background()
	repoStore := mockdatabase.NewMockRepoStore(t)
	namespaceStore := mockdatabase.NewMockNamespaceStore(t)
	organizationStore := mockdatabase.NewMockOrgStore(t)
	user := database.User{ID: 7, Username: "alice", UUID: "alice-uuid"}
	organizationUUID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	repositories := []database.Repository{
		{ID: 41, Path: "alice/personal"},
		{ID: 42, Path: "acme/shared"},
	}

	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 0).Return(repositories, nil).Once()
	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 1).Return([]database.Repository{}, nil).Once()
	namespaceStore.EXPECT().FindByPath(ctx, "alice").Return(database.Namespace{
		Path: "alice", NamespaceType: database.UserNamespace, User: database.User{UUID: user.UUID},
	}, nil).Once()
	namespaceStore.EXPECT().FindByPath(ctx, "acme").Return(database.Namespace{
		Path: "acme", NamespaceType: database.OrgNamespace,
	}, nil).Once()
	organizationStore.EXPECT().FindByPath(ctx, "acme").Return(database.Organization{UUID: organizationUUID}, nil).Once()

	gotRepositories, relationships, err := loadUserRepositoryRelationships(
		ctx, repoStore, namespaceStore, organizationStore, user,
	)
	require.NoError(t, err)
	require.Equal(t, repositories, gotRepositories)
	require.Equal(t, []rebac.Relationship{
		{Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(41)},
		{
			Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organizationUUID.String()),
			Relation: rebac.RelationOrganization,
			Object:   rebac.RepositoryObject(42),
		},
	}, relationships)
}

// TestLoadUserRepositoryRelationshipsResolvesDeletedPersonalNamespace verifies retained repositories can be cleaned after account soft deletion.
func TestLoadUserRepositoryRelationshipsResolvesDeletedPersonalNamespace(t *testing.T) {
	ctx := context.Background()
	repoStore := mockdatabase.NewMockRepoStore(t)
	namespaceStore := mockdatabase.NewMockNamespaceStore(t)
	user := database.User{ID: 7, Username: "alice", UUID: "alice-uuid"}
	repository := database.Repository{ID: 41, Path: "alice/personal"}

	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 0).Return([]database.Repository{repository}, nil).Once()
	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 1).Return([]database.Repository{}, nil).Once()
	namespaceStore.EXPECT().FindByPath(ctx, "alice").Return(database.Namespace{}, errors.New("namespace deleted")).Once()

	_, relationships, err := loadUserRepositoryRelationships(ctx, repoStore, namespaceStore, nil, user)
	require.NoError(t, err)
	require.Equal(t, []rebac.Relationship{{
		Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(repository.ID),
	}}, relationships)
}

// TestDeleteRepositoryNamespaceRelationshipsRemovesOrganizationDirect verifies repository cleanup removes both organization relations.
func TestDeleteRepositoryNamespaceRelationshipsRemovesOrganizationDirect(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	organizationRelationship := rebac.Relationship{
		Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, "organization-uuid"),
		Relation: rebac.RelationOrganization,
		Object:   rebac.RepositoryObject(42),
	}
	directRelationship := organizationRelationship
	directRelationship.Relation = rebac.RelationOrganizationDirect
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{organizationRelationship, directRelationship}).Return(nil).Once()

	require.NoError(t, deleteRepositoryNamespaceRelationships(ctx, authorizer, []rebac.Relationship{organizationRelationship}))
}

// TestDeleteUserNamespaceRelationshipRemovesOwner verifies user deletion removes the personal namespace owner tuple.
func TestDeleteUserNamespaceRelationshipRemovesOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	correlationID := rebac.BatchCheckCorrelationID(0)
	authorizer.EXPECT().BatchCheck(ctx, rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{{
		CorrelationID: correlationID,
		Check: rebac.CheckRequest{
			Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
			Consistency: rebac.ConsistencyHigher,
		},
	}}}).Return(rebac.BatchCheckResult{Results: map[string]rebac.BatchCheckOutcome{
		correlationID: {Decision: rebac.Decision{Allowed: true}},
	}}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := deleteNamespaceRelationship(ctx, authorizer, relationship)
	require.NoError(t, err)
}

// TestDeleteUserNamespaceRelationshipSkipsMissingTuple verifies repeated user deletion is idempotent.
func TestDeleteUserNamespaceRelationshipSkipsMissingTuple(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	correlationID := rebac.BatchCheckCorrelationID(0)
	authorizer.EXPECT().BatchCheck(ctx, rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{{
		CorrelationID: correlationID,
		Check: rebac.CheckRequest{
			Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
			Consistency: rebac.ConsistencyHigher,
		},
	}}}).Return(rebac.BatchCheckResult{Results: map[string]rebac.BatchCheckOutcome{
		correlationID: {Decision: rebac.Decision{}},
	}}, nil).Once()

	err := deleteNamespaceRelationship(ctx, authorizer, relationship)
	require.NoError(t, err)
}

// expectOrganizationReBACCleanup verifies direct member roles and the namespace tuple are deleted.
func expectOrganizationReBACCleanup(
	t *testing.T,
	authorizer *mockrebac.MockAuthorizer,
	cleanup types.OrganizationReBACCleanup,
) {
	expectOrganizationReBACCleanups(t, authorizer, []types.OrganizationReBACCleanup{cleanup})
}

// expectOrganizationReBACCleanups verifies member cleanup and batched namespace tuple deletion.
func expectOrganizationReBACCleanups(
	t *testing.T,
	authorizer *mockrebac.MockAuthorizer,
	cleanups []types.OrganizationReBACCleanup,
) {
	t.Helper()
	namespaceRelationships := make([]rebac.Relationship, 0, len(cleanups))
	for _, cleanup := range cleanups {
		checks := make([]rebac.BatchCheckItem, 0, len(cleanup.UserUUIDs)*len(organizationMemberRelations))
		results := make(map[string]rebac.BatchCheckOutcome, cap(checks))
		memberRelationships := make([]rebac.Relationship, 0, cap(checks))
		for userIndex, userUUID := range cleanup.UserUUIDs {
			for relationIndex, relation := range organizationMemberRelations {
				correlationID := organizationMemberCorrelationID(userIndex, relationIndex)
				checks = append(checks, rebac.BatchCheckItem{
					CorrelationID: correlationID,
					Check: rebac.CheckRequest{
						Subject: rebac.UserSubject(userUUID), Relation: relation,
						Object: rebac.OrganizationObject(cleanup.OrganizationUUID), Consistency: rebac.ConsistencyHigher,
					},
				})
				results[correlationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: true}}
				memberRelationships = append(memberRelationships, rebac.Relationship{
					Subject: rebac.UserSubject(userUUID), Relation: relation,
					Object: rebac.OrganizationObject(cleanup.OrganizationUUID),
				})
			}
		}
		if len(checks) > 0 {
			authorizer.EXPECT().BatchCheck(mock.Anything, rebac.BatchCheckRequest{Checks: checks}).
				Return(rebac.BatchCheckResult{Results: results}, nil).Once()
			authorizer.EXPECT().Delete(mock.Anything, memberRelationships).Return(nil).Once()
		}
		if cleanup.NamespaceUUID != "" {
			namespaceRelationships = append(namespaceRelationships, rebac.Relationship{
				Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, cleanup.OrganizationUUID),
				Relation: rebac.RelationOrganization, Object: rebac.NamespaceObject(cleanup.NamespaceUUID),
			})
		}
	}
	if len(namespaceRelationships) == 0 {
		return
	}
	checks := make([]rebac.BatchCheckItem, 0, len(namespaceRelationships))
	results := make(map[string]rebac.BatchCheckOutcome, len(namespaceRelationships))
	for index, relationship := range namespaceRelationships {
		correlationID := rebac.BatchCheckCorrelationID(index)
		checks = append(checks, rebac.BatchCheckItem{
			CorrelationID: correlationID,
			Check: rebac.CheckRequest{
				Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
				Consistency: rebac.ConsistencyHigher,
			},
		})
		results[correlationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: true}}
	}
	authorizer.EXPECT().BatchCheck(mock.Anything, rebac.BatchCheckRequest{Checks: checks}).
		Return(rebac.BatchCheckResult{Results: results}, nil).Once()
	authorizer.EXPECT().Delete(mock.Anything, namespaceRelationships).Return(nil).Once()
}

// TestDeleteOrganizationReBACRelationshipsRemovesMembersAndNamespace verifies organization deletion removes all direct tuples.
func TestDeleteOrganizationReBACRelationshipsRemovesMembersAndNamespace(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	cleanup := types.OrganizationReBACCleanup{
		OrganizationUUID: "organization-uuid",
		NamespaceUUID:    "namespace-uuid",
		UserUUIDs:        []string{"user-uuid"},
	}
	expectOrganizationReBACCleanup(t, authorizer, cleanup)

	err := deleteOrganizationReBACRelationships(ctx, authorizer, []types.OrganizationReBACCleanup{cleanup})
	require.NoError(t, err)
}

// TestReconcileOrganizationMemberRelationshipsSplitsLargeBatches verifies OpenFGA's batch limit is respected.
func TestReconcileOrganizationMemberRelationshipsSplitsLargeBatches(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	userUUIDs := make([]string, 17)
	for index := range userUUIDs {
		userUUIDs[index] = fmt.Sprintf("user-%02d", index)
	}
	batchSizes := make([]int, 0, 2)
	authorizer.EXPECT().BatchCheck(ctx, mock.Anything).RunAndReturn(func(_ context.Context, request rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
		batchSizes = append(batchSizes, len(request.Checks))
		results := make(map[string]rebac.BatchCheckOutcome, len(request.Checks))
		for _, check := range request.Checks {
			results[check.CorrelationID] = rebac.BatchCheckOutcome{}
		}
		return rebac.BatchCheckResult{Results: results}, nil
	}).Twice()

	err := reconcileOrganizationMemberRelationships(ctx, authorizer, "organization-uuid", userUUIDs, nil)
	require.NoError(t, err)
	require.Equal(t, []int{48, 3}, batchSizes)
}

// TestDeleteNamespaceRelationshipsSplitsLargeBatches verifies the OpenFGA batch limit for namespace cleanup.
func TestDeleteNamespaceRelationshipsSplitsLargeBatches(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationships := make([]rebac.Relationship, 51)
	for index := range relationships {
		relationships[index] = rebac.Relationship{
			Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, fmt.Sprintf("org-%03d", index)),
			Relation: rebac.RelationOrganization,
			Object:   rebac.NamespaceObject(fmt.Sprintf("namespace-%03d", index)),
		}
	}
	batchSizes := make([]int, 0, 2)
	authorizer.EXPECT().BatchCheck(ctx, mock.Anything).RunAndReturn(func(_ context.Context, request rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
		batchSizes = append(batchSizes, len(request.Checks))
		results := make(map[string]rebac.BatchCheckOutcome, len(request.Checks))
		for _, check := range request.Checks {
			results[check.CorrelationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: true}}
		}
		return rebac.BatchCheckResult{Results: results}, nil
	}).Twice()
	authorizer.EXPECT().Delete(ctx, mock.MatchedBy(func(batch []rebac.Relationship) bool {
		return len(batch) == 50
	})).Return(nil).Once()
	authorizer.EXPECT().Delete(ctx, mock.MatchedBy(func(batch []rebac.Relationship) bool {
		return len(batch) == 1
	})).Return(nil).Once()

	require.NoError(t, deleteNamespaceRelationships(ctx, authorizer, relationships))
	require.Equal(t, []int{50, 1}, batchSizes)
}

// TestRebacTupleWorkerNextRetry preserves River's fourth-power backoff and jitter.
// Error history determines the delay even when snoozes affect the attempt count.
func TestRebacTupleWorkerNextRetry(t *testing.T) {
	worker := NewRebacTupleWorker(nil)
	for _, errorCount := range []int{0, 1, 2, 5, 10} {
		t.Run(fmt.Sprintf("errors=%d", errorCount), func(t *testing.T) {
			job := &river.Job[workhub.RebacTupleArgs]{JobRow: &rivertype.JobRow{
				Attempt: 42,
				Errors:  make([]rivertype.AttemptError, errorCount),
			}}
			n := time.Duration(errorCount + 1)
			baseDelay := n * n * n * n * time.Second
			before := time.Now()
			next := worker.NextRetry(job)
			after := time.Now()
			require.False(t, next.Before(before.Add(baseDelay-baseDelay/10)))
			require.False(t, next.After(after.Add(baseDelay+baseDelay/10)))
		})
	}
}

// TestRebacTupleWorkerRequiresAuthorizer reports a missing execution dependency.
func TestRebacTupleWorkerRequiresAuthorizer(t *testing.T) {
	event := types.NewRebacTuple("test", types.TupleOperation{Relationships: []rebac.Relationship{{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}}})
	w := NewRebacTupleWorker(nil)
	require.ErrorContains(t, w.Work(context.Background(), &river.Job[workhub.RebacTupleArgs]{Args: workhub.RebacTupleArgs{SubjectKey: workhub.RebacSequenceKey(event.Operations[0].Relationships[0]), RebacTuple: event}}), "tuple authorizer is required")
}

// TestRebacTupleWorkerUsesSingleAuthorizer applies every event through the injected instance.
func TestRebacTupleWorkerUsesSingleAuthorizer(t *testing.T) {
	a := &recordingAuthorizer{}
	w := NewRebacTupleWorker(a)
	for _, model := range []string{types.OpenFgaAuthorizationModelIDVer1_0, types.OpenFgaAuthorizationModelIDVer1_2, "future-model-1.3"} {
		t.Run(model, func(t *testing.T) {
			event := types.NewRebacTuple("test", types.TupleOperation{Relationships: []rebac.Relationship{{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}}})
			event.ModelID = model
			before := len(a.ops)
			require.NoError(t, w.Work(context.Background(), &river.Job[workhub.RebacTupleArgs]{Args: workhub.RebacTupleArgs{SubjectKey: workhub.RebacSequenceKey(event.Operations[0].Relationships[0]), RebacTuple: event}}))
			require.Equal(t, event.Operations, a.ops[before:])
		})
	}
}

// recordingAuthorizer records external effects and injects partial failures.
type recordingAuthorizer struct {
	rebac.Authorizer
	ops       []types.TupleOperation
	failWrite bool
}

// Write records the attempted write, including failed attempts.
func (a *recordingAuthorizer) Write(_ context.Context, relationships []rebac.Relationship) error {
	a.ops = append(a.ops, types.TupleOperation{Relationships: relationships})
	if a.failWrite {
		return errors.New("FGA unavailable")
	}
	return nil
}

// Delete records a successful delete attempt.
func (a *recordingAuthorizer) Delete(_ context.Context, relationships []rebac.Relationship) error {
	a.ops = append(a.ops, types.TupleOperation{Delete: true, Relationships: relationships})
	return nil
}

// TestApplyValidatesEntireEvent prevents partial writes for malformed payloads.
func TestApplyValidatesEntireEvent(t *testing.T) {
	a := &recordingAuthorizer{}
	r := rebac.Relationship{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}
	event := types.NewRebacTuple("test", types.TupleOperation{Relationships: []rebac.Relationship{r}}, types.TupleOperation{})
	require.Error(t, applyRebacTuple(context.Background(), a, event))
	require.Empty(t, a.ops)
}

// TestApplyRetriesPartialMutation verifies delete/write replay preserves order.
func TestApplyRetriesPartialMutation(t *testing.T) {
	a := &recordingAuthorizer{failWrite: true}
	r := rebac.Relationship{Subject: rebac.UserSubject("u"), Relation: rebac.RelationReader, Object: rebac.RepositoryObject(1)}
	w := r
	w.Relation = rebac.RelationWriter
	event := types.NewRebacTuple("test", types.TupleOperation{Delete: true, Relationships: []rebac.Relationship{r}}, types.TupleOperation{Relationships: []rebac.Relationship{w}})
	require.ErrorContains(t, applyRebacTuple(context.Background(), a, event), "FGA unavailable")
	a.failWrite = false
	require.NoError(t, applyRebacTuple(context.Background(), a, event))
	require.Len(t, a.ops, 4)
	require.Equal(t, a.ops[0:2], a.ops[2:4])
}

// TestApplyBoundsOpenFGABatches verifies snapshots larger than one request.
func TestApplyBoundsOpenFGABatches(t *testing.T) {
	a := &recordingAuthorizer{}
	var relationships []rebac.Relationship
	for i := int64(1); i <= int64(rebac.DefaultMaxBatchSize+1); i++ {
		relationships = append(relationships, rebac.Relationship{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(i)})
	}
	require.NoError(t, applyRebacTuple(context.Background(), a, types.NewRebacTuple("test", types.TupleOperation{Relationships: relationships})))
	require.Len(t, a.ops, 2)
	require.Len(t, a.ops[0].Relationships, rebac.DefaultMaxBatchSize)
	require.Len(t, a.ops[1].Relationships, 1)
}

// TestApplyDeduplicatesWithinOperations preserves intentional delete-then-write transitions.
func TestApplyDeduplicatesWithinOperations(t *testing.T) {
	a := &recordingAuthorizer{}
	r := rebac.Relationship{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}
	event := types.NewRebacTuple("test", types.TupleOperation{Delete: true, Relationships: []rebac.Relationship{r, r}}, types.TupleOperation{Relationships: []rebac.Relationship{r, r}})
	require.NoError(t, applyRebacTuple(context.Background(), a, event))
	require.Len(t, a.ops, 2)
	require.True(t, a.ops[0].Delete)
	require.False(t, a.ops[1].Delete)
	require.Equal(t, []rebac.Relationship{r}, a.ops[0].Relationships)
	require.Equal(t, []rebac.Relationship{r}, a.ops[1].Relationships)
	require.Len(t, event.Operations[0].Relationships, 2)
}

// retryTupleAuthorizer simulates a temporary OpenFGA failure without touching business rows.
type retryTupleAuthorizer struct {
	rebac.Authorizer
	attempts atomic.Int32
}

// Write succeeds only after River retries the first failed attempt.
func (a *retryTupleAuthorizer) Write(context.Context, []rebac.Relationship) error {
	if a.attempts.Add(1) == 1 {
		return errors.New("temporary OpenFGA failure")
	}
	return nil
}

// TestRebacTupleRiverConsumption verifies persisted payload decoding and actual River retries.
func TestRebacTupleRiverConsumption(t *testing.T) {
	db := tests.InitTransactionTestDBWithHooks()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	user := &database.User{Username: "river-tuple-user", UUID: "river-tuple-user"}
	require.NoError(t, database.NewUserStoreWithDBAndJobClients(db, nil, producer).Create(ctx, user, &database.Namespace{Path: user.Username, UUID: "river-tuple-ns"}))
	pool, ok := db.GetPGXPool()
	require.True(t, ok)
	authorizer := &retryTupleAuthorizer{}
	worker := NewRebacTupleWorker(authorizer)
	client, err := NewRebacTupleWorkClient(ctx, pool.Config().ConnString(), worker)
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, client.Stop(stop))
	})
	require.Eventually(t, func() bool {
		var state string
		if err := db.BunDB.NewSelect().Table("river_job").Column("state").Where("kind = ?", workhub.RebacTupleQueue).Scan(ctx, &state); err != nil {
			return false
		}
		return state == "completed"
	}, 15*time.Second, 100*time.Millisecond)
	require.Equal(t, int32(2), authorizer.attempts.Load())
}

// orderedTupleAuthorizer pauses the first grant while independent sequences proceed.
type orderedTupleAuthorizer struct {
	rebac.Authorizer
	mu           sync.Mutex
	first        chan struct{}
	release      chan struct{}
	attempts     int
	present      map[string]bool
	orderedKey   string
	deletedEarly bool
}

// Write fails the paused first attempt, then applies the retried grant.
func (a *orderedTupleAuthorizer) Write(ctx context.Context, relationships []rebac.Relationship) error {
	key := workhub.RebacSequenceKey(relationships[0])
	a.mu.Lock()
	first := false
	if key == a.orderedKey {
		a.attempts++
		first = a.attempts == 1
	}
	a.mu.Unlock()
	if first {
		close(a.first)
		select {
		case <-a.release:
			return errors.New("temporary grant failure")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.present[key] = true
	return nil
}

// Delete records whether revocation overtook its preceding grant.
func (a *orderedTupleAuthorizer) Delete(_ context.Context, relationships []rebac.Relationship) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, relationship := range relationships {
		key := workhub.RebacSequenceKey(relationship)
		if key == a.orderedKey && !a.present[key] {
			a.deletedEarly = true
		}
		a.present[key] = false
	}
	return nil
}

// TestRebacTupleRiverSubjectOrder verifies retries and isolation across two consumers.
// Concrete subjects serialize across objects; wildcard visibility serializes per knowledge base.
func TestRebacTupleRiverSubjectOrder(t *testing.T) {
	for _, wildcard := range []bool{false, true} {
		t.Run(fmt.Sprintf("wildcard=%t", wildcard), func(t *testing.T) {
			db := tests.InitTransactionTestDBWithHooks()
			t.Cleanup(func() { _ = db.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			producer, err := workhub.NewRebacTupleJobClientWithDB(db)
			require.NoError(t, err)
			r := rebac.Relationship{Subject: rebac.UserSubject("ordered"), Relation: rebac.RelationReader, Object: rebac.RepositoryObject(1)}
			other := r
			other.Subject = rebac.UserSubject("independent")
			if wildcard {
				r = rebac.Relationship{Subject: rebac.PublicUserWildcard(), Relation: rebac.RelationPublic, Object: rebac.KnowledgeBaseObject(1)}
				other = r
				other.Object = rebac.KnowledgeBaseObject(2)
			}
			tx, err := db.BunDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			require.NoError(t, producer.InsertRebacTupleJobsTx(ctx, tx.Tx, []types.RebacTuple{
				types.NewRebacTuple("order.write", types.TupleOperation{Relationships: []rebac.Relationship{r}}),
				types.NewRebacTuple("order.delete", types.TupleOperation{Delete: true, Relationships: []rebac.Relationship{r}}),
				types.NewRebacTuple("order.independent", types.TupleOperation{Delete: wildcard, Relationships: []rebac.Relationship{other}}),
			}))
			require.NoError(t, tx.Commit())
			pool, ok := db.GetPGXPool()
			require.True(t, ok)
			a := &orderedTupleAuthorizer{first: make(chan struct{}), release: make(chan struct{}), present: make(map[string]bool), orderedKey: workhub.RebacSequenceKey(r)}
			if wildcard {
				a.present[workhub.RebacSequenceKey(other)] = true
			}
			for i := 0; i < 2; i++ {
				client, err := NewRebacTupleWorkClient(ctx, pool.Config().ConnString(), NewRebacTupleWorker(a))
				require.NoError(t, err)
				require.NoError(t, client.Start(ctx))
				t.Cleanup(func() {
					stop, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer stopCancel()
					require.NoError(t, client.Stop(stop))
				})
			}
			select {
			case <-a.first:
			case <-ctx.Done():
				t.Fatal("grant did not start")
			}
			require.Eventually(t, func() bool {
				a.mu.Lock()
				defer a.mu.Unlock()
				return a.present[workhub.RebacSequenceKey(other)] != wildcard
			}, 10*time.Second, 50*time.Millisecond)
			var state string
			require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("state").Where("args->>'source' = ?", "order.delete").Scan(ctx, &state))
			require.Equal(t, "pending", state)
			close(a.release)
			require.Eventually(t, func() bool {
				n, err := db.BunDB.NewSelect().Table("river_job").Where("kind = ? AND state = 'completed'", workhub.RebacTupleQueue).Count(ctx)
				return err == nil && n == 3
			}, 15*time.Second, 50*time.Millisecond)
			a.mu.Lock()
			defer a.mu.Unlock()
			require.Equal(t, 2, a.attempts)
			require.False(t, a.deletedEarly)
			require.False(t, a.present[workhub.RebacSequenceKey(r)])
		})
	}
}

// TestRebacTupleRiverTerminalContinuation advances successors after cancellation or discard.
func TestRebacTupleRiverTerminalContinuation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancelled=%t", cancelled), func(t *testing.T) {
			db := tests.InitTransactionTestDBWithHooks()
			t.Cleanup(func() { _ = db.Close() })
			ctx := context.Background()
			producer, err := workhub.NewRebacTupleJobClientWithDB(db)
			require.NoError(t, err)
			tx, err := db.BunDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			r := rebac.Relationship{Subject: rebac.UserSubject("terminal"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)}
			require.NoError(t, producer.InsertRebacTupleJobsTx(ctx, tx.Tx, []types.RebacTuple{
				types.NewRebacTuple("terminal.first", types.TupleOperation{Relationships: []rebac.Relationship{r}}),
				types.NewRebacTuple("terminal.next", types.TupleOperation{Delete: true, Relationships: []rebac.Relationship{r}}),
			}))
			require.NoError(t, tx.Commit())
			var firstID int64
			require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("id").Where("args->>'source' = 'terminal.first'").Scan(ctx, &firstID))
			var continues bool
			require.NoError(t, db.BunDB.NewRaw("SELECT sequence_continue_on_cancelled AND sequence_continue_on_discarded FROM river_job WHERE id = ?", firstID).Scan(ctx, &continues))
			require.True(t, continues, "continuation policy must be stored with the job")
			if cancelled {
				jobs, err := workhub.NewJobClient(ctx, db.BunDB)
				require.NoError(t, err)
				tx, err := db.BunDB.BeginTx(ctx, nil)
				require.NoError(t, err)
				require.NoError(t, jobs.JobCancelTx(ctx, tx.Tx, firstID))
				require.NoError(t, tx.Commit())
			} else {
				_, err := db.BunDB.NewUpdate().Table("river_job").Set("max_attempts = 1").Where("id = ?", firstID).Exec(ctx)
				require.NoError(t, err)
			}
			pool, ok := db.GetPGXPool()
			require.True(t, ok)
			a := &recordingAuthorizer{failWrite: true}
			client, err := NewRebacTupleWorkClient(ctx, pool.Config().ConnString(), NewRebacTupleWorker(a))
			require.NoError(t, err)
			require.NoError(t, client.Start(ctx))
			t.Cleanup(func() {
				stop, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				require.NoError(t, client.Stop(stop))
			})
			require.Eventually(t, func() bool {
				var state string
				err := db.BunDB.NewSelect().Table("river_job").Column("state").Where("args->>'source' = 'terminal.next'").Scan(ctx, &state)
				return err == nil && state == "completed"
			}, 10*time.Second, 50*time.Millisecond)
			var state string
			require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("state").Where("id = ?", firstID).Scan(ctx, &state))
			wantState := "discarded"
			if cancelled {
				wantState = "cancelled"
			}
			require.Equal(t, wantState, state, "continuation must preserve the failed job's outcome")
			var resolved bool
			require.NoError(t, db.BunDB.NewRaw("SELECT sequence_resolved_at IS NOT NULL FROM river_job WHERE id = ?", firstID).Scan(ctx, &resolved))
			require.True(t, resolved)
			var mode string
			require.NoError(t, db.BunDB.NewRaw("SELECT mode FROM river_sequence_group WHERE key = (SELECT sequence_key FROM river_job WHERE id = ?)", firstID).Scan(ctx, &mode))
			require.Equal(t, "ordered", mode)
		})
	}
}
