package rebac

import "context"

type mockProvider struct {
	name             string
	write            func(context.Context, []Relationship) error
	delete           func(context.Context, []Relationship) error
	check            func(context.Context, CheckRequest) (Decision, error)
	batchCheck       func(context.Context, BatchCheckRequest) (BatchCheckResult, error)
	listObjects      func(context.Context, ListObjectsRequest) (ListObjectsResult, error)
	listSubjects     func(context.Context, ListSubjectsRequest) (ListSubjectsResult, error)
	checkCalls       int
	batchCheckCalls  int
	listObjectCalls  int
	listSubjectCalls int
	writeCalls       int
	deleteCalls      int
}

func (m *mockProvider) Write(ctx context.Context, relationships []Relationship) error {
	m.writeCalls++
	if m.write == nil {
		return ErrUnsupported
	}
	return m.write(ctx, relationships)
}

func (m *mockProvider) Delete(ctx context.Context, relationships []Relationship) error {
	m.deleteCalls++
	if m.delete == nil {
		return ErrUnsupported
	}
	return m.delete(ctx, relationships)
}

func (m *mockProvider) Check(ctx context.Context, request CheckRequest) (Decision, error) {
	m.checkCalls++
	if m.check == nil {
		return Decision{}, ErrUnsupported
	}
	return m.check(ctx, request)
}

func (m *mockProvider) BatchCheck(ctx context.Context, request BatchCheckRequest) (BatchCheckResult, error) {
	m.batchCheckCalls++
	if m.batchCheck == nil {
		return BatchCheckResult{}, ErrUnsupported
	}
	return m.batchCheck(ctx, request)
}

func (m *mockProvider) ListObjects(ctx context.Context, request ListObjectsRequest) (ListObjectsResult, error) {
	m.listObjectCalls++
	if m.listObjects == nil {
		return ListObjectsResult{}, ErrUnsupported
	}
	return m.listObjects(ctx, request)
}

func (m *mockProvider) ListSubjects(ctx context.Context, request ListSubjectsRequest) (ListSubjectsResult, error) {
	m.listSubjectCalls++
	if m.listSubjects == nil {
		return ListSubjectsResult{}, ErrUnsupported
	}
	return m.listSubjects(ctx, request)
}

func (m *mockProvider) Name() string {
	return m.name
}

func validCheckRequest() CheckRequest {
	return CheckRequest{
		Subject:  UserSubject("user-uuid"),
		Relation: RepositoryCanRead,
		Object:   RepositoryObject(42),
	}
}
