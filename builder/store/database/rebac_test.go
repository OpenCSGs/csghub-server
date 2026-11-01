package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// failingTupleJobs injects failure after River has inserted on the business tx.
type failingTupleJobs struct {
	delegate database.RebacTupleJobClient
	check    func()
	// source optionally limits failure injection to a specific permission event.
	source string
}

// InsertRebacTupleJobsTx simulates a later failure while retaining real SQL behavior.
func (f failingTupleJobs) InsertRebacTupleJobsTx(ctx context.Context, tx *sql.Tx, inputs []types.RebacTuple) error {
	if err := f.delegate.InsertRebacTupleJobsTx(ctx, tx, inputs); err != nil {
		return err
	}
	matches := f.source == ""
	for _, input := range inputs {
		matches = matches || input.Source == f.source
	}
	if !matches {
		return nil
	}
	if f.check != nil {
		f.check()
	}
	return errors.New("injected failure after enqueue")
}

// tupleJobCount reads from the root pool, not an in-flight transaction connection.
func tupleJobCount(t *testing.T, db *database.DB) int {
	t.Helper()
	n, err := db.BunDB.NewSelect().Table("river_job").Where("kind = ?", workhub.RebacTupleQueue).Count(context.Background())
	require.NoError(t, err)
	return n
}

// TestRebacTupleTransactionAtomicity verifies real River visibility and rollback.
func TestRebacTupleTransactionAtomicity(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, producerErr)
	before := tupleJobCount(t, db)

	store := database.NewUserStoreWithDBAndJobClients(db, nil, failingTupleJobs{delegate: producer, check: func() { require.Equal(t, before, tupleJobCount(t, db)) }})
	user := &database.User{Username: "tuple-rollback", UUID: "tuple-user"}
	ns := &database.Namespace{Path: user.Username, UUID: "tuple-namespace"}
	require.ErrorContains(t, store.Create(ctx, user, ns), "injected failure")
	exists, err := store.IsExist(ctx, user.Username)
	require.NoError(t, err)
	require.False(t, exists)
	require.Equal(t, before, tupleJobCount(t, db))
	store = database.NewUserStoreWithDBAndJobClients(db, nil, producer)
	user.ID, ns.ID = 0, 0
	require.NoError(t, store.Create(ctx, user, ns))
	require.Equal(t, before+1, tupleJobCount(t, db))
	var payload workhub.RebacTupleArgs
	var encoded []byte
	require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("args").Where("kind = ?", workhub.RebacTupleQueue).OrderExpr("id DESC").Limit(1).Scan(ctx, &encoded))
	require.NoError(t, json.Unmarshal(encoded, &payload))
	require.Len(t, payload.Operations, 1)
	require.Len(t, payload.Operations[0].Relationships, 2)
}

// TestRebacTupleStoreDependencyInjection verifies Store-local producers and missing-wiring rollback.
func TestRebacTupleStoreDependencyInjection(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	before := tupleJobCount(t, db)
	reader := database.NewUserStoreWithDB(db)
	failing := database.NewUserStoreWithDBAndJobClients(db, nil, failingTupleJobs{delegate: producer})
	successful := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
	create := func(store database.UserStore, name string) error {
		return store.Create(ctx, &database.User{Username: name, UUID: name}, &database.Namespace{Path: name, UUID: name + "-ns"})
	}
	require.ErrorContains(t, create(reader, "missing-producer"), "transactional tuple job client is required")
	require.ErrorContains(t, create(failing, "failed-producer"), "injected failure")
	require.Equal(t, before, tupleJobCount(t, db))
	require.NoError(t, create(successful, "successful-producer"))
	require.ErrorContains(t, create(failing, "still-failed-producer"), "injected failure")
	require.Equal(t, before+1, tupleJobCount(t, db))
	for _, name := range []string{"missing-producer", "failed-producer", "still-failed-producer"} {
		exists, err := reader.IsExist(ctx, name)
		require.NoError(t, err)
		require.False(t, exists)
		var namespaces int
		namespaces, err = db.BunDB.NewSelect().Model((*database.Namespace)(nil)).Where("path = ?", name).Count(ctx)
		require.NoError(t, err)
		require.Zero(t, namespaces)
	}
	exists, err := reader.IsExist(ctx, "successful-producer")
	require.NoError(t, err)
	require.True(t, exists)
}

// TestKnowledgeBaseTupleRollback keeps an existing public flag when enqueue fails.
func TestKnowledgeBaseTupleRollback(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, producerErr)
	store := database.NewAgentKnowledgeBaseStoreWithRebacTupleJobClient(db, producer)
	kb, err := store.Create(ctx, &database.AgentKnowledgeBase{UserUUID: "tuple-user", NsUUID: "tuple-namespace", Name: "tuple-kb", ContentID: "tuple-content", Public: true})
	require.NoError(t, err)
	before := tupleJobCount(t, db)
	store = database.NewAgentKnowledgeBaseStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
	kb.Public = false
	require.ErrorContains(t, store.Update(ctx, kb, &kb.Public), "injected failure")
	var public bool
	require.NoError(t, db.BunDB.NewSelect().Table("agent_knowledge_bases").Column("public").Where("id = ?", kb.ID).Scan(ctx, &public))
	require.True(t, public)
	require.Equal(t, before, tupleJobCount(t, db))
	require.ErrorContains(t, store.Delete(ctx, kb.ID), "injected failure")
	require.Equal(t, before, tupleJobCount(t, db))
}

// TestKnowledgeBaseMetadataUpdatePreservesVisibility replays a stale edit after a visibility change.
func TestKnowledgeBaseMetadataUpdatePreservesVisibility(t *testing.T) {
	for _, public := range []bool{false, true} {
		t.Run(fmt.Sprintf("public=%t", public), func(t *testing.T) {
			db := tests.InitTransactionTestDB()
			t.Cleanup(func() { _ = db.Close() })
			ctx := context.Background()
			producer, err := workhub.NewRebacTupleJobClientWithDB(db)
			require.NoError(t, err)
			store := database.NewAgentKnowledgeBaseStoreWithRebacTupleJobClient(db, producer)
			kb, err := store.Create(ctx, &database.AgentKnowledgeBase{
				UserUUID: "tuple-user", NsUUID: "tuple-namespace", Name: "tuple-kb",
				ContentID: "tuple-content", Public: !public,
			})
			require.NoError(t, err)

			// Request A reads the old row before request B commits its visibility change.
			stale, err := store.FindByID(ctx, kb.ID)
			require.NoError(t, err)
			before := tupleJobCount(t, db)
			require.NoError(t, store.Update(ctx, kb, &public))
			require.Equal(t, before+1, tupleJobCount(t, db))
			event := latestRebacTuple(t, db, "knowledge_base")
			require.Len(t, event.Operations, 1)
			require.Equal(t, !public, event.Operations[0].Delete)
			require.Equal(t, []rebac.Relationship{{
				Subject: rebac.PublicUserWildcard(), Relation: rebac.RelationPublic, Object: rebac.KnowledgeBaseObject(kb.ID),
			}}, event.Operations[0].Relationships)

			// Request A finishes last; a metadata edit needs no permission producer.
			metadataStore := database.NewAgentKnowledgeBaseStoreWithDB(db)
			stale.Description = "updated description"
			require.NoError(t, metadataStore.Update(ctx, stale, nil))
			current, err := store.FindByID(ctx, kb.ID)
			require.NoError(t, err)
			require.Equal(t, "updated description", current.Description)
			require.Equal(t, public, current.Public)
			require.Equal(t, before+1, tupleJobCount(t, db))
		})
	}
}

// latestRebacTuple reconstructs the last event's subject chunks from its SQL transaction.
func latestRebacTuple(t *testing.T, db *database.DB, sources ...string) types.RebacTuple {
	t.Helper()
	var payloads [][]byte
	source := ""
	if len(sources) > 0 {
		source = sources[0]
	}
	require.NoError(t, db.BunDB.NewRaw(`SELECT args FROM river_job
		WHERE kind = ? AND xmin = (SELECT xmin FROM river_job WHERE kind = ? AND (? = '' OR args->>'source' = ?) ORDER BY id DESC LIMIT 1)
		AND args->>'source' = (SELECT args->>'source' FROM river_job WHERE kind = ? AND (? = '' OR args->>'source' = ?) ORDER BY id DESC LIMIT 1)
		ORDER BY id`, workhub.RebacTupleQueue, workhub.RebacTupleQueue, source, source, workhub.RebacTupleQueue, source, source).Scan(context.Background(), &payloads))
	var result types.RebacTuple
	for _, payload := range payloads {
		var event types.RebacTuple
		require.NoError(t, json.Unmarshal(payload, &event))
		result.Version, result.Source, result.ModelID = event.Version, event.Source, event.ModelID
		for _, operation := range event.Operations {
			if len(result.Operations) > 0 && result.Operations[len(result.Operations)-1].Delete == operation.Delete {
				last := &result.Operations[len(result.Operations)-1]
				last.Relationships = append(last.Relationships, operation.Relationships...)
			} else {
				result.Operations = append(result.Operations, operation)
			}
		}
	}
	return result
}

// TestRebacTupleStoreSnapshots covers ownership, role deltas and transaction rollback.
func TestRebacTupleStoreSnapshots(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, producerErr)
	users := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
	owner := &database.User{Username: "tuple-owner", UUID: "tuple-owner"}
	ns := &database.Namespace{Path: owner.Username, UUID: "tuple-owner-ns"}
	require.NoError(t, users.Create(ctx, owner, ns))
	target := &database.User{Username: "tuple-target", UUID: "tuple-target"}
	require.NoError(t, users.Create(ctx, target, &database.Namespace{Path: target.Username, UUID: "tuple-target-ns"}))
	setOrgStoreTestDB(t, db)
	orgs := database.NewOrgStoreWithJobClients(false, nil, nil, producer)
	org := &database.Organization{Name: "tuple-org", UUID: uuid.New(), UserID: owner.ID}
	orgNS := &database.Namespace{Path: "tuple-org", UUID: "tuple-org-ns"}
	t.Run("organization creation is atomic", func(t *testing.T) {
		before := tupleJobCount(t, db)
		orgs = database.NewOrgStoreWithJobClients(false, nil, nil, failingTupleJobs{delegate: producer})
		require.ErrorContains(t, orgs.CreateWithRelations(ctx, org, orgNS, nil), "injected failure")
		exists, err := orgs.Exists(ctx, org.Name)
		require.NoError(t, err)
		require.False(t, exists)
		require.Equal(t, before, tupleJobCount(t, db))
		orgs = database.NewOrgStoreWithJobClients(false, nil, nil, producer)
		org.ID = 0
		orgNS.ID = 0
		require.NoError(t, orgs.CreateWithRelations(ctx, org, orgNS, nil))
		payload := latestRebacTuple(t, db)
		require.ElementsMatch(t, []rebac.Relationship{
			{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, org.UUID.String()), Relation: rebac.RelationOrganization, Object: rebac.NamespaceObject(orgNS.UUID)},
			{Subject: rebac.UserSubject(owner.UUID), Relation: rebac.RelationAdmin, Object: rebac.OrganizationObject(org.UUID.String())},
		}, payload.Operations[0].Relationships)
	})
	t.Run("member changes repair obsolete roles", func(t *testing.T) {
		store := database.NewMemberStoreWithRebacTupleJobClient(db, producer)
		require.NoError(t, store.Add(ctx, org.ID, target.ID, string(types.UserRead)))
		before := tupleJobCount(t, db)
		store = database.NewMemberStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
		require.ErrorContains(t, store.Update(ctx, org.ID, target.ID, string(types.UserWrite)), "injected failure")
		member, err := store.Find(ctx, org.ID, target.ID)
		require.NoError(t, err)
		require.Equal(t, string(types.UserRead), member.Role)
		require.Equal(t, before, tupleJobCount(t, db))
		store = database.NewMemberStoreWithRebacTupleJobClient(db, producer)
		require.NoError(t, store.Update(ctx, org.ID, target.ID, string(types.UserWrite)))
		payload := latestRebacTuple(t, db)
		require.Len(t, payload.Operations, 2)
		require.True(t, payload.Operations[0].Delete)
		require.Len(t, payload.Operations[0].Relationships, 2)
		require.Equal(t, rebac.RelationWriter, payload.Operations[1].Relationships[0].Relation)
		require.NoError(t, store.Delete(ctx, org.ID, target.ID))
		payload = latestRebacTuple(t, db)
		require.Len(t, payload.Operations, 1)
		require.True(t, payload.Operations[0].Delete)
		require.Len(t, payload.Operations[0].Relationships, 3)
		require.NoError(t, store.ReconcileTuples(ctx, org.ID, target.ID))
		require.Equal(t, payload.Operations, latestRebacTuple(t, db).Operations)
	})
	t.Run("repository transfer snapshots both owners", func(t *testing.T) {
		store := database.NewRepoStoreWithDBAndJobClients(db, nil, producer)
		repo, err := store.CreateRepo(ctx, database.Repository{Path: "tuple-owner/model", Name: "model", RepositoryType: types.ModelRepo, UserID: owner.ID})
		require.NoError(t, err)
		require.Equal(t, rebac.UserSubject(owner.UUID), latestRebacTuple(t, db).Operations[0].Relationships[0].Subject)
		updated := *repo
		updated.Path = "tuple-org/model"
		updated.TupleNamespacePath = "tuple-org"
		updated.OrgInheritBlocked = true
		before := tupleJobCount(t, db)
		store = database.NewRepoStoreWithDBAndJobClients(db, nil, failingTupleJobs{delegate: producer})
		_, err = store.UpdateRepo(ctx, updated)
		require.ErrorContains(t, err, "injected failure")
		var path string
		require.NoError(t, db.BunDB.NewSelect().Table("repositories").Column("path").Where("id = ?", repo.ID).Scan(ctx, &path))
		require.Equal(t, repo.Path, path)
		require.Equal(t, before, tupleJobCount(t, db))
		store = database.NewRepoStoreWithDBAndJobClients(db, nil, producer)
		_, err = store.UpdateRepo(ctx, updated)
		require.NoError(t, err)
		payload := latestRebacTuple(t, db)
		require.Len(t, payload.Operations, 2)
		require.True(t, payload.Operations[0].Delete)
		require.Equal(t, rebac.UserSubject(owner.UUID), payload.Operations[0].Relationships[0].Subject)
		require.Equal(t, rebac.RelationOrganizationDirect, payload.Operations[1].Relationships[0].Relation)
		require.Equal(t, org.UUID.String(), payload.Operations[1].Relationships[0].Subject.ID)
		updated.OrgInheritBlocked = false
		_, err = store.UpdateRepo(ctx, updated)
		require.NoError(t, err)
		payload = latestRebacTuple(t, db)
		require.Len(t, payload.Operations[0].Relationships, 2)
		require.Equal(t, rebac.RelationOrganization, payload.Operations[1].Relationships[0].Relation)
		before = tupleJobCount(t, db)
		updated.TupleNamespacePath = ""
		updated.Description = "metadata only"
		_, err = store.UpdateRepo(ctx, updated)
		require.NoError(t, err)
		require.Equal(t, before, tupleJobCount(t, db))
	})
	t.Run("temporary path uses real namespace", func(t *testing.T) {
		store := database.NewRepoStoreWithDBAndJobClients(db, nil, producer)
		repo, err := store.CreateRepo(ctx, database.Repository{Path: "12345678/1234-1234-1234-123456789abc", GitPath: "models_temp/repo", Name: "temporary", RepositoryType: types.ModelRepo, UserID: owner.ID, TupleNamespacePath: "tuple-owner"})
		require.NoError(t, err)
		require.Equal(t, rebac.UserSubject(owner.UUID), latestRebacTuple(t, db).Operations[0].Relationships[0].Subject)
		repo.Path = "tuple-owner/temporary"
		_, err = store.UpdateRepo(ctx, *repo)
		require.NoError(t, err)
	})
	t.Run("MCP cleanup saves deleted identity", func(t *testing.T) {
		store := database.NewMCPServerStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
		repo := &database.Repository{Path: "tuple-owner/mcp-space", GitPath: "spaces_tuple-owner/mcp-space", Name: "mcp-space", RepositoryType: types.SpaceRepo, UserID: owner.ID}
		space := &database.Space{}
		beforeCreate := tupleJobCount(t, db)
		failedTuple, err := store.CreateSpaceAndRepoForDeploy(ctx, repo, space)
		require.ErrorContains(t, err, "injected failure")
		require.Equal(t, rebac.Relationship{}, failedTuple)
		require.Equal(t, beforeCreate, tupleJobCount(t, db))
		exists, err := db.BunDB.NewSelect().Model((*database.Repository)(nil)).Where("id = ?", repo.ID).Exists(ctx)
		require.NoError(t, err)
		require.False(t, exists)
		repo.ID, space.ID = 0, 0
		store = database.NewMCPServerStoreWithRebacTupleJobClient(db, producer)
		ownerTuple, err := store.CreateSpaceAndRepoForDeploy(ctx, repo, space)
		require.NoError(t, err)
		before := tupleJobCount(t, db)
		store = database.NewMCPServerStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
		require.ErrorContains(t, store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID, ownerTuple), "injected failure")
		exists, err = db.BunDB.NewSelect().Model((*database.Space)(nil)).Where("id = ?", space.ID).Exists(ctx)
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, before, tupleJobCount(t, db))
		store = database.NewMCPServerStoreWithRebacTupleJobClient(db, producer)
		require.NoError(t, store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID, ownerTuple))
		payload := latestRebacTuple(t, db)
		require.True(t, payload.Operations[0].Delete)
		require.Equal(t, rebac.RepositoryObject(repo.ID), payload.Operations[0].Relationships[0].Object)
	})
}

