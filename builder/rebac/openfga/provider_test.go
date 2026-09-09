package openfga

import (
	"context"
	"testing"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

type fakeOpenFGAServer struct {
	requests           []*openfgav1.WriteRequest
	checkRequest       *openfgav1.CheckRequest
	batchCheckRequest  *openfgav1.BatchCheckRequest
	listObjectsRequest *openfgav1.ListObjectsRequest
	listUsersRequest   *openfgav1.ListUsersRequest
	checkResponse      *openfgav1.CheckResponse
	batchResponse      *openfgav1.BatchCheckResponse
	objectsResponse    *openfgav1.ListObjectsResponse
	usersResponse      *openfgav1.ListUsersResponse
}

func (s *fakeOpenFGAServer) Write(_ context.Context, request *openfgav1.WriteRequest) (*openfgav1.WriteResponse, error) {
	s.requests = append(s.requests, request)
	return &openfgav1.WriteResponse{}, nil
}

func (s *fakeOpenFGAServer) Check(_ context.Context, request *openfgav1.CheckRequest) (*openfgav1.CheckResponse, error) {
	s.checkRequest = request
	return s.checkResponse, nil
}

func (s *fakeOpenFGAServer) BatchCheck(_ context.Context, request *openfgav1.BatchCheckRequest) (*openfgav1.BatchCheckResponse, error) {
	s.batchCheckRequest = request
	return s.batchResponse, nil
}

func (s *fakeOpenFGAServer) ListObjects(_ context.Context, request *openfgav1.ListObjectsRequest) (*openfgav1.ListObjectsResponse, error) {
	s.listObjectsRequest = request
	return s.objectsResponse, nil
}

func (s *fakeOpenFGAServer) ListUsers(_ context.Context, request *openfgav1.ListUsersRequest) (*openfgav1.ListUsersResponse, error) {
	s.listUsersRequest = request
	return s.usersResponse, nil
}

func (*fakeOpenFGAServer) Close() {}

// TestProviderName verifies the stable provider name without requiring a database.
func TestProviderName(t *testing.T) {
	provider := &Provider{}
	require.Equal(t, "openfga", provider.Name())
}

// TestProviderWriteAndDeleteUseOpenFGAWrite verifies tuple mutation request mappings.
func TestProviderWriteAndDeleteUseOpenFGAWrite(t *testing.T) {
	server := &fakeOpenFGAServer{}
	provider := &Provider{server: server}
	relationships := []rebac.Relationship{{
		Subject:  rebac.UserSubject("user-1"),
		Relation: rebac.RelationReader,
		Object:   rebac.RepositoryObject(42),
	}}

	require.NoError(t, provider.Write(context.Background(), relationships))
	require.NoError(t, provider.Delete(context.Background(), relationships))
	require.Len(t, server.requests, 2)

	writeRequest := server.requests[0]
	require.Equal(t, commontypes.OpenFgaStoreID, writeRequest.GetStoreId())
	require.Equal(t, commontypes.OpenFgaAuthorizationModelID, writeRequest.GetAuthorizationModelId())
	require.Nil(t, writeRequest.GetDeletes())
	require.Len(t, writeRequest.GetWrites().GetTupleKeys(), 1)
	require.Equal(t, "user:user-1", writeRequest.GetWrites().GetTupleKeys()[0].GetUser())
	require.Equal(t, "reader", writeRequest.GetWrites().GetTupleKeys()[0].GetRelation())
	require.Equal(t, "repository:42", writeRequest.GetWrites().GetTupleKeys()[0].GetObject())

	deleteRequest := server.requests[1]
	require.Nil(t, deleteRequest.GetWrites())
	require.Len(t, deleteRequest.GetDeletes().GetTupleKeys(), 1)
	require.Equal(t, "user:user-1", deleteRequest.GetDeletes().GetTupleKeys()[0].GetUser())
	require.Equal(t, "reader", deleteRequest.GetDeletes().GetTupleKeys()[0].GetRelation())
	require.Equal(t, "repository:42", deleteRequest.GetDeletes().GetTupleKeys()[0].GetObject())
}

// TestProviderReadOperationsMapOpenFGARequests verifies read request and response mappings.
func TestProviderReadOperationsMapOpenFGARequests(t *testing.T) {
	server := &fakeOpenFGAServer{
		checkResponse: &openfgav1.CheckResponse{Allowed: true},
		batchResponse: &openfgav1.BatchCheckResponse{Result: map[string]*openfgav1.BatchCheckSingleResult{
			"allowed": {CheckResult: &openfgav1.BatchCheckSingleResult_Allowed{Allowed: true}},
			"denied":  {CheckResult: &openfgav1.BatchCheckSingleResult_Allowed{Allowed: false}},
			"error":   {CheckResult: &openfgav1.BatchCheckSingleResult_Error{Error: &openfgav1.CheckError{Message: "invalid tuple"}}},
		}},
		objectsResponse: &openfgav1.ListObjectsResponse{Objects: []string{"repository:42", "repository:43"}},
		usersResponse: &openfgav1.ListUsersResponse{Users: []*openfgav1.User{
			{User: &openfgav1.User_Object{Object: &openfgav1.Object{Type: "user", Id: "u-1"}}},
			{User: &openfgav1.User_Userset{Userset: &openfgav1.UsersetUser{Type: "organization", Id: "org-1", Relation: "member"}}},
			{User: &openfgav1.User_Wildcard{Wildcard: &openfgav1.TypedWildcard{Type: "user"}}},
		}},
	}
	provider := &Provider{server: server}
	relationship := rebac.Relationship{Subject: rebac.UserSubject("u-2"), Relation: rebac.RelationReader, Object: rebac.RepositoryObject(42)}

	decision, err := provider.Check(context.Background(), rebac.CheckRequest{
		Subject: rebac.UserSubject("u-1"), Relation: rebac.RepositoryCanRead, Object: rebac.RepositoryObject(42),
		ConditionContext: map[string]any{"tenant": "acme"}, ContextualRelationships: []rebac.Relationship{relationship}, Consistency: rebac.ConsistencyHigher,
	})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, "user:u-1", server.checkRequest.GetTupleKey().GetUser())
	require.Equal(t, "can_read", server.checkRequest.GetTupleKey().GetRelation())
	require.Equal(t, openfgav1.ConsistencyPreference_HIGHER_CONSISTENCY, server.checkRequest.GetConsistency())
	require.Equal(t, "acme", server.checkRequest.GetContext().GetFields()["tenant"].GetStringValue())
	require.Len(t, server.checkRequest.GetContextualTuples().GetTupleKeys(), 1)

	batch, err := provider.BatchCheck(context.Background(), rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{
		{CorrelationID: "allowed", Check: rebac.CheckRequest{Subject: rebac.UserSubject("u-1"), Relation: rebac.RepositoryCanRead, Object: rebac.RepositoryObject(42), Consistency: rebac.ConsistencyMinimizeLatency}},
		{CorrelationID: "denied", Check: rebac.CheckRequest{Subject: rebac.UserSubject("u-2"), Relation: rebac.RepositoryCanRead, Object: rebac.RepositoryObject(42)}},
		{CorrelationID: "error", Check: rebac.CheckRequest{Subject: rebac.UserSubject("u-3"), Relation: rebac.RepositoryCanRead, Object: rebac.RepositoryObject(42)}},
	}})
	require.NoError(t, err)
	require.True(t, batch.Results["allowed"].Decision.Allowed)
	require.False(t, batch.Results["denied"].Decision.Allowed)
	require.EqualError(t, batch.Results["error"].Err, "OpenFGA BatchCheck: invalid tuple")
	require.Equal(t, openfgav1.ConsistencyPreference_MINIMIZE_LATENCY, server.batchCheckRequest.GetConsistency())
	require.Len(t, server.batchCheckRequest.GetChecks(), 3)

	objects, err := provider.ListObjects(context.Background(), rebac.ListObjectsRequest{
		Subject: rebac.UserSubject("u-1"), Relation: rebac.RepositoryCanRead, ObjectType: rebac.ObjectTypeRepository,
		ContextualRelationships: []rebac.Relationship{relationship}, ConditionContext: map[string]any{"tenant": "acme"}, Consistency: rebac.ConsistencyHigher,
	})
	require.NoError(t, err)
	require.Equal(t, []rebac.Object{rebac.RepositoryObject(42), rebac.RepositoryObject(43)}, objects.Objects)
	require.Equal(t, "user:u-1", server.listObjectsRequest.GetUser())
	require.Equal(t, openfgav1.ConsistencyPreference_HIGHER_CONSISTENCY, server.listObjectsRequest.GetConsistency())

	subjects, err := provider.ListSubjects(context.Background(), rebac.ListSubjectsRequest{
		Object: rebac.RepositoryObject(42), Relation: rebac.RepositoryCanRead, SubjectType: rebac.ObjectTypeUser,
		ConditionContext: map[string]any{"tenant": "acme"}, Consistency: rebac.ConsistencyMinimizeLatency,
	})
	require.NoError(t, err)
	require.Equal(t, []rebac.Subject{
		rebac.UserSubject("u-1"), rebac.NewUserset(rebac.ObjectTypeOrganization, "org-1", rebac.RelationMember), rebac.PublicUserWildcard(),
	}, subjects.Subjects)
	require.Equal(t, "repository", server.listUsersRequest.GetObject().GetType())
	require.Equal(t, openfgav1.ConsistencyPreference_MINIMIZE_LATENCY, server.listUsersRequest.GetConsistency())
	require.Equal(t, "acme", server.listUsersRequest.GetContext().GetFields()["tenant"].GetStringValue())
}

