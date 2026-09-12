package component

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

type deletionStep struct {
	steps *[]string
	err   error
}

func (s deletionStep) DeleteRepo(context.Context, string) error {
	*s.steps = append(*s.steps, "git")
	return s.err
}

type deletionResources struct {
	steps *[]string
	err   error
}

func (r deletionResources) DeleteRepositoryResources(context.Context, workhub.RepositoryDeletionArgs) error {
	*r.steps = append(*r.steps, "resources")
	return r.err
}

type deletionAuthorizer struct {
	steps         *[]string
	relationships []rebac.Relationship
	checkRequests []rebac.CheckRequest
	decision      rebac.Decision
	err           error
}

func (a *deletionAuthorizer) Delete(_ context.Context, relationships []rebac.Relationship) error {
	*a.steps = append(*a.steps, "rebac")
	a.relationships = relationships
	return a.err
}
func (*deletionAuthorizer) Write(context.Context, []rebac.Relationship) error { return nil }
func (a *deletionAuthorizer) Check(_ context.Context, request rebac.CheckRequest) (rebac.Decision, error) {
	*a.steps = append(*a.steps, "rebac-check")
	a.checkRequests = append(a.checkRequests, request)
	return a.decision, a.err
}
func (*deletionAuthorizer) Authorize(context.Context, rebac.CheckRequest) error { return nil }
func (*deletionAuthorizer) BatchCheck(context.Context, rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
	return rebac.BatchCheckResult{}, nil
}
func (*deletionAuthorizer) ListObjects(context.Context, rebac.ListObjectsRequest) (rebac.ListObjectsResult, error) {
	return rebac.ListObjectsResult{}, nil
}
func (*deletionAuthorizer) ListSubjects(context.Context, rebac.ListSubjectsRequest) (rebac.ListSubjectsResult, error) {
	return rebac.ListSubjectsResult{}, nil
}

type deletionFinalizer struct {
	steps *[]string
	err   error
}

type deletionMirrorTaskFinder struct {
	steps  *[]string
	taskID int64
	err    error
}

func (f deletionMirrorTaskFinder) FindCurrentMirrorTaskID(context.Context, int64) (int64, error) {
	*f.steps = append(*f.steps, "mirror-find")
	return f.taskID, f.err
}

type deletionMirrorCanceler struct {
	steps   *[]string
	taskIDs []int64
	err     error
}

func (c *deletionMirrorCanceler) CancelMirror(_ context.Context, taskID int64) error {
	*c.steps = append(*c.steps, "mirror-cancel")
	c.taskIDs = append(c.taskIDs, taskID)
	return c.err
}

func (f deletionFinalizer) FinalizeRepositoryDeletion(context.Context, int64) error {
	*f.steps = append(*f.steps, "database")
	return f.err
}

func TestRepositoryDeletionWorkerCleansResourcesBeforeDatabase(t *testing.T) {
	steps := []string{}
	authorizer := &deletionAuthorizer{steps: &steps, decision: rebac.Decision{Allowed: true}}
	mirrorCanceler := &deletionMirrorCanceler{steps: &steps}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps}, authorizer,
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps, taskID: 88}, mirrorCanceler,
	)
	args := workhub.RepositoryDeletionArgs{
		RepositoryID: 42, RepositoryType: types.ModelRepo, GitalyPath: "models_org/repo.git",
		OwnerType: database.OrgNamespace, OwnerUUID: "org-uuid",
	}

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: args})
	require.NoError(t, err)
	require.Equal(t, []string{"mirror-find", "mirror-cancel", "resources", "git", "rebac-check", "rebac", "database"}, steps)
	require.Equal(t, []int64{88}, mirrorCanceler.taskIDs)
	require.Equal(t, []rebac.Relationship{{
		Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, "org-uuid"), Relation: rebac.RelationOrganization,
		Object: rebac.RepositoryObject(42),
	}}, authorizer.relationships)
	require.Equal(t, []rebac.CheckRequest{{
		Subject: rebac.NewSubject(rebac.ObjectTypeOrganization, "org-uuid"), Relation: rebac.RelationOrganization,
		Object: rebac.RepositoryObject(42), Consistency: rebac.ConsistencyHigher,
	}}, authorizer.checkRequests)
}

func TestRepositoryDeletionWorkerTreatsMissingGitRepositoryAsSuccess(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps, err: status.Error(codes.NotFound, "missing")},
		deletionResources{steps: &steps}, &deletionAuthorizer{steps: &steps},
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{
		RepositoryID: 7, OwnerType: database.UserNamespace, OwnerUUID: "user-uuid",
	}})
	require.NoError(t, err)
	require.Equal(t, []string{"mirror-find", "resources", "git", "rebac-check", "database"}, steps)
}

func TestRepositoryDeletionWorkerStopsBeforeDatabaseOnExternalFailure(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps, err: errors.New("storage unavailable")},
		&deletionAuthorizer{steps: &steps}, deletionFinalizer{steps: &steps},
		deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{RepositoryID: 9}})
	require.ErrorContains(t, err, "delete repository resources")
	require.Equal(t, []string{"mirror-find", "resources"}, steps)
}

func TestRepositoryDeletionWorkerRetriesAfterReBACDeleteWhenFinalizerFails(t *testing.T) {
	steps := []string{}
	authorizer := &deletionAuthorizer{steps: &steps, decision: rebac.Decision{Allowed: true}}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps}, authorizer,
		deletionFinalizer{steps: &steps, err: errors.New("database unavailable")},
		deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)
	job := &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{
		RepositoryID: 42, OwnerType: database.UserNamespace, OwnerUUID: "user-uuid",
	}}

	require.ErrorContains(t, worker.Work(context.Background(), job), "finalize repository database deletion")
	authorizer.decision.Allowed = false
	worker.finalizer = deletionFinalizer{steps: &steps}
	require.NoError(t, worker.Work(context.Background(), job))
	require.Len(t, authorizer.relationships, 1)
}

func TestRepositoryDeletionWorkerStopsBeforeResourcesWhenMirrorCancelFails(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps}, &deletionAuthorizer{steps: &steps},
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps, taskID: 99},
		&deletionMirrorCanceler{steps: &steps, err: errors.New("mirror unavailable")},
	)

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{RepositoryID: 9}})
	require.ErrorContains(t, err, "cancel repository mirror task")
	require.Equal(t, []string{"mirror-find", "mirror-cancel"}, steps)
}