// TestRebacTupleMetadataUpdatePreservesInheritance verifies a stale metadata snapshot
// cannot undo either inheritance transition or change its durable tuple projection.
func TestRebacTupleMetadataUpdatePreservesInheritance(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	_, err = database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer).CreateRoot(ctx, database.CreateRootOrganizationInput{
		Organization: org, Namespace: &database.Namespace{Path: org.Name, UUID: uuid.NewString()},
	})
	require.NoError(t, err)
	store := database.NewRepoStoreWithDBAndJobClients(db, nil, producer)
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("initially_blocked=%t", blocked), func(t *testing.T) {
			name := uuid.NewString()
			repo, err := store.CreateRepo(ctx, database.Repository{
				Path: org.Name + "/" + name, Name: name, GitPath: uuid.NewString(), RepositoryType: types.ModelRepo, OrgInheritBlocked: blocked,
			})
			require.NoError(t, err)
			stale := *repo
			require.Empty(t, stale.TupleNamespacePath)
			changed := *repo
			changed.OrgInheritBlocked = !blocked
			changed.TupleNamespacePath = org.Name
			before := tupleJobCount(t, db)
			_, err = store.UpdateRepo(ctx, changed)
			require.NoError(t, err)
			require.Equal(t, before+1, tupleJobCount(t, db), "explicit inheritance changes must enqueue a tuple event")
			stale.Description = "updated metadata"
			result, err := store.UpdateRepo(ctx, stale)
			require.NoError(t, err)
			require.Equal(t, changed.OrgInheritBlocked, result.OrgInheritBlocked, "response must reflect the current inheritance setting")
			var stored database.Repository
			require.NoError(t, db.BunDB.NewSelect().Model(&stored).Where("id = ?", repo.ID).Scan(ctx))
			require.Equal(t, stale.Description, stored.Description)
			require.Equal(t, changed.OrgInheritBlocked, stored.OrgInheritBlocked)
			require.Equal(t, before+1, tupleJobCount(t, db), "metadata updates must not enqueue another tuple event")
			relation := rebac.RelationOrganization
			if changed.OrgInheritBlocked {
				relation = rebac.RelationOrganizationDirect
			}
			assertRepositoryTupleProjection(t, db, repo.ID, []rebac.Relationship{{
				Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, org.UUID.String()), Relation: relation, Object: rebac.RepositoryObject(repo.ID),
			}})
		})
	}
}

// TestRebacTuplePermissionNoOp preserves metadata, timestamps and the queue for repeated settings.
func TestRebacTuplePermissionNoOp(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
	require.NoError(t, database.NewUserStoreWithDBAndJobClients(db, nil, producer).Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
	org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	_, err = database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer).CreateRoot(ctx, database.CreateRootOrganizationInput{
		Organization: org, Namespace: &database.Namespace{Path: org.Name, UUID: uuid.NewString()},
	})
	require.NoError(t, err)
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("inheritance=%t", blocked), func(t *testing.T) {
			store := database.NewRepoStoreWithDBAndJobClients(db, nil, producer)
			name := uuid.NewString()
			repo, err := store.CreateRepo(ctx, database.Repository{Path: org.Name + "/" + name, Name: name, GitPath: uuid.NewString(), RepositoryType: types.ModelRepo, OrgInheritBlocked: blocked})
			require.NoError(t, err)
			stale := *repo
			stale.TupleNamespacePath = org.Name
			// Simulate metadata committed after the Component read its repository snapshot.
			_, err = db.BunDB.NewUpdate().Model((*database.Repository)(nil)).Set("description = ?", "new metadata").Where("id = ?", repo.ID).Exec(ctx)
			require.NoError(t, err)
			var before database.Repository
			require.NoError(t, db.BunDB.NewSelect().Model(&before).Where("id = ?", repo.ID).Scan(ctx))
			count := tupleJobCount(t, db)
			for _, jobs := range []database.RebacTupleJobClient{nil, failingTupleJobs{delegate: producer}} {
				result, err := database.NewRepoStoreWithDBAndJobClients(db, nil, jobs).UpdateRepo(ctx, stale)
				require.NoError(t, err, "no-op must not require a working tuple producer")
				require.Equal(t, before, *result, "no-op must return the latest metadata")
				var after database.Repository
				require.NoError(t, db.BunDB.NewSelect().Model(&after).Where("id = ?", repo.ID).Scan(ctx))
				require.Equal(t, before, after)
				require.Equal(t, count, tupleJobCount(t, db))
			}
		})
	}
	repo := &database.Repository{Path: org.Name + "/grants", GitPath: uuid.NewString(), RepositoryType: types.ModelRepo}
	_, err = db.BunDB.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	for _, subjectType := range []types.RepoAuthSubjectType{types.RepoAuthSubjectUser, types.RepoAuthSubjectOrganization} {
		for _, role := range []types.UserRole{types.UserRead, types.UserWrite} {
			t.Run(fmt.Sprintf("role=%s/%s", subjectType, role), func(t *testing.T) {
				id, subjectUUID := user.ID, user.UUID
				if subjectType == types.RepoAuthSubjectOrganization {
					id, subjectUUID = org.ID, org.UUID.String()
				}
				grants := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
				require.NoError(t, grants.Create(ctx, &database.RepositoryAuthorization{RepositoryID: repo.ID, SubjectType: subjectType, SubjectID: id, SubjectUUID: subjectUUID, Role: role}))
				// A fixed old timestamp makes an accidental UPDATE observable without sleeping.
				_, err := db.BunDB.NewUpdate().Model((*database.RepositoryAuthorization)(nil)).Set("updated_at = ?", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)).Where("repository_id = ? AND subject_type = ? AND subject_id = ?", repo.ID, subjectType, id).Exec(ctx)
				require.NoError(t, err)
				before, err := grants.Find(ctx, repo.ID, id, subjectType)
				require.NoError(t, err)
				count := tupleJobCount(t, db)
				for _, jobs := range []database.RebacTupleJobClient{nil, failingTupleJobs{delegate: producer}} {
					store := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, jobs)
					require.NoError(t, store.UpdateRole(ctx, repo.ID, id, subjectType, role))
					after, err := store.Find(ctx, repo.ID, id, subjectType)
					require.NoError(t, err)
					require.Equal(t, before, after)
					require.Equal(t, count, tupleJobCount(t, db))
				}
			})
		}
	}
}

// TestRebacTuplePermissionNoOpConcurrency rechecks settings after real lock waits.
// Committed changes make the waiter a no-op; rolled-back changes must still be applied.
func TestRebacTuplePermissionNoOpConcurrency(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	for _, operation := range []string{"inheritance", "role"} {
		for _, rollback := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/rollback=%t", operation, rollback), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				t.Cleanup(cancel)
				user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
				require.NoError(t, database.NewUserStoreWithDBAndJobClients(db, nil, producer).Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
				repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, producer).CreateRepo(ctx, database.Repository{Path: user.Username + "/repo", Name: "repo", GitPath: uuid.NewString(), UserID: user.ID, RepositoryType: types.ModelRepo})
				require.NoError(t, err)
				grants := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
				require.NoError(t, grants.Create(ctx, &database.RepositoryAuthorization{RepositoryID: repo.ID, SubjectType: types.RepoAuthSubjectUser, SubjectID: user.ID, SubjectUUID: user.UUID, Role: types.UserRead}))
				firstJobs := producer
				if rollback {
					firstJobs = failingTupleJobs{delegate: producer}
				}
				apply := func(jobs database.RebacTupleJobClient) func(context.Context) error {
					return func(opCtx context.Context) error {
						if operation == "role" {
							return database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, jobs).UpdateRole(opCtx, repo.ID, user.ID, types.RepoAuthSubjectUser, types.UserWrite)
						}
						input := *repo
						input.TupleNamespacePath, input.OrgInheritBlocked = user.Username, true
						_, err := database.NewRepoStoreWithDBAndJobClients(db, nil, jobs).UpdateRepo(opCtx, input)
						return err
					}
				}
				count := tupleJobCount(t, db)
				hook := &rebacGrantPauseHook{query: `FROM "repositories"`, querySuffix: "FOR UPDATE", reached: make(chan struct{}), resume: make(chan struct{})}
				if operation == "inheritance" {
					// pg_stat_activity may truncate the full repository SELECT before its FROM clause.
					hook.waitQuery = `SELECT "repository".%`
				}
				firstErr, secondErr := hook.run(t, ctx, db, user.ID, apply(firstJobs), apply(producer))
				if rollback {
					require.ErrorContains(t, firstErr, "injected failure")
				} else {
					require.NoError(t, firstErr)
				}
				require.NoError(t, secondErr, "the waiter must finish without a deadlock")
				require.Equal(t, count+1, tupleJobCount(t, db))
				if operation == "role" {
					grant, err := grants.Find(ctx, repo.ID, user.ID, types.RepoAuthSubjectUser)
					require.NoError(t, err)
					require.Equal(t, types.UserWrite, grant.Role)
				} else {
					var stored database.Repository
					require.NoError(t, db.BunDB.NewSelect().Model(&stored).Where("id = ?", repo.ID).Scan(ctx))
					require.True(t, stored.OrgInheritBlocked)
				}
			})
		}
	}
}