// TestProviderReadOperationsRejectInvalidContext verifies protobuf context conversion errors.
func TestProviderReadOperationsRejectInvalidContext(t *testing.T) {
	provider := &Provider{server: &fakeOpenFGAServer{checkResponse: &openfgav1.CheckResponse{}}}
	_, err := provider.Check(context.Background(), rebac.CheckRequest{ConditionContext: map[string]any{"invalid": func() {}}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "OpenFGA request context")

	_, err = provider.ListObjects(context.Background(), rebac.ListObjectsRequest{ConditionContext: map[string]any{"invalid": func() {}}})
	require.Error(t, err)
	_, err = provider.ListSubjects(context.Background(), rebac.ListSubjectsRequest{ConditionContext: map[string]any{"invalid": func() {}}})
	require.Error(t, err)
}

// TestGetServerInitializationFailureCanRetry verifies failed server initialization is not cached as success.
func TestGetServerInitializationFailureCanRetry(t *testing.T) {
	previousDB := database.GetDB()
	database.SetDB(nil)
	t.Cleanup(func() { database.SetDB(previousDB) })

	server, err := getServer()
	require.Error(t, err)
	require.Nil(t, server)

	server, err = getServer()
	require.Error(t, err)
	require.Nil(t, server)
}

// TestNewProviderInitializationFailureCanRetry verifies failed provider initialization is not cached as success.
func TestNewProviderInitializationFailureCanRetry(t *testing.T) {
	previousDB := database.GetDB()
	database.SetDB(nil)
	t.Cleanup(func() { database.SetDB(previousDB) })

	provider, err := NewProvider()
	require.Error(t, err)
	require.Nil(t, provider)

	provider, err = NewProvider()
	require.Error(t, err)
	require.Nil(t, provider)
}

// TestNewProviderWithPGXPoolRejectsNil verifies the migration-specific provider entry point validates its dependency.
func TestNewProviderWithPGXPoolRejectsNil(t *testing.T) {
	provider, err := NewProviderWithPGXPool(nil)
	require.Error(t, err)
	require.Nil(t, provider)
}
