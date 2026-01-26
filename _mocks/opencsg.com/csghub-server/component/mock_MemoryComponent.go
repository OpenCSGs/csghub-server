// Code generated manually to mirror mockery patterns. DO NOT EDIT.

package component

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	memorytypes "opencsg.com/csghub-server/common/types"
)

// MockMemoryComponent is a mock for MemoryComponent.
type MockMemoryComponent struct {
	mock.Mock
}

func (m *MockMemoryComponent) Capabilities() memorytypes.MemoryCapabilities {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(memorytypes.MemoryCapabilities)
	}
	return memorytypes.MemoryCapabilities{}
}

func (m *MockMemoryComponent) CreateProject(ctx context.Context, req *memorytypes.CreateMemoryProjectRequest) (*memorytypes.MemoryProjectResponse, error) {
	ret := m.Called(ctx, req)
	var r0 *memorytypes.MemoryProjectResponse
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*memorytypes.MemoryProjectResponse)
	}
	return r0, ret.Error(1)
}

func (m *MockMemoryComponent) GetProject(ctx context.Context, req *memorytypes.GetMemoryProjectRequest) (*memorytypes.MemoryProjectResponse, error) {
	ret := m.Called(ctx, req)
	var r0 *memorytypes.MemoryProjectResponse
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*memorytypes.MemoryProjectResponse)
	}
	return r0, ret.Error(1)
}

func (m *MockMemoryComponent) ListProjects(ctx context.Context) ([]*memorytypes.MemoryProjectRef, error) {
	ret := m.Called(ctx)
	var r0 []*memorytypes.MemoryProjectRef
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]*memorytypes.MemoryProjectRef)
	}
	return r0, ret.Error(1)
}

func (m *MockMemoryComponent) DeleteProject(ctx context.Context, req *memorytypes.DeleteMemoryProjectRequest) error {
	ret := m.Called(ctx, req)
	return ret.Error(0)
}

func (m *MockMemoryComponent) AddMemories(ctx context.Context, req *memorytypes.AddMemoriesRequest) (*memorytypes.AddMemoriesResponse, error) {
	ret := m.Called(ctx, req)
	var r0 *memorytypes.AddMemoriesResponse
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*memorytypes.AddMemoriesResponse)
	}
	return r0, ret.Error(1)
}

func (m *MockMemoryComponent) SearchMemories(ctx context.Context, req *memorytypes.SearchMemoriesRequest) (*memorytypes.SearchMemoriesResponse, error) {
	ret := m.Called(ctx, req)
	var r0 *memorytypes.SearchMemoriesResponse
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*memorytypes.SearchMemoriesResponse)
	}
	return r0, ret.Error(1)
}

func (m *MockMemoryComponent) ListMemories(ctx context.Context, req *memorytypes.ListMemoriesRequest) (*memorytypes.ListMemoriesResponse, error) {
	ret := m.Called(ctx, req)
	var r0 *memorytypes.ListMemoriesResponse
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*memorytypes.ListMemoriesResponse)
	}
	return r0, ret.Error(1)
}

func (m *MockMemoryComponent) DeleteMemories(ctx context.Context, req *memorytypes.DeleteMemoriesRequest) error {
	ret := m.Called(ctx, req)
	return ret.Error(0)
}

func (m *MockMemoryComponent) Health(ctx context.Context) (*memorytypes.MemoryHealthResponse, error) {
	ret := m.Called(ctx)
	var r0 *memorytypes.MemoryHealthResponse
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*memorytypes.MemoryHealthResponse)
	}
	return r0, ret.Error(1)
}

// NewMockMemoryComponent creates a new instance of MockMemoryComponent.
func NewMockMemoryComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMemoryComponent {
	mockObj := &MockMemoryComponent{}
	mockObj.Mock.Test(t)

	t.Cleanup(func() { mockObj.AssertExpectations(t) })

	return mockObj
}