// TestMCPDeployTupleCleanupSnapshot proves cleanup never resolves the namespace or subject again.
func TestMCPDeployTupleCleanupSnapshot(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	_, err = database.NewMCPServerStoreWithRebacTupleJobClient(db, producer).CreateSpaceAndRepoForDeploy(ctx, nil, &database.Space{})
	require.ErrorContains(t, err, "repository is required")
	for _, tc := range []struct {
		name                                  string
		organization, direct, removeNamespace bool
	}{
		{name: "user"},
		{name: "organization", organization: true},
		{name: "organization direct", organization: true, direct: true},
		{name: "missing namespace", removeNamespace: true},
		{name: "missing organization namespace", organization: true, removeNamespace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
			ns := &database.Namespace{Path: user.Username, UUID: uuid.NewString()}
			users := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
			require.NoError(t, users.Create(ctx, user, ns))
			subject := rebac.UserSubject(user.UUID)
			var org database.Organization
			if tc.organization {
				ns = &database.Namespace{Path: uuid.NewString(), UUID: uuid.NewString(), NamespaceType: database.OrgNamespace}
				_, err := db.Core.NewInsert().Model(ns).Exec(ctx)
				require.NoError(t, err)
				org = database.Organization{Name: ns.Path, UUID: uuid.New(), NamespaceID: ns.ID, UserID: user.ID}
				_, err = db.Core.NewInsert().Model(&org).Exec(ctx)
				require.NoError(t, err)
				subject = rebac.NewSubject(rebac.ObjectTypeOrganization, org.UUID.String())
			}
			repo := &database.Repository{Path: ns.Path + "/mcp-space", Name: "mcp-space", UserID: user.ID, RepositoryType: types.SpaceRepo, OrgInheritBlocked: tc.direct}
			space := &database.Space{}
			store := database.NewMCPServerStoreWithRebacTupleJobClient(db, producer)
			owner, err := store.CreateSpaceAndRepoForDeploy(ctx, repo, space)
			require.NoError(t, err)
			require.Equal(t, subject, owner.Subject)
			require.Equal(t, []rebac.Relationship{owner}, latestRebacTuple(t, db).Operations[0].Relationships)
			if tc.direct {
				require.Equal(t, rebac.RelationOrganizationDirect, owner.Relation)
			}
			query := db.Core.NewDelete().Model(ns).WherePK()
			if tc.removeNamespace {
				query.ForceDelete()
			}
			_, err = query.Exec(ctx)
			require.NoError(t, err)
			if tc.organization {
				_, err = db.Core.NewDelete().Model(&org).WherePK().Exec(ctx)
			} else {
				_, err = db.Core.NewDelete().Model(user).WherePK().Exec(ctx)
			}
			require.NoError(t, err)
			_, err = db.Core.NewDelete().Model(repo).WherePK().Exec(ctx)
			require.NoError(t, err)
			before := tupleJobCount(t, db)
			wrongOwner := owner
			wrongOwner.Object = rebac.RepositoryObject(repo.ID + 1)
			require.ErrorContains(t, store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID, wrongOwner), "does not match")
			store = database.NewMCPServerStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
			require.ErrorContains(t, store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID, owner), "injected failure")
			require.Equal(t, before, tupleJobCount(t, db))
			exists, err := db.Core.NewSelect().Model((*database.Space)(nil)).Where("id = ?", space.ID).Exists(ctx)
			require.NoError(t, err)
			require.True(t, exists)
			exists, err = db.Core.NewSelect().Model((*database.Repository)(nil)).WhereAllWithDeleted().Where("id = ?", repo.ID).Exists(ctx)
			require.NoError(t, err)
			require.True(t, exists)
			store = database.NewMCPServerStoreWithRebacTupleJobClient(db, producer)
			require.NoError(t, store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID, owner))
			require.Equal(t, before+1, tupleJobCount(t, db))
			event := latestRebacTuple(t, db)
			require.Len(t, event.Operations, 1)
			require.True(t, event.Operations[0].Delete)
			expected := []rebac.Relationship{owner}
			if tc.organization {
				expected = []rebac.Relationship{
					{Subject: subject, Relation: rebac.RelationOrganization, Object: rebac.RepositoryObject(repo.ID)},
					{Subject: subject, Relation: rebac.RelationOrganizationDirect, Object: rebac.RepositoryObject(repo.ID)},
				}
			}
			require.ElementsMatch(t, expected, event.Operations[0].Relationships)
			for _, table := range []string{"spaces", "repositories"} {
				id := repo.ID
				if table == "spaces" {
					id = space.ID
				}
				n, err := db.Core.NewSelect().Table(table).Where("id = ?", id).Count(ctx)
				require.NoError(t, err)
				require.Zero(t, n)
			}
		})
	}
}

// TestMCPDeployTupleCleanupAfterInheritanceChange replays both inheritance transitions
// followed by compensation and verifies no organization owner relationship remains.
func TestMCPDeployTupleCleanupAfterInheritanceChange(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	_, err = database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer).CreateRoot(ctx, database.CreateRootOrganizationInput{
		Organization: org, Namespace: &database.Namespace{Path: org.Name, UUID: uuid.NewString()},
	})
	require.NoError(t, err)
	store := database.NewMCPServerStoreWithRebacTupleJobClient(db, producer)
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("initially_blocked=%t", blocked), func(t *testing.T) {
			name := uuid.NewString()
			repo := &database.Repository{Path: org.Name + "/" + name, Name: name, GitPath: uuid.NewString(), RepositoryType: types.SpaceRepo, OrgInheritBlocked: blocked}
			space := &database.Space{}
			owner, err := store.CreateSpaceAndRepoForDeploy(ctx, repo, space)
			require.NoError(t, err)
			updated := *repo
			updated.OrgInheritBlocked = !blocked
			updated.TupleNamespacePath = org.Name
			_, err = database.NewRepoStoreWithDBAndJobClients(db, nil, producer).UpdateRepo(ctx, updated)
			require.NoError(t, err)
			require.NoError(t, store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID, owner))
			assertRepositoryTupleProjection(t, db, repo.ID, nil)
		})
	}
}

// TestRebacTupleGrantSnapshot keeps the persisted UUID and role when subjects change later.
func TestRebacTupleGrantSnapshot(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, producerErr)
	repo := &database.Repository{Path: "grant-owner/repo", GitPath: "models_grant-owner/repo", RepositoryType: types.ModelRepo}
	_, err := db.BunDB.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	store := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
	// Grant writes require an active subject after acquiring the deletion lock.
	_, err = db.BunDB.NewInsert().Model(&database.User{ID: 42, Username: "grant-subject", UUID: "active-subject"}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.BunDB.NewInsert().Model(&database.Organization{ID: 42, Name: "grant-org-subject", UUID: uuid.New()}).Exec(ctx)
	require.NoError(t, err)
	for _, subjectType := range []types.RepoAuthSubjectType{types.RepoAuthSubjectUser, types.RepoAuthSubjectOrganization} {
		t.Run(string(subjectType), func(t *testing.T) {
			grant := &database.RepositoryAuthorization{RepositoryID: repo.ID, SubjectType: subjectType, SubjectID: 42, SubjectUUID: "original-uuid", Role: types.UserRead}
			require.NoError(t, store.Create(ctx, grant))
			before := tupleJobCount(t, db)
			store = database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
			require.ErrorContains(t, store.UpdateRole(ctx, repo.ID, 42, subjectType, types.UserWrite), "injected failure")
			persisted, err := store.Find(ctx, repo.ID, 42, subjectType)
			require.NoError(t, err)
			require.Equal(t, types.UserRead, persisted.Role)
			require.Equal(t, before, tupleJobCount(t, db))
			store = database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
			grant.SubjectUUID = "replacement-uuid"
			grant.Role = types.UserWrite
			require.NoError(t, store.Create(ctx, grant))
			payload := latestRebacTuple(t, db)
			require.Len(t, payload.Operations, 2)
			old := payload.Operations[0].Relationships[0]
			next := payload.Operations[1].Relationships[0]
			require.True(t, payload.Operations[0].Delete)
			require.Equal(t, "original-uuid", old.Subject.ID)
			require.Equal(t, rebac.RelationReader, old.Relation)
			require.Equal(t, "replacement-uuid", next.Subject.ID)
			require.Equal(t, rebac.RelationWriter, next.Relation)
			if subjectType == types.RepoAuthSubjectOrganization {
				require.Equal(t, rebac.RelationMember, next.Subject.Relation)
			}
			require.NoError(t, store.UpdateRole(ctx, repo.ID, 42, subjectType, types.UserRead))
			persisted, err = store.Find(ctx, repo.ID, 42, subjectType)
			require.NoError(t, err)
			require.Equal(t, types.UserRead, persisted.Role)
			require.NoError(t, store.Delete(ctx, repo.ID, 42, subjectType))
			payload = latestRebacTuple(t, db)
			require.True(t, payload.Operations[0].Delete)
			require.Equal(t, "replacement-uuid", payload.Operations[0].Relationships[0].Subject.ID)
			before = tupleJobCount(t, db)
			require.NoError(t, store.Delete(ctx, repo.ID, 42, subjectType))
			require.Equal(t, before, tupleJobCount(t, db))
		})
	}
}

// TestRebacTupleHierarchySnapshots verifies root/child edges and current creator permissions.
func TestRebacTupleHierarchySnapshots(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	setOrgStoreTestDB(t, db)
	ctx := context.Background()
	producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, producerErr)
	owner := &database.User{Username: "hierarchy-owner", UUID: "hierarchy-owner"}
	require.NoError(t, database.NewUserStoreWithDBAndJobClients(db, nil, producer).Create(ctx, owner, &database.Namespace{Path: owner.Username, UUID: "hierarchy-owner-ns"}))
	store := database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer)
	root := &database.Organization{Name: "tuple-root", UUID: uuid.New(), UserID: owner.ID}
	rootNS := &database.Namespace{Path: root.Name, UUID: "tuple-root-ns"}
	_, err := store.CreateRoot(ctx, database.CreateRootOrganizationInput{Organization: root, Namespace: rootNS, CreatorUserID: owner.ID})
	require.NoError(t, err)
	require.Len(t, latestRebacTuple(t, db).Operations[0].Relationships, 2)
	child := &database.Organization{Name: "tuple-child", UUID: uuid.New()}
	childNS := &database.Namespace{Path: child.Name, UUID: "tuple-child-ns"}
	input := database.CreateOrganizationUnitInput{RootOrganizationID: root.ID, Organization: child, Namespace: childNS, MaxDepth: 8}
	before := tupleJobCount(t, db)
	store = database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, failingTupleJobs{delegate: producer})
	_, err = store.Create(ctx, input)
	require.ErrorContains(t, err, "injected failure")
	require.Equal(t, before, tupleJobCount(t, db))
	store = database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer)
	child.ID = 0
	childNS.ID = 0
	_, err = store.Create(ctx, input)
	require.NoError(t, err)
	payload := latestRebacTuple(t, db)
	require.ElementsMatch(t, []rebac.Relationship{
		{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, child.UUID.String()), Relation: rebac.RelationOrganization, Object: rebac.NamespaceObject(childNS.UUID)},
		{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, root.UUID.String()), Relation: rebac.RelationParent, Object: rebac.OrganizationObject(child.UUID.String())},
		{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, child.UUID.String()), Relation: rebac.RelationChild, Object: rebac.OrganizationObject(root.UUID.String())},
	}, payload.Operations[0].Relationships)
	members := database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, producer)
	target := &database.User{Username: "hierarchy-target", UUID: "hierarchy-target"}
	require.NoError(t, database.NewUserStoreWithDBAndJobClients(db, nil, producer).Create(ctx, target, &database.Namespace{Path: target.Username, UUID: "hierarchy-target-ns"}))
	add := database.AddOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: root.ID, UserUUIDs: []string{target.UUID}, Role: types.UserRead}
	_, err = members.UpsertMembers(ctx, add)
	require.NoError(t, err)
	require.Equal(t, rebac.RelationReader, latestRebacTuple(t, db).Operations[1].Relationships[0].Relation)
	before = tupleJobCount(t, db)
	members = database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer})
	require.ErrorContains(t, members.UpdateMemberRole(ctx, database.UpdateOrganizationMemberRoleInput{RootOrganizationID: root.ID, OrganizationID: root.ID, OrganizationUUID: root.UUID.String(), UserID: target.ID, Role: types.UserWrite}), "injected failure")
	role, err := members.FindRole(ctx, root.ID, target.ID)
	require.NoError(t, err)
	require.Equal(t, types.UserRead, role)
	require.Equal(t, before, tupleJobCount(t, db))
	members = database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, producer)
	_, err = members.RemoveMembers(ctx, database.RemoveOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: root.ID, UserUUIDs: []string{target.UUID}})
	require.NoError(t, err)
	require.True(t, latestRebacTuple(t, db).Operations[0].Delete)
	// Mirror repair uses current membership and must not re-grant a former creator.
	orgs := database.NewOrgStoreWithJobClients(true, nil, nil, producer)
	for _, removed := range []bool{false, true} {
		if removed {
			_, err = db.BunDB.NewDelete().Model((*database.Member)(nil)).Where("organization_id = ? AND user_id = ?", root.ID, owner.ID).ForceDelete().Exec(ctx)
		} else {
			_, err = db.BunDB.NewUpdate().Model((*database.Member)(nil)).Set("role = ?", types.UserRead).Where("organization_id = ? AND user_id = ?", root.ID, owner.ID).Exec(ctx)
		}
		require.NoError(t, err)
		require.NoError(t, orgs.ReconcileNamespaceTuples(ctx, root.ID))
		require.Len(t, latestRebacTuple(t, db).Operations[0].Relationships, 1)
	}
}

// withTupleNamespace completes repository fixtures without bypassing the real Store transaction.
// Existing namespaces retain their owner, including organization and shared-user fixtures.
func withTupleNamespace(t *testing.T, db *database.DB, repository database.Repository) database.Repository {
	t.Helper()
	path, name := repository.NamespaceAndName()
	require.NotEmpty(t, path, "repository fixture requires a namespace path")
	require.NotEmpty(t, name, "repository fixture requires namespace/name")
	seedTupleNamespace(t, db, path)
	return repository
}

// seedTupleNamespace supplies a namespace owner and reuses existing user identities.
func seedTupleNamespace(t *testing.T, db *database.DB, path string) {
	t.Helper()
	ctx := context.Background()
	exists, err := db.Core.NewSelect().Model((*database.Namespace)(nil)).Where("path = ?", path).Exists(ctx)
	require.NoError(t, err)
	if exists {
		return
	}
	user := &database.User{}
	err = db.Core.NewSelect().Model(user).Where("username = ?", path).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		user = &database.User{Username: path, UUID: uuid.NewString()}
		_, err = db.Core.NewInsert().Model(user).Exec(ctx)
	}
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.Namespace{Path: path, UUID: uuid.NewString(), UserID: user.ID, NamespaceType: database.UserNamespace}).Exec(ctx)
	require.NoError(t, err)
}

// seedTupleOrganization supplies the persisted identity required for membership events.
func seedTupleOrganization(t *testing.T, db *database.DB, id int64) {
	t.Helper()
	_, err := db.Core.NewInsert().Model(&database.Organization{ID: id, Name: uuid.NewString(), UUID: uuid.New()}).Exec(context.Background())
	require.NoError(t, err)
}

// TestRebacTupleUserDeletion verifies grant snapshots survive deletion and rollback together.
func TestRebacTupleUserDeletion(t *testing.T) {
	for _, soft := range []bool{false, true} {
		name := "hard"
		if soft {
			name = "soft"
		}
		t.Run(name, func(t *testing.T) {
			db := tests.InitTransactionTestDB()
			t.Cleanup(func() { _ = db.Close() })
			ctx := context.Background()
			producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
			require.NoError(t, producerErr)
			store := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
			user := &database.User{Username: "tuple-deleted-user", UUID: "tuple-deleted-user"}
			require.NoError(t, store.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: "deleted-user-ns"}))
			grant := &database.RepositoryAuthorization{RepositoryID: 9001, SubjectID: user.ID, SubjectUUID: "persisted-grant-uuid", SubjectType: types.RepoAuthSubjectUser, Role: types.UserWrite}
			_, err := db.BunDB.NewInsert().Model(grant).Exec(ctx)
			require.NoError(t, err)
			var repo *database.Repository
			if soft {
				repo = &database.Repository{Path: user.Username + "/repo", GitPath: "models_deleted/repo", UserID: user.ID, RepositoryType: types.ModelRepo}
				_, err = db.BunDB.NewInsert().Model(repo).Exec(ctx)
				require.NoError(t, err)
			}
			remove := func() error {
				if soft {
					return store.SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{Repository: true})
				}
				return store.DeleteUserAndRelations(ctx, *user, types.CloseAccountReq{})
			}
			before := tupleJobCount(t, db)
			store = database.NewUserStoreWithDBAndJobClients(db, nil, failingTupleJobs{delegate: producer})
			require.ErrorContains(t, remove(), "injected failure")
			exists, err := store.IsExist(ctx, user.Username)
			require.NoError(t, err)
			require.True(t, exists)
			exists, err = db.BunDB.NewSelect().Model((*database.RepositoryAuthorization)(nil)).Where("id = ?", grant.ID).Exists(ctx)
			require.NoError(t, err)
			require.True(t, exists)
			require.Equal(t, before, tupleJobCount(t, db))
			store = database.NewUserStoreWithDBAndJobClients(db, nil, producer)
			require.NoError(t, remove())
			expectedJobs := before + 2
			if soft {
				expectedJobs++
			}
			require.Equal(t, expectedJobs, tupleJobCount(t, db))
			exists, err = db.BunDB.NewSelect().Model((*database.RepositoryAuthorization)(nil)).Where("id = ?", grant.ID).Exists(ctx)
			require.NoError(t, err)
			require.False(t, exists)
			var encoded []byte
			require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("args").Where("args->>'source' = ?", "user.direct_grants.delete").Scan(ctx, &encoded))
			require.Contains(t, string(encoded), "persisted-grant-uuid")
			payload := latestRebacTuple(t, db)
			require.True(t, payload.Operations[0].Delete)
			if soft {
				require.Contains(t, payload.Operations[0].Relationships, rebac.Relationship{Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(repo.ID)})
			}
		})
	}
}

// rebacGrantPauseKey marks the single operation paused by a concurrency test.
type rebacGrantPauseKey struct{}

// rebacGrantPauseHook holds an operation after reading its authorization snapshot.
// The competing operation must wait before acquiring repository or grant row locks.
type rebacGrantPauseHook struct {
	// query and querySuffix optionally select another operation and lock mode.
	// waitQuery selects the competing SQL lock wait; empty means the subject lock.
	query       string
	querySuffix string
	waitQuery   string
	reached     chan struct{}
	resume      chan struct{}
	pauseOnce   sync.Once
	resumeOnce  sync.Once
}

// BeforeQuery preserves the operation context used to select the barrier owner.
func (*rebacGrantPauseHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery pauses only the marked query and respects cancellation.
func (h *rebacGrantPauseHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	query := h.query
	if query == "" {
		query = `FROM "repository_authorizations"`
	}
	if ctx.Value(rebacGrantPauseKey{}) != h || event.Err != nil ||
		!strings.Contains(event.Query, query) || !strings.HasSuffix(event.Query, h.querySuffix) {
		return
	}
	h.pauseOnce.Do(func() {
		close(h.reached)
		select {
		case <-h.resume:
		case <-ctx.Done():
		}
	})
}

// release resumes the paused transaction exactly once, including on test failure.
func (h *rebacGrantPauseHook) release() {
	h.resumeOnce.Do(func() { close(h.resume) })
}

// awaitRebacGrantResult bounds completion so a lock cycle cannot hang the test suite.
func awaitRebacGrantResult(t *testing.T, ctx context.Context, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		t.Fatal("concurrent grant/deletion operation did not finish: possible lock cycle", ctx.Err())
		return ctx.Err()
	}
}

// run observes a real PostgreSQL lock wait before allowing the first transaction to finish.
// Both operations are cancelled and joined on failure so a deadlock cannot leak goroutines.
func (h *rebacGrantPauseHook) run(t *testing.T, ctx context.Context, db *database.DB, userID int64, first, second func(context.Context) error) (error, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	var operations sync.WaitGroup
	defer func() {
		h.release()
		cancel()
		operations.Wait()
	}()
	db.BunDB.AddQueryHook(h)
	firstResult, secondResult := make(chan error, 1), make(chan error, 1)
	operations.Add(1)
	go func() {
		defer operations.Done()
		firstResult <- first(context.WithValue(ctx, rebacGrantPauseKey{}, h))
	}()
	select {
	case <-h.reached:
	case err := <-firstResult:
		t.Fatalf("first operation returned before the snapshot barrier: %v", err)
	case <-ctx.Done():
		t.Fatal("first operation did not reach the snapshot barrier", ctx.Err())
	}
	operations.Add(1)
	go func() {
		defer operations.Done()
		secondResult <- second(ctx)
	}()
	pattern := h.waitQuery
	if pattern == "" {
		pattern = fmt.Sprintf("%%rebac:user_repository_grants:%d'%%", userID)
	}
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.BunDB.NewRaw(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE ?)`, pattern).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond, "competitor must wait on the expected transaction lock")
	h.release()
	return awaitRebacGrantResult(t, ctx, firstResult), awaitRebacGrantResult(t, ctx, secondResult)
}

// assertRebacGrantProjection replays durable intents and compares the grant with SQL.
func assertRebacGrantProjection(t *testing.T, db *database.DB, repoID int64, userUUID string, role types.UserRole) {
	t.Helper()
	ctx := context.Background()
	var payloads [][]byte
	require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("args").Where("kind = ?", workhub.RebacTupleQueue).Order("id ASC").Scan(ctx, &payloads))
	present := make(map[rebac.Relationship]bool)
	for _, payload := range payloads {
		var event types.RebacTuple
		require.NoError(t, json.Unmarshal(payload, &event))
		for _, operation := range event.Operations {
			for _, relationship := range operation.Relationships {
				present[relationship] = !operation.Delete
			}
		}
	}
	for _, relation := range []rebac.Relation{rebac.RelationReader, rebac.RelationWriter} {
		expected := role == types.UserRead && relation == rebac.RelationReader || role == types.UserWrite && relation == rebac.RelationWriter
		r := rebac.Relationship{Subject: rebac.UserSubject(userUUID), Relation: relation, Object: rebac.RepositoryObject(repoID)}
		require.Equal(t, expected, present[r], "durable projection for %s", r.String())
	}
	var grants []database.RepositoryAuthorization
	require.NoError(t, db.BunDB.NewSelect().Model(&grants).Where("repository_id = ? AND subject_uuid = ?", repoID, userUUID).Scan(ctx))
	if role == "" {
		require.Empty(t, grants)
	} else {
		require.Len(t, grants, 1)
		require.Equal(t, role, grants[0].Role)
	}
}

// TestRebacTupleUserGrantConcurrency checks both lock orders and transaction rollback
// with real PostgreSQL waits. Owned-repository cases detect row/advisory lock inversion.
func TestRebacTupleUserGrantConcurrency(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	producer, producerErr := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, producerErr)
	for _, tc := range []struct {
		name         string
		soft         bool
		deleteFirst  bool
		ownedRepo    bool
		upsert       bool
		missingGrant bool
		rollback     bool
	}{
		{name: "hard/update first"},
		{name: "soft/update first", soft: true},
		{name: "hard/owned repository update first", ownedRepo: true},
		{name: "soft/owned repository update first", soft: true, ownedRepo: true},
		{name: "hard/delete first", deleteFirst: true},
		{name: "soft/delete first", soft: true, deleteFirst: true},
		{name: "hard/owned repository lock order", deleteFirst: true, ownedRepo: true},
		{name: "soft/owned repository lock order", soft: true, deleteFirst: true, ownedRepo: true},
		{name: "upsert first", upsert: true},
		{name: "delete before upsert", deleteFirst: true, upsert: true},
		{name: "delete before first grant", deleteFirst: true, upsert: true, missingGrant: true},
		{name: "rollback releases lock", deleteFirst: true, ownedRepo: true, rollback: true},
		{name: "soft rollback releases lock", soft: true, deleteFirst: true, ownedRepo: true, rollback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			t.Cleanup(cancel)
			users := database.NewUserStoreWithDBAndJobClients(db, &userDeletionJobClient{}, producer)
			user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
			require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
			repo := &database.Repository{Path: "external/" + uuid.NewString(), GitPath: "models_" + user.Username + "/repo", Name: "repo", RepositoryType: types.ModelRepo}
			if tc.ownedRepo {
				repo.UserID, repo.Path = user.ID, user.Username+"/repo"
			}
			_, err := db.BunDB.NewInsert().Model(repo).Exec(ctx)
			require.NoError(t, err)
			grants := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
			grant := &database.RepositoryAuthorization{RepositoryID: repo.ID, SubjectType: types.RepoAuthSubjectUser, SubjectID: user.ID, SubjectUUID: user.UUID, Role: types.UserRead}
			if !tc.missingGrant {
				require.NoError(t, grants.Create(ctx, grant))
			}
			if tc.rollback {
				users = database.NewUserStoreWithDBAndJobClients(db, &userDeletionJobClient{}, failingTupleJobs{delegate: producer, source: "user.direct_grants.delete"})
			}
			remove := func(opCtx context.Context) error {
				if tc.soft {
					return users.SoftDeleteUserAndRelations(opCtx, *user, types.CloseAccountReq{Repository: true})
				}
				return users.DeleteUserAndRelations(opCtx, *user, types.CloseAccountReq{})
			}
			mutate := func(opCtx context.Context) error {
				if tc.upsert {
					next := *grant
					next.Role = types.UserWrite
					return grants.Create(opCtx, &next)
				}
				return grants.UpdateRole(opCtx, repo.ID, user.ID, types.RepoAuthSubjectUser, types.UserWrite)
			}
			first, second := mutate, remove
			if tc.deleteFirst {
				first, second = remove, mutate
			}
			hook := &rebacGrantPauseHook{reached: make(chan struct{}), resume: make(chan struct{})}
			firstErr, secondErr := hook.run(t, ctx, db, user.ID, first, second)
			if tc.rollback {
				require.ErrorContains(t, firstErr, "injected failure")
			} else {
				require.NoError(t, firstErr, "first operation must not deadlock")
			}
			if tc.deleteFirst && !tc.rollback {
				require.True(t, errors.Is(secondErr, sql.ErrNoRows) || errors.Is(secondErr, errorx.ErrDatabaseNoRows) || errors.Is(secondErr, errorx.ErrRepoAuthorizationNotFound),
					"expected a missing/deleted subject or repository, not a deadlock: %v", secondErr)
			} else {
				require.NoError(t, secondErr, "second operation must not deadlock")
			}
			expected := types.UserRole("")
			if tc.rollback {
				expected = types.UserWrite
			}
			assertRebacGrantProjection(t, db, repo.ID, user.UUID, expected)
		})
	}
}

// assertMemberProjection compares committed permission intents with the active SQL membership.
func assertMemberProjection(t *testing.T, db *database.DB, org database.Organization, user database.User, role types.UserRole) {
	t.Helper()
	var payloads [][]byte
	require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("args").Where("kind = ?", workhub.RebacTupleQueue).Order("id ASC").Scan(context.Background(), &payloads))
	present := make(map[rebac.Relationship]bool)
	for _, payload := range payloads {
		var event types.RebacTuple
		require.NoError(t, json.Unmarshal(payload, &event))
		for _, op := range event.Operations {
			for _, relationship := range op.Relationships {
				present[relationship] = !op.Delete
			}
		}
	}
	desired, _ := role.ReBACRelation()
	for _, relation := range []rebac.Relation{rebac.RelationReader, rebac.RelationWriter, rebac.RelationAdmin} {
		r := rebac.Relationship{Subject: rebac.UserSubject(user.UUID), Relation: relation, Object: rebac.OrganizationObject(org.UUID.String())}
		require.Equal(t, role != "" && relation == desired, present[r], "durable projection for %s", r.String())
	}
	member, err := database.NewMemberStoreWithDB(db).Find(context.Background(), org.ID, user.ID)
	if role == "" {
		require.ErrorIs(t, err, sql.ErrNoRows)
	} else {
		require.NoError(t, err)
		require.Equal(t, string(role), member.Role)
	}
}

// TestRebacTupleUserMemberConcurrency verifies both commit orders and rollback with
// real lock waits, including administrator protection and hierarchy row locks.
func TestRebacTupleUserMemberConcurrency(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	setOrgStoreTestDB(t, db)
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	users := database.NewUserStoreWithDBAndJobClients(db, &userDeletionJobClient{}, producer)
	keeper := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
	require.NoError(t, users.Create(context.Background(), keeper, &database.Namespace{Path: keeper.Username, UUID: uuid.NewString()}))
	org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New(), UserID: keeper.ID}
	orgs := database.NewOrgStoreWithJobClients(true, nil, nil, producer)
	require.NoError(t, orgs.CreateWithRelations(context.Background(), org, &database.Namespace{Path: org.Name, UUID: uuid.NewString()}, nil))
	members := database.NewMemberStoreWithRebacTupleJobClient(db, producer)
	hierarchy := database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, producer)
	for _, operation := range []string{"add", "update", "reconcile", "hierarchy add", "hierarchy role", "creator reconcile"} {
		for _, soft := range []bool{false, true} {
			for _, order := range []string{"member first", "delete first", "delete rollback"} {
				t.Run(fmt.Sprintf("%s/soft=%t/%s", operation, soft, order), func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					t.Cleanup(cancel)
					user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
					require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
					if operation != "add" && operation != "hierarchy add" {
						// Two admins exercise deletion's existing administrator protection.
						require.NoError(t, members.Add(ctx, org.ID, user.ID, string(types.UserAdmin)))
					}
					if operation == "creator reconcile" {
						_, err := db.BunDB.NewUpdate().Model((*database.Organization)(nil)).Set("user_id = ?", user.ID).Where("id = ?", org.ID).Exec(ctx)
						require.NoError(t, err)
					}
					deletingUsers := users
					if order == "delete rollback" {
						deletingUsers = database.NewUserStoreWithDBAndJobClients(db, &userDeletionJobClient{}, failingTupleJobs{delegate: producer, source: "user.delete"})
					}
					remove := func(opCtx context.Context) error {
						if soft {
							return deletingUsers.SoftDeleteUserAndRelations(opCtx, *user, types.CloseAccountReq{Repository: true})
						}
						return deletingUsers.DeleteUserAndRelations(opCtx, *user, types.CloseAccountReq{})
					}
					mutate := func(opCtx context.Context) error {
						switch operation {
						case "add":
							return members.Add(opCtx, org.ID, user.ID, string(types.UserWrite))
						case "update":
							return members.Update(opCtx, org.ID, user.ID, string(types.UserWrite))
						case "reconcile":
							return members.ReconcileTuples(opCtx, org.ID, user.ID)
						case "hierarchy add":
							_, err := hierarchy.UpsertMembers(opCtx, database.AddOrganizationMembersInput{RootOrganizationID: org.ID, OrganizationID: org.ID, UserUUIDs: []string{user.UUID}, Role: types.UserWrite})
							return err
						case "hierarchy role":
							return hierarchy.UpdateMemberRole(opCtx, database.UpdateOrganizationMemberRoleInput{RootOrganizationID: org.ID, OrganizationID: org.ID, UserID: user.ID, Role: types.UserWrite})
						default:
							return orgs.ReconcileNamespaceTuples(opCtx, org.ID)
						}
					}
					first, second := remove, mutate
					query := `FROM "repository_authorizations"`
					if order == "member first" {
						first, second = mutate, remove
						query = `FROM "users"`
						if operation == "hierarchy add" {
							query = `INSERT INTO "members"`
						}
					}
					hook := &rebacGrantPauseHook{query: query, reached: make(chan struct{}), resume: make(chan struct{})}
					firstErr, secondErr := hook.run(t, ctx, db, user.ID, first, second)
					if order == "delete rollback" {
						require.ErrorContains(t, firstErr, "injected failure")
					} else {
						require.NoError(t, firstErr, "first operation must not deadlock")
					}
					if order == "delete first" && operation != "creator reconcile" && !(operation == "reconcile" && soft) {
						require.Error(t, secondErr, "deleted users cannot receive new grants")
						require.NotContains(t, secondErr.Error(), "deadlock")
						require.NotContains(t, secondErr.Error(), "deadline")
					} else {
						require.NoError(t, secondErr, "second operation must not deadlock")
					}
					expected := types.UserRole("")
					if order == "delete rollback" {
						expected = types.UserWrite
						if operation == "reconcile" || operation == "creator reconcile" {
							expected = types.UserAdmin
						}
					}
					assertMemberProjection(t, db, *org, *user, expected)
				})
			}
		}
	}
}

// userAdminLockFixture owns independent users and a hierarchy for a lock-order test.
type userAdminLockFixture struct {
	db                 *database.DB
	users              database.UserStore
	members            database.OrganizationUnitMemberStore
	root, organization database.Organization
	unitID             *int64
	deleting, changing database.User
}

// newUserAdminLockFixture gives both competing users admin roles and an optional survivor.
func newUserAdminLockFixture(t *testing.T, child, keeper bool) userAdminLockFixture {
	t.Helper()
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	users := database.NewUserStoreWithDBAndJobClients(db, &userDeletionJobClient{}, producer)
	fixture := userAdminLockFixture{db: db, users: users, members: database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, producer)}
	people := []*database.User{&fixture.deleting, &fixture.changing}
	if keeper {
		people = append(people, &database.User{})
	}
	for _, person := range people {
		person.Username, person.UUID = uuid.NewString(), uuid.NewString()
		require.NoError(t, users.Create(ctx, person, &database.Namespace{Path: person.Username, UUID: uuid.NewString()}))
	}
	units := database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer)
	fixture.root = database.Organization{Name: uuid.NewString(), UUID: uuid.New(), UserID: fixture.deleting.ID}
	_, err = units.CreateRoot(ctx, database.CreateRootOrganizationInput{Organization: &fixture.root,
		Namespace: &database.Namespace{Path: fixture.root.Name, UUID: uuid.NewString()}, CreatorUserID: fixture.deleting.ID})
	require.NoError(t, err)
	fixture.organization = fixture.root
	ordinary := database.NewMemberStoreWithRebacTupleJobClient(db, producer)
	for _, person := range people[1:] {
		require.NoError(t, ordinary.Add(ctx, fixture.root.ID, person.ID, string(types.UserAdmin)))
	}
	if child {
		fixture.organization = database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
		_, err := units.Create(ctx, database.CreateOrganizationUnitInput{RootOrganizationID: fixture.root.ID,
			Organization: &fixture.organization, Namespace: &database.Namespace{Path: fixture.organization.Name, UUID: uuid.NewString()}, MaxDepth: 8})
		require.NoError(t, err)
		var unit database.OrganizationUnit
		require.NoError(t, db.Core.NewSelect().Model(&unit).Where("organization_id = ?", fixture.organization.ID).Scan(ctx))
		fixture.unitID = &unit.ID
		for _, person := range people {
			require.NoError(t, ordinary.Add(ctx, fixture.organization.ID, person.ID, string(types.UserAdmin)))
		}
		// Child-only admins still require the hierarchy root lock during deletion.
		_, err = fixture.members.RemoveMembers(ctx, database.RemoveOrganizationMembersInput{
			RootOrganizationID: fixture.root.ID, OrganizationID: fixture.root.ID,
			UserUUIDs: []string{fixture.deleting.UUID}})
		require.NoError(t, err)
	}
	return fixture
}

// changeMember exercises each hierarchy mutation against the competing administrator.
func (f userAdminLockFixture) changeMember(ctx context.Context, operation string) error {
	switch operation {
	case "upsert":
		_, err := f.members.UpsertMembers(ctx, database.AddOrganizationMembersInput{
			RootOrganizationID: f.root.ID, OrganizationID: f.organization.ID, OrganizationUnitID: f.unitID,
			UserUUIDs: []string{f.changing.UUID}, Role: types.UserWrite})
		return err
	case "remove":
		_, err := f.members.RemoveMembers(ctx, database.RemoveOrganizationMembersInput{
			RootOrganizationID: f.root.ID, OrganizationID: f.organization.ID, OrganizationUnitID: f.unitID,
			UserUUIDs: []string{f.changing.UUID}})
		return err
	default:
		return f.members.UpdateMemberRole(ctx, database.UpdateOrganizationMemberRoleInput{
			RootOrganizationID: f.root.ID, OrganizationID: f.organization.ID,
			OrganizationUUID: f.organization.UUID.String(), OrganizationUnitID: f.unitID,
			UserID: f.changing.ID, Role: types.UserWrite})
	}
}

// deleteUser selects the production hard or soft deletion transaction.
func (f userAdminLockFixture) deleteUser(ctx context.Context, soft bool) error {
	if soft {
		return f.users.SoftDeleteUserAndRelations(ctx, f.deleting, types.CloseAccountReq{Repository: true})
	}
	return f.users.DeleteUserAndRelations(ctx, f.deleting, types.CloseAccountReq{})
}

// TestRebacTupleUserAdminLockOrder verifies both commit orders, roots and children,
// and the last-admin guard for all three hierarchy membership mutations.
func TestRebacTupleUserAdminLockOrder(t *testing.T) {
	for _, child := range []bool{false, true} {
		for _, soft := range []bool{false, true} {
			for _, operation := range []string{"upsert", "remove", "role"} {
				for _, keeper := range []bool{false, true} {
					for _, deleteFirst := range []bool{false, true} {
						t.Run(fmt.Sprintf("child=%t/soft=%t/%s/keeper=%t/deleteFirst=%t", child, soft, operation, keeper, deleteFirst), func(t *testing.T) {
							fixture := newUserAdminLockFixture(t, child, keeper)
							ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
							t.Cleanup(cancel)
							change := func(ctx context.Context) error { return fixture.changeMember(ctx, operation) }
							remove := func(ctx context.Context) error { return fixture.deleteUser(ctx, soft) }
							first, second := change, remove
							hook := &rebacGrantPauseHook{query: `FROM "organizations"`, querySuffix: "FOR UPDATE",
								waitQuery: "%organizations%FOR UPDATE%", reached: make(chan struct{}), resume: make(chan struct{})}
							if deleteFirst {
								first, second = remove, change
								hook.query, hook.querySuffix = "JOIN users AS admin_user", ""
							}
							firstErr, secondErr := hook.run(t, ctx, fixture.db, fixture.deleting.ID, first, second)
							require.NoError(t, firstErr)
							if keeper {
								require.NoError(t, secondErr)
							} else {
								require.ErrorIs(t, secondErr, errorx.ErrLastOrgAdmin, "expected protection rather than a deadlock")
							}
							deletingRole, changingRole := types.UserRole(""), types.UserWrite
							if operation == "remove" {
								changingRole = ""
							}
							if !keeper {
								if deleteFirst {
									changingRole = types.UserAdmin
								} else {
									deletingRole = types.UserAdmin
								}
							}
							assertMemberProjection(t, fixture.db, fixture.organization, fixture.deleting, deletingRole)
							assertMemberProjection(t, fixture.db, fixture.organization, fixture.changing, changingRole)
							if child {
								assertMemberProjection(t, fixture.db, fixture.root, fixture.deleting, "")
								assertMemberProjection(t, fixture.db, fixture.root, fixture.changing, types.UserAdmin)
							}
						})
					}
				}
			}
		}
	}
}

// TestRebacTupleUserAdminLockRollback checks that a failed member transaction
// releases the root and preserves its admin role and permission event atomically.
func TestRebacTupleUserAdminLockRollback(t *testing.T) {
	for _, child := range []bool{false, true} {
		for _, soft := range []bool{false, true} {
			t.Run(fmt.Sprintf("child=%t/soft=%t", child, soft), func(t *testing.T) {
				fixture := newUserAdminLockFixture(t, child, false)
				producer, err := workhub.NewRebacTupleJobClientWithDB(fixture.db)
				require.NoError(t, err)
				fixture.members = database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(fixture.db,
					failingTupleJobs{delegate: producer})
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				t.Cleanup(cancel)
				hook := &rebacGrantPauseHook{query: `FROM "organizations"`, querySuffix: "FOR UPDATE",
					waitQuery: "%organizations%FOR UPDATE%", reached: make(chan struct{}), resume: make(chan struct{})}
				changeErr, deleteErr := hook.run(t, ctx, fixture.db, fixture.deleting.ID,
					func(ctx context.Context) error { return fixture.changeMember(ctx, "role") },
					func(ctx context.Context) error { return fixture.deleteUser(ctx, soft) })
				require.ErrorContains(t, changeErr, "injected failure")
				require.NoError(t, deleteErr)
				assertMemberProjection(t, fixture.db, fixture.organization, fixture.deleting, "")
				assertMemberProjection(t, fixture.db, fixture.organization, fixture.changing, types.UserAdmin)
			})
		}
	}
}

// TestRebacTupleUserAdminLockDeletedHierarchy rechecks live organizations after
// root/subtree deletion commits while the last-admin check waits for its root.
func TestRebacTupleUserAdminLockDeletedHierarchy(t *testing.T) {
	for _, rootDeletion := range []bool{false, true} {
		for _, soft := range []bool{false, true} {
			t.Run(fmt.Sprintf("rootDeletion=%t/soft=%t", rootDeletion, soft), func(t *testing.T) {
				fixture := newUserAdminLockFixture(t, true, false)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				t.Cleanup(cancel)
				// The deleted user is the child's last admin and has no root membership.
				require.NoError(t, fixture.changeMember(ctx, "remove"))
				producer, err := workhub.NewRebacTupleJobClientWithDB(fixture.db)
				require.NoError(t, err)
				jobs := &testRepositoryDeletionJobClient{}
				units := database.NewOrganizationUnitStoreWithDBAndJobClients(fixture.db, jobs, jobs, producer)
				removeOrganization := func(ctx context.Context) error {
					if rootDeletion {
						_, err := units.DeleteRoot(ctx, database.DeleteRootOrganizationInput{OrganizationUUID: fixture.root.UUID.String()})
						return err
					}
					_, err := units.Delete(ctx, database.DeleteOrganizationUnitInput{RootOrganizationID: fixture.root.ID, UnitID: *fixture.unitID})
					return err
				}
				hook := &rebacGrantPauseHook{query: `FROM "organizations"`, querySuffix: "FOR UPDATE",
					waitQuery: "%organizations%FOR UPDATE%", reached: make(chan struct{}), resume: make(chan struct{})}
				orgErr, userErr := hook.run(t, ctx, fixture.db, fixture.deleting.ID, removeOrganization,
					func(ctx context.Context) error { return fixture.deleteUser(ctx, soft) })
				require.NoError(t, orgErr)
				require.NoError(t, userErr, "a deleted organization must not trigger the last-admin guard")
				exists, err := fixture.users.IsExist(ctx, fixture.deleting.Username)
				require.NoError(t, err)
				require.False(t, exists)
				assertMemberProjection(t, fixture.db, fixture.organization, fixture.deleting, "")
				assertMemberProjection(t, fixture.db, fixture.root, fixture.deleting, "")
				inputs := jobs.recordedOrganizationInputs()
				require.Len(t, inputs, 1)
				require.Contains(t, inputs[0].OrganizationIDs, fixture.organization.ID)
			})
		}
	}
}

// TestRebacTupleCreatorConcurrency verifies automatic memberships share the deletion
// lock, and a failed last-admin deletion rolls back its permission cleanup event.
func TestRebacTupleCreatorConcurrency(t *testing.T) {
	for _, hierarchy := range []bool{false, true} {
		for _, soft := range []bool{false, true} {
			for _, deleteFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("hierarchy=%t/soft=%t/deleteFirst=%t", hierarchy, soft, deleteFirst), func(t *testing.T) {
					db := tests.InitTransactionTestDB()
					t.Cleanup(func() { _ = db.Close() })
					setOrgStoreTestDB(t, db)
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					t.Cleanup(cancel)
					producer, err := workhub.NewRebacTupleJobClientWithDB(db)
					require.NoError(t, err)
					users := database.NewUserStoreWithDBAndJobClients(db, &userDeletionJobClient{}, producer)
					user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
					require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
					org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New(), UserID: user.ID}
					ns := &database.Namespace{Path: org.Name, UUID: uuid.NewString()}
					create := func(opCtx context.Context) error {
						if hierarchy {
							_, err := database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer).CreateRoot(opCtx, database.CreateRootOrganizationInput{Organization: org, Namespace: ns, CreatorUserID: user.ID})
							return err
						}
						return database.NewOrgStoreWithJobClients(false, nil, nil, producer).CreateWithRelations(opCtx, org, ns, nil)
					}
					remove := func(opCtx context.Context) error {
						if soft {
							return users.SoftDeleteUserAndRelations(opCtx, *user, types.CloseAccountReq{Repository: true})
						}
						return users.DeleteUserAndRelations(opCtx, *user, types.CloseAccountReq{})
					}
					first, second := create, remove
					query := `INSERT INTO "members"`
					if deleteFirst {
						first, second = remove, create
						query = `FROM "repository_authorizations"`
					}
					hook := &rebacGrantPauseHook{query: query, reached: make(chan struct{}), resume: make(chan struct{})}
					firstErr, secondErr := hook.run(t, ctx, db, user.ID, first, second)
					require.NoError(t, firstErr)
					if deleteFirst {
						require.True(t, errors.Is(secondErr, sql.ErrNoRows) || errors.Is(secondErr, errorx.ErrDatabaseNoRows), "expected missing creator, got %v", secondErr)
						count, err := db.BunDB.NewSelect().Model((*database.Organization)(nil)).Where("uuid = ?", org.UUID).Count(ctx)
						require.NoError(t, err)
						require.Zero(t, count)
						assertMemberProjection(t, db, *org, *user, "")
					} else {
						require.ErrorIs(t, secondErr, errorx.ErrLastOrgAdmin)
						assertMemberProjection(t, db, *org, *user, types.UserAdmin)
					}
				})
			}
		}
	}
}

// TestRebacTupleMemberBatchLockOrder uses opposite caller order and different
// organizations so only subject locking can prevent conflicting batch lock order.
func TestRebacTupleMemberBatchLockOrder(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	setOrgStoreTestDB(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	users := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
	first := &database.User{Username: uuid.NewString(), UUID: "z-" + uuid.NewString()}
	second := &database.User{Username: uuid.NewString(), UUID: "a-" + uuid.NewString()}
	for _, user := range []*database.User{first, second} {
		require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
	}
	root := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	require.NoError(t, database.NewOrgStoreWithJobClients(true, nil, nil, producer).CreateWithRelations(ctx, root, &database.Namespace{Path: root.Name, UUID: uuid.NewString()}, nil))
	unitStore := database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer)
	child := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	unit, err := unitStore.Create(ctx, database.CreateOrganizationUnitInput{RootOrganizationID: root.ID, Organization: child, Namespace: &database.Namespace{Path: child.Name, UUID: uuid.NewString()}, MaxDepth: 8})
	require.NoError(t, err)
	unitID := organizationUnitIDForUnit(t, ctx, unitStore, unit.UUID)
	members := database.NewOrganizationUnitMemberStoreWithRebacTupleJobClient(db, producer)
	firstBatch := func(opCtx context.Context) error {
		_, err := members.UpsertMembers(opCtx, database.AddOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: root.ID, UserUUIDs: []string{second.UUID, first.UUID, second.UUID}, Role: types.UserAdmin})
		return err
	}
	secondBatch := func(opCtx context.Context) error {
		_, err := members.UpsertMembers(opCtx, database.AddOrganizationMembersInput{RootOrganizationID: root.ID, OrganizationID: child.ID, OrganizationUnitID: unitID, UserUUIDs: []string{first.UUID, second.UUID}, Role: types.UserAdmin})
		return err
	}
	// Pause immediately after the first subject lock, before the first batch owns
	// any organization or user row lock. The other batch must wait on that same ID.
	hook := &rebacGrantPauseHook{query: "pg_advisory_xact_lock", reached: make(chan struct{}), resume: make(chan struct{})}
	firstErr, secondErr := hook.run(t, ctx, db, first.ID, firstBatch, secondBatch)
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	for _, org := range []*database.Organization{root, child} {
		for _, user := range []*database.User{first, second} {
			assertMemberProjection(t, db, *org, *user, types.UserAdmin)
		}
	}
}

// TestRebacTupleSoftDeleteRepositoryConcurrency checks actual namespace lock waits
// for ordinary, MCP and mirror creates, upserts and transfers, including rollback
// after the repository event.
func TestRebacTupleSoftDeleteRepositoryConcurrency(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	setOrgStoreTestDB(t, db)
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	for _, operation := range []string{"create", "create empty", "mcp create", "mirror create", "upsert", "transfer in", "transfer out"} {
		for _, order := range []string{"writer first", "delete first", "delete rollback"} {
			if operation == "create empty" && order == "delete rollback" {
				continue
			}
			t.Run(operation+"/"+order, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				t.Cleanup(cancel)
				users := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
				target := &database.User{Username: "z-" + uuid.NewString(), UUID: uuid.NewString()}
				other := &database.User{Username: "m-" + uuid.NewString(), UUID: uuid.NewString()}
				for _, user := range []*database.User{target, other} {
					require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
				}
				org := &database.Organization{Name: "a-" + uuid.NewString(), UUID: uuid.New(), UserID: other.ID}
				require.NoError(t, database.NewOrgStoreWithJobClients(false, nil, nil, producer).CreateWithRelations(ctx, org, &database.Namespace{Path: org.Name, UUID: uuid.NewString()}, nil))
				repos := database.NewRepoStoreWithDBAndJobClients(db, nil, producer)
				var base, orgRepo *database.Repository
				if operation != "create empty" {
					base, err = repos.CreateRepo(ctx, database.Repository{UserID: target.ID, Name: "base", Path: target.Username + "/base", GitPath: uuid.NewString(), RepositoryType: types.ModelRepo})
					require.NoError(t, err)
					// A user's deletion range also includes repositories they created in an organization.
					orgRepo, err = repos.CreateRepo(ctx, database.Repository{UserID: target.ID, Name: "org", Path: org.Name + "/org", GitPath: uuid.NewString(), RepositoryType: types.ModelRepo})
					require.NoError(t, err)
				}
				candidate := database.Repository{UserID: target.ID, Name: "concurrent", Path: target.Username + "/concurrent", GitPath: uuid.NewString(), RepositoryType: types.ModelRepo}
				if operation == "transfer in" || operation == "transfer out" {
					if operation == "transfer in" {
						candidate.UserID, candidate.Path = other.ID, other.Username+"/concurrent"
					}
					created, err := repos.CreateRepo(ctx, candidate)
					require.NoError(t, err)
					candidate = *created
				}
				mutate := func(opCtx context.Context) error {
					var updated *database.Repository
					var err error
					switch operation {
					case "create", "create empty":
						updated, err = repos.CreateRepo(opCtx, candidate)
					case "mcp create", "mirror create":
						err = createRebacRepositoryForConcurrency(opCtx, db, producer, operation, &candidate)
						updated = &candidate
					case "upsert":
						updated, err = repos.UpdateOrCreateRepo(opCtx, candidate)
					default:
						moved := candidate
						owner := target
						if operation == "transfer out" {
							owner = other
						}
						moved.UserID, moved.Path = owner.ID, owner.Username+"/concurrent"
						moved.TupleNamespacePath = owner.Username
						updated, err = repos.UpdateRepo(opCtx, moved)
					}
					if err == nil {
						candidate = *updated
					}
					return err
				}
				if order == "delete rollback" {
					users = database.NewUserStoreWithDBAndJobClients(db, nil, failingTupleJobs{delegate: producer, source: "user.repositories.delete"})
				}
				remove := func(opCtx context.Context) error {
					return users.SoftDeleteUserAndRelations(opCtx, *target, types.CloseAccountReq{Repository: true})
				}
				first, second := mutate, remove
				pauseLock, waitLock := "FOR KEY SHARE", "FOR UPDATE"
				if order != "writer first" {
					first, second = remove, mutate
					pauseLock, waitLock = waitLock, pauseLock
				}
				hook := &rebacGrantPauseHook{query: target.Username, querySuffix: pauseLock,
					waitQuery: `%FROM "namespaces"%` + target.Username + `%` + waitLock,
					reached:   make(chan struct{}), resume: make(chan struct{})}
				firstErr, secondErr := hook.run(t, ctx, db, target.ID, first, second)
				if order == "delete rollback" {
					require.ErrorContains(t, firstErr, "injected failure")
				} else {
					require.NoError(t, firstErr)
				}
				if order == "delete first" {
					require.ErrorContains(t, secondErr, "is deleted")
				} else {
					require.NoError(t, secondErr, "writer and deletion must not deadlock")
				}
				for _, repo := range []*database.Repository{base, orgRepo, &candidate} {
					if repo == nil || repo.ID == 0 {
						continue
					}
					var stored database.Repository
					require.NoError(t, db.BunDB.NewSelect().Model(&stored).WhereAllWithDeleted().Where("id = ?", repo.ID).Scan(ctx))
					expectedDeleted := order != "delete rollback" && stored.UserID == target.ID
					require.Equal(t, expectedDeleted, !stored.DeletedAt.IsZero())
					var expected []rebac.Relationship
					if stored.DeletedAt.IsZero() {
						owner := target
						if stored.UserID == other.ID {
							owner = other
						}
						r := rebac.Relationship{Subject: rebac.UserSubject(owner.UUID), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(repo.ID)}
						if strings.HasPrefix(stored.Path, org.Name+"/") {
							r.Subject, r.Relation = rebac.NewSubject(rebac.ObjectTypeOrganization, org.UUID.String()), rebac.RelationOrganization
						}
						expected = append(expected, r)
					}
					assertRepositoryTupleProjection(t, db, repo.ID, expected)
				}
				active, err := users.IsExist(ctx, target.Username)
				require.NoError(t, err)
				require.Equal(t, order == "delete rollback", active)
			})
		}
	}
}

// createRebacRepositoryForConcurrency exercises the actual MCP and mirror creation transactions.
func createRebacRepositoryForConcurrency(ctx context.Context, db *database.DB, producer database.RebacTupleJobClient, operation string, repo *database.Repository) error {
	if operation == "mcp create" {
		repo.RepositoryType = types.SpaceRepo
		_, err := database.NewMCPServerStoreWithRebacTupleJobClient(db, producer).CreateSpaceAndRepoForDeploy(ctx, repo, &database.Space{})
		return err
	}
	mirror, err := database.NewMirrorRepoStoreWithJobClients(db, nil, producer).CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		CreateRepository: true, Repository: repo,
		Mirror: database.Mirror{SourceUrl: "https://example.com/source.git", SourceRepoPath: "upstream/repo", Priority: types.ASAPMirrorPriority},
	})
	if err == nil {
		*repo = *mirror.Repository
	}
	return err
}

// TestRebacTupleOrganizationRepositoryCreationConcurrency verifies deletion snapshots
// include committed MCP/mirror repositories and reject creators after namespace deletion.
func TestRebacTupleOrganizationRepositoryCreationConcurrency(t *testing.T) {
	for _, route := range []string{"ordinary", "root", "subtree"} {
		for _, operation := range []string{"mcp create", "mirror create"} {
			for _, order := range []string{"writer first", "delete first", "delete rollback"} {
				t.Run(route+"/"+operation+"/"+order, func(t *testing.T) {
					db := tests.InitTransactionTestDB()
					t.Cleanup(func() { _ = db.Close() })
					setOrgStoreTestDB(t, db)
					producer, err := workhub.NewRebacTupleJobClientWithDB(db)
					require.NoError(t, err)
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					jobs := &testRepositoryDeletionJobClient{}
					org, remove := newRebacRepositoryDeletionFixture(t, ctx, db, producer, jobs, route)
					if order == "delete rollback" {
						jobs.organizationErr = errors.New("injected organization deletion enqueue failure")
					}
					repo := database.Repository{Name: "concurrent", Path: org.Name + "/concurrent", GitPath: uuid.NewString(), RepositoryType: types.ModelRepo}
					mutate := func(opCtx context.Context) error {
						return createRebacRepositoryForConcurrency(opCtx, db, producer, operation, &repo)
					}
					first, second := mutate, remove
					pauseLock, waitLock := "FOR KEY SHARE", "FOR UPDATE"
					if order != "writer first" {
						first, second = remove, mutate
						pauseLock, waitLock = waitLock, pauseLock
					}
					hook := &rebacGrantPauseHook{query: `FROM "namespaces"`, querySuffix: pauseLock,
						waitQuery: `%FROM "namespaces"%` + waitLock,
						reached:   make(chan struct{}), resume: make(chan struct{})}
					countCreationJobs := func() int {
						n, err := db.BunDB.NewSelect().Table("river_job").Where("kind = ? AND args->>'source' = ?", workhub.RebacTupleQueue, "repository.namespace").Count(ctx)
						require.NoError(t, err)
						return n
					}
					before := countCreationJobs()
					firstErr, secondErr := hook.run(t, ctx, db, 0, first, second)
					if order == "delete rollback" {
						require.ErrorContains(t, firstErr, "injected")
					} else {
						require.NoError(t, firstErr)
					}
					if order == "delete first" {
						require.ErrorContains(t, secondErr, "is deleted")
						require.Zero(t, repo.ID, "rejected creation must not insert a repository")
						require.Empty(t, jobs.recordedInputs())
						require.Equal(t, before, countCreationJobs(), "rejected creation cannot commit a tuple event")
						return
					}
					require.NoError(t, secondErr, "creation and deletion must not deadlock")
					require.Equal(t, before+1, countCreationJobs())
					var stored database.Repository
					require.NoError(t, db.BunDB.NewSelect().Model(&stored).WhereAllWithDeleted().Where("id = ?", repo.ID).Scan(ctx))
					require.Equal(t, order == "writer first", !stored.DeletedAt.IsZero())
					if order == "writer first" {
						inputs := jobs.recordedInputs()
						require.Len(t, inputs, 1, "deletion snapshot must include the concurrent repository")
						require.Equal(t, repo.ID, inputs[0].RepositoryID)
						require.Equal(t, database.OrgNamespace, inputs[0].OwnerType)
						require.Equal(t, org.UUID.String(), inputs[0].OwnerUUID)
					}
					var expected []rebac.Relationship
					if order != "writer first" {
						expected = []rebac.Relationship{{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, org.UUID.String()), Relation: rebac.RelationOrganization, Object: rebac.RepositoryObject(repo.ID)}}
					}
					assertRepositoryTupleProjection(t, db, repo.ID, expected)
				})
			}
		}
	}
}

// newRebacRepositoryDeletionFixture supplies each existing organization deletion entry point.
func newRebacRepositoryDeletionFixture(t *testing.T, ctx context.Context, db *database.DB, producer database.RebacTupleJobClient, jobs *testRepositoryDeletionJobClient, route string) (*database.Organization, func(context.Context) error) {
	t.Helper()
	org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	ns := &database.Namespace{Path: org.Name, UUID: uuid.NewString()}
	if route == "ordinary" {
		store := database.NewOrgStoreWithJobClients(false, jobs, jobs, producer)
		require.NoError(t, store.CreateWithRelations(ctx, org, ns, nil))
		return org, func(opCtx context.Context) error { _, err := store.Delete(opCtx, org.Name); return err }
	}
	store := database.NewOrganizationUnitStoreWithDBAndJobClients(db, jobs, jobs, producer)
	root, err := store.CreateRoot(ctx, database.CreateRootOrganizationInput{Organization: org, Namespace: ns})
	require.NoError(t, err)
	if route == "root" {
		return org, func(opCtx context.Context) error {
			_, err := store.DeleteRoot(opCtx, database.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
			return err
		}
	}
	org = &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	unit, err := store.Create(ctx, database.CreateOrganizationUnitInput{RootOrganizationID: root.ID, Organization: org, Namespace: &database.Namespace{Path: org.Name, UUID: uuid.NewString()}, MaxDepth: 8})
	require.NoError(t, err)
	unitID := organizationUnitIDForUnit(t, ctx, store, unit.UUID)
	return org, func(opCtx context.Context) error {
		_, err := store.Delete(opCtx, database.DeleteOrganizationUnitInput{RootOrganizationID: root.ID, UnitID: *unitID})
		return err
	}
}

// assertRepositoryTupleProjection replays durable events and checks the repository's current owner.
func assertRepositoryTupleProjection(t *testing.T, db *database.DB, repoID int64, expected []rebac.Relationship) {
	t.Helper()
	var payloads [][]byte
	require.NoError(t, db.BunDB.NewSelect().Table("river_job").Column("args").Where("kind = ?", workhub.RebacTupleQueue).Order("id ASC").Scan(context.Background(), &payloads))
	present := make(map[rebac.Relationship]bool)
	for _, payload := range payloads {
		var event types.RebacTuple
		require.NoError(t, json.Unmarshal(payload, &event))
		for _, op := range event.Operations {
			for _, r := range op.Relationships {
				if r.Object == rebac.RepositoryObject(repoID) {
					present[r] = !op.Delete
				}
			}
		}
	}
	var actual []rebac.Relationship
	for r, exists := range present {
		if exists {
			actual = append(actual, r)
		}
	}
	require.ElementsMatch(t, expected, actual)
}

// TestRebacTupleSoftDeleteRepositoryScopeChange rejects a newly discovered namespace
// rather than acquiring its lock after repository locks or deleting an unrecorded scope.
func TestRebacTupleSoftDeleteRepositoryScopeChange(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	setOrgStoreTestDB(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	users := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
	user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
	require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
	org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
	require.NoError(t, database.NewOrgStoreWithJobClients(false, nil, nil, producer).CreateWithRelations(ctx, org, &database.Namespace{Path: org.Name, UUID: uuid.NewString()}, nil))
	hook := &rebacGrantPauseHook{query: `FROM "repositories"`, reached: make(chan struct{}), resume: make(chan struct{})}
	db.BunDB.AddQueryHook(hook)
	result := make(chan error, 1)
	var pending sync.WaitGroup
	pending.Add(1)
	defer func() { hook.release(); cancel(); pending.Wait() }()
	go func() {
		defer pending.Done()
		result <- users.SoftDeleteUserAndRelations(context.WithValue(ctx, rebacGrantPauseKey{}, hook), *user, types.CloseAccountReq{Repository: true})
	}()
	select {
	case <-hook.reached:
	case <-ctx.Done():
		t.Fatal("deletion did not reach repository discovery", ctx.Err())
	}
	repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, producer).CreateRepo(ctx, database.Repository{
		UserID: user.ID, Path: org.Name + "/late", Name: "late", GitPath: uuid.NewString(), RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)
	before := tupleJobCount(t, db)
	hook.release()
	require.ErrorContains(t, awaitRebacGrantResult(t, ctx, result), "namespace changed concurrently")
	require.Equal(t, before, tupleJobCount(t, db), "all deletion events must roll back")
	active, err := users.IsExist(ctx, user.Username)
	require.NoError(t, err)
	require.True(t, active)
	assertRepositoryTupleProjection(t, db, repo.ID, []rebac.Relationship{{Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, org.UUID.String()), Relation: rebac.RelationOrganization, Object: rebac.RepositoryObject(repo.ID)}})
	// Retrying uses the new namespace in the initial lock set and deletes the same repository.
	require.NoError(t, users.SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{Repository: true}))
	assertRepositoryTupleProjection(t, db, repo.ID, nil)
}

// TestRebacTupleSoftDeleteRepositorySplitsJobs keeps the existing per-job limit
// and rolls back the already executed SQL deletion if its permission event is too large.
func TestRebacTupleSoftDeleteRepositorySplitsJobs(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	users := database.NewUserStoreWithDBAndJobClients(db, nil, producer)
	user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
	require.NoError(t, users.Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
	var repos []database.Repository
	for i := 0; i <= workhub.RebacTupleMaxRelationships; i++ {
		repos = append(repos, database.Repository{UserID: user.ID, Path: fmt.Sprintf("%s/repo-%d", user.Username, i), GitPath: uuid.NewString(), RepositoryType: types.ModelRepo})
	}
	_, err = db.BunDB.NewInsert().Model(&repos).Exec(ctx)
	require.NoError(t, err)
	before := tupleJobCount(t, db)
	require.NoError(t, users.SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{Repository: true}))
	require.Equal(t, before+3, tupleJobCount(t, db))
	count, err := db.BunDB.NewSelect().Model((*database.Repository)(nil)).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	active, err := users.IsExist(ctx, user.Username)
	require.NoError(t, err)
	require.False(t, active)
	_, err = database.NewNamespaceStoreWithDB(db).FindByPath(ctx, user.Username)
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
}

// TestRebacTupleOrganizationGrantConcurrency verifies organization deletion and
// grant mutations share row locks, including root/subtree repository lock order.
func TestRebacTupleOrganizationGrantConcurrency(t *testing.T) {
	for _, route := range []string{"ordinary", "root", "subtree"} {
		for _, operation := range []string{"create", "update", "revoke"} {
			for _, owned := range []bool{false, true} {
				for _, order := range []string{"grant first", "delete first", "delete rollback", "grant rollback"} {
					t.Run(fmt.Sprintf("%s/%s/owned=%t/%s", route, operation, owned, order), func(t *testing.T) {
						db := tests.InitTransactionTestDB()
						t.Cleanup(func() { _ = db.Close() })
						setOrgStoreTestDB(t, db)
						ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
						defer cancel()
						producer, err := workhub.NewRebacTupleJobClientWithDB(db)
						require.NoError(t, err)
						jobs := &testRepositoryDeletionJobClient{}
						org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
						var remove func(context.Context) error
						if route == "ordinary" {
							orgs := database.NewOrgStoreWithJobClients(false, jobs, jobs, producer)
							require.NoError(t, orgs.CreateWithRelations(ctx, org, &database.Namespace{Path: org.Name, UUID: uuid.NewString()}, nil))
							remove = func(opCtx context.Context) error { _, err := orgs.Delete(opCtx, org.Name); return err }
						} else {
							units := database.NewOrganizationUnitStoreWithDBAndJobClients(db, jobs, jobs, producer)
							root, err := units.CreateRoot(ctx, database.CreateRootOrganizationInput{Organization: org, Namespace: &database.Namespace{Path: org.Name, UUID: uuid.NewString()}})
							require.NoError(t, err)
							if route == "root" {
								remove = func(opCtx context.Context) error {
									_, err := units.DeleteRoot(opCtx, database.DeleteRootOrganizationInput{OrganizationUUID: root.UUID.String()})
									return err
								}
							}
							if route == "subtree" || !owned {
								child := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
								unit, err := units.Create(ctx, database.CreateOrganizationUnitInput{RootOrganizationID: root.ID, Organization: child, Namespace: &database.Namespace{Path: child.Name, UUID: uuid.NewString()}, MaxDepth: 8})
								require.NoError(t, err)
								org = child
								if route == "subtree" {
									unitID := organizationUnitIDForUnit(t, ctx, units, unit.UUID)
									remove = func(opCtx context.Context) error {
										_, err := units.Delete(opCtx, database.DeleteOrganizationUnitInput{RootOrganizationID: root.ID, UnitID: *unitID})
										return err
									}
									// Grant the descendant, then delete its ancestor subtree.
									grandchild := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
									_, err := units.Create(ctx, database.CreateOrganizationUnitInput{RootOrganizationID: root.ID, ParentUnitUUID: &unit.UUID, Organization: grandchild, Namespace: &database.Namespace{Path: grandchild.Name, UUID: uuid.NewString()}, MaxDepth: 8})
									require.NoError(t, err)
									org = grandchild
								}
							}
						}
						namespace := org.Name
						if !owned {
							user := &database.User{Username: uuid.NewString(), UUID: uuid.NewString()}
							require.NoError(t, database.NewUserStoreWithDBAndJobClients(db, nil, producer).Create(ctx, user, &database.Namespace{Path: user.Username, UUID: uuid.NewString()}))
							namespace = user.Username
						}
						repo, err := database.NewRepoStoreWithDBAndJobClients(db, nil, producer).CreateRepo(ctx, database.Repository{Path: namespace + "/repo", GitPath: uuid.NewString(), Name: "repo", RepositoryType: types.ModelRepo})
						require.NoError(t, err)
						grants := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
						grant := &database.RepositoryAuthorization{RepositoryID: repo.ID, SubjectID: org.ID, SubjectUUID: org.UUID.String(), SubjectType: types.RepoAuthSubjectOrganization, Role: types.UserRead}
						if operation != "create" {
							require.NoError(t, grants.Create(ctx, grant))
						}
						if order == "grant rollback" {
							grants = database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, failingTupleJobs{delegate: producer, source: "repository.authorization"})
						}
						if order == "delete rollback" {
							jobs.organizationErr = errors.New("injected organization deletion enqueue failure")
						}
						mutate := func(opCtx context.Context) error {
							switch operation {
							case "create":
								next := *grant
								next.Role = types.UserWrite
								return grants.Create(opCtx, &next)
							case "update":
								return grants.UpdateRole(opCtx, repo.ID, org.ID, types.RepoAuthSubjectOrganization, types.UserWrite)
							default:
								return grants.Delete(opCtx, repo.ID, org.ID, types.RepoAuthSubjectOrganization)
							}
						}
						first, second := mutate, remove
						query := `FROM "repository_authorizations"`
						if operation == "create" {
							// The absent-grant SELECT returns ErrNoRows, so pause on
							// the successful organization lock before the first insert.
							query = "FOR UPDATE"
						}
						if order == "delete first" || order == "delete rollback" {
							first, second = remove, mutate
							query = "FOR UPDATE"
						}
						countGrantJobs := func() int {
							n, err := db.BunDB.NewSelect().Table("river_job").Where("kind = ? AND args->>'source' = ?", workhub.RebacTupleQueue, "repository.authorization").Count(ctx)
							require.NoError(t, err)
							return n
						}
						before := countGrantJobs()
						hook := &rebacGrantPauseHook{query: query, waitQuery: `%FROM "organizations"%FOR UPDATE%`, reached: make(chan struct{}), resume: make(chan struct{})}
						firstErr, secondErr := hook.run(t, ctx, db, 0, first, second)
						if order == "grant rollback" || order == "delete rollback" {
							require.ErrorContains(t, firstErr, "injected")
						} else {
							require.NoError(t, firstErr)
						}
						mutated := order != "grant rollback"
						if order == "delete first" && (operation != "revoke" || owned) {
							require.True(t, errors.Is(secondErr, sql.ErrNoRows) || errors.Is(secondErr, errorx.ErrDatabaseNoRows), "expected deleted subject/repository, got %v", secondErr)
							mutated = false
						} else {
							require.NoError(t, secondErr, "operations must complete without a lock cycle")
						}
						if mutated {
							require.Equal(t, before+1, countGrantJobs())
							payload := latestRebacTuple(t, db, "repository.authorization")
							require.Equal(t, "repository.authorization", payload.Source)
							if operation == "revoke" {
								require.True(t, payload.Operations[0].Delete)
							} else {
								require.False(t, payload.Operations[len(payload.Operations)-1].Delete)
								require.Equal(t, rebac.RelationWriter, payload.Operations[len(payload.Operations)-1].Relationships[0].Relation)
							}
						} else {
							require.Equal(t, before, countGrantJobs(), "rejected/rolled-back grants cannot commit a tuple job")
						}
						var storedOrg database.Organization
						require.NoError(t, db.BunDB.NewSelect().Model(&storedOrg).WhereAllWithDeleted().Where("id = ?", org.ID).Scan(ctx))
						require.Equal(t, order != "delete rollback", !storedOrg.DeletedAt.IsZero())
						var storedRepo database.Repository
						require.NoError(t, db.BunDB.NewSelect().Model(&storedRepo).WhereAllWithDeleted().Where("id = ?", repo.ID).Scan(ctx))
						require.Equal(t, owned && order != "delete rollback", !storedRepo.DeletedAt.IsZero())
						// This is the deletion worker's view after commit, which must include
						// every grant that won the lock before deletion and exclude rejected writes.
						remaining, err := grants.ListBySubject(ctx, types.RepoAuthSubjectOrganization, org.ID)
						require.NoError(t, err)
						expectedRole := types.UserRole("")
						if operation != "create" {
							expectedRole = types.UserRead
						}
						if mutated {
							expectedRole = types.UserWrite
							if operation == "revoke" {
								expectedRole = ""
							}
						}
						if expectedRole == "" {
							require.Empty(t, remaining)
						} else {
							require.Len(t, remaining, 1)
							require.Equal(t, expectedRole, remaining[0].Role)
						}
					})
				}
			}
		}
	}
}

// TestRebacTupleOrganizationGrantInactiveSubjects rejects writes without leaving
// SQL changes or events, while allowing repeated cleanup of retained grants.
func TestRebacTupleOrganizationGrantInactiveSubjects(t *testing.T) {
	for _, state := range []string{"missing organization", "deleted organization", "deleted root", "missing unit"} {
		t.Run(state, func(t *testing.T) {
			db := tests.InitTransactionTestDB()
			t.Cleanup(func() { _ = db.Close() })
			setOrgStoreTestDB(t, db)
			ctx := context.Background()
			producer, err := workhub.NewRebacTupleJobClientWithDB(db)
			require.NoError(t, err)
			root := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
			units := database.NewOrganizationUnitStoreWithDBAndJobClients(db, nil, nil, producer)
			_, err = units.CreateRoot(ctx, database.CreateRootOrganizationInput{Organization: root, Namespace: &database.Namespace{Path: root.Name, UUID: uuid.NewString()}})
			require.NoError(t, err)
			org := &database.Organization{Name: uuid.NewString(), UUID: uuid.New()}
			_, err = units.Create(ctx, database.CreateOrganizationUnitInput{RootOrganizationID: root.ID, Organization: org, Namespace: &database.Namespace{Path: org.Name, UUID: uuid.NewString()}, MaxDepth: 8})
			require.NoError(t, err)
			repo := &database.Repository{Path: "external/" + uuid.NewString(), RepositoryType: types.ModelRepo}
			_, err = db.BunDB.NewInsert().Model(repo).Exec(ctx)
			require.NoError(t, err)
			grants := database.NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, producer)
			grant := &database.RepositoryAuthorization{RepositoryID: repo.ID, SubjectID: org.ID, SubjectUUID: org.UUID.String(), SubjectType: types.RepoAuthSubjectOrganization, Role: types.UserRead}
			require.NoError(t, grants.Create(ctx, grant))
			switch state {
			case "missing organization":
				_, err = db.BunDB.NewDelete().Model(org).WherePK().ForceDelete().Exec(ctx)
			case "deleted organization":
				_, err = db.BunDB.NewDelete().Model(org).WherePK().Exec(ctx)
			case "deleted root":
				_, err = db.BunDB.NewDelete().Model(root).WherePK().Exec(ctx)
			case "missing unit":
				_, err = db.BunDB.NewDelete().Model((*database.OrganizationUnit)(nil)).Where("organization_id = ?", org.ID).ForceDelete().Exec(ctx)
			}
			require.NoError(t, err)
			before := tupleJobCount(t, db)
			next := *grant
			next.Role = types.UserWrite
			require.ErrorIs(t, grants.Create(ctx, &next), errorx.ErrDatabaseNoRows)
			require.ErrorIs(t, grants.UpdateRole(ctx, repo.ID, org.ID, types.RepoAuthSubjectOrganization, types.UserWrite), errorx.ErrDatabaseNoRows)
			require.Equal(t, before, tupleJobCount(t, db))
			stored, err := grants.Find(ctx, repo.ID, org.ID, types.RepoAuthSubjectOrganization)
			require.NoError(t, err)
			require.Equal(t, types.UserRead, stored.Role)
			require.NoError(t, grants.Delete(ctx, repo.ID, org.ID, types.RepoAuthSubjectOrganization))
			require.Equal(t, before+1, tupleJobCount(t, db))
			require.True(t, latestRebacTuple(t, db).Operations[0].Delete)
			require.NoError(t, grants.Delete(ctx, repo.ID, org.ID, types.RepoAuthSubjectOrganization))
			require.Equal(t, before+1, tupleJobCount(t, db))
		})
	}
}

// TestRebacTupleBatchLockOrder verifies reversed subjects wait rather than deadlock.
func TestRebacTupleBatchLockOrder(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback=%t", rollback), func(t *testing.T) {
			db := tests.InitTransactionTestDB()
			t.Cleanup(func() { _ = db.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			producer, err := workhub.NewRebacTupleJobClientWithDB(db)
			require.NoError(t, err)
			event := func(subject string) types.RebacTuple {
				return types.NewRebacTuple("batch.order", types.TupleOperation{Relationships: []rebac.Relationship{
					{Subject: rebac.UserSubject(subject), Relation: rebac.RelationReader, Object: rebac.RepositoryObject(1)},
				}})
			}
			first, err := db.BunDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = first.Rollback() }()
			second, err := db.BunDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = second.Rollback() }()
			var secondPID int
			require.NoError(t, second.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &secondPID))
			require.NoError(t, producer.InsertRebacTupleJobsTx(ctx, first.Tx, []types.RebacTuple{event("a"), event("b")}))
			result := make(chan error, 1)
			go func() {
				result <- producer.InsertRebacTupleJobsTx(ctx, second.Tx, []types.RebacTuple{event("b"), event("a")})
			}()
			require.Eventually(t, func() bool {
				var blocked bool
				err := db.BunDB.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = ? AND wait_event_type = 'Lock')", secondPID).Scan(ctx, &blocked)
				return err == nil && blocked
			}, 5*time.Second, 20*time.Millisecond)
			if rollback {
				require.NoError(t, first.Rollback())
			} else {
				require.NoError(t, first.Commit())
			}
			select {
			case err := <-result:
				require.NoError(t, err, "reversed groups must not deadlock")
			case <-ctx.Done():
				t.Fatal("second batch did not finish")
			}
			require.NoError(t, second.Commit())
			want := 4
			if rollback {
				want = 2
			}
			require.Equal(t, want, tupleJobCount(t, db))
		})
	}
}

// TestRebacTupleRiverMigration verifies schema rollback preserves ordinary jobs.
func TestRebacTupleRiverMigration(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	client, err := workhub.NewJobClient(ctx, db.BunDB)
	require.NoError(t, err)
	id, err := client.Insert(ctx, workhub.RepoArgs{}, nil)
	require.NoError(t, err)
	apply := func(direction string) error {
		// Roll back newer River schema first and restore it after ordering schema.
		names := []string{
			"20261014144512_add_river_ordering_schema." + direction,
			"20261020113455_add_river_queue_pause_schema." + direction,
		}
		if direction == "down" {
			names[0], names[1] = names[1], names[0]
		}
		return db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			for _, name := range names {
				content, err := os.ReadFile("builder/store/database/migrations/" + name + ".sql")
				if err != nil {
					return err
				}
				for statement := range strings.SplitSeq(string(content), "--bun:split") {
					if _, err := tx.ExecContext(ctx, statement); err != nil {
						return err
					}
				}
			}
			return nil
		})
	}
	require.NoError(t, apply("down"))
	var version int
	require.NoError(t, db.BunDB.NewRaw("SELECT MAX(version) FROM river_migration WHERE line = 'main'").Scan(ctx, &version))
	require.Equal(t, 7, version)
	require.NoError(t, apply("up"))
	n, err := db.BunDB.NewSelect().Table("river_job").Where("id = ? AND sequence_key IS NULL", id).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	producer, err := workhub.NewRebacTupleJobClientWithDB(db)
	require.NoError(t, err)
	require.NoError(t, db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return producer.InsertRebacTupleJobsTx(ctx, tx.Tx, []types.RebacTuple{types.NewRebacTuple("migration.guard", types.TupleOperation{Relationships: []rebac.Relationship{
			{Subject: rebac.UserSubject("u"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(1)},
		}})})
	}))
	require.ErrorContains(t, apply("down"), "unresolved jobs remain")
	require.NoError(t, db.BunDB.NewRaw("SELECT MAX(version) FROM river_migration WHERE line = 'main'").Scan(ctx, &version))
	require.Equal(t, 11, version)
}
