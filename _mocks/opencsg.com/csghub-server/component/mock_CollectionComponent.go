// Code generated by mockery v2.49.1. DO NOT EDIT.

package component

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"

	types "opencsg.com/csghub-server/common/types"
)

// MockCollectionComponent is an autogenerated mock type for the CollectionComponent type
type MockCollectionComponent struct {
	mock.Mock
}

type MockCollectionComponent_Expecter struct {
	mock *mock.Mock
}

func (_m *MockCollectionComponent) EXPECT() *MockCollectionComponent_Expecter {
	return &MockCollectionComponent_Expecter{mock: &_m.Mock}
}

// AddReposToCollection provides a mock function with given fields: ctx, req
func (_m *MockCollectionComponent) AddReposToCollection(ctx context.Context, req types.UpdateCollectionReposReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for AddReposToCollection")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, types.UpdateCollectionReposReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCollectionComponent_AddReposToCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AddReposToCollection'
type MockCollectionComponent_AddReposToCollection_Call struct {
	*mock.Call
}

// AddReposToCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - req types.UpdateCollectionReposReq
func (_e *MockCollectionComponent_Expecter) AddReposToCollection(ctx interface{}, req interface{}) *MockCollectionComponent_AddReposToCollection_Call {
	return &MockCollectionComponent_AddReposToCollection_Call{Call: _e.mock.On("AddReposToCollection", ctx, req)}
}

func (_c *MockCollectionComponent_AddReposToCollection_Call) Run(run func(ctx context.Context, req types.UpdateCollectionReposReq)) *MockCollectionComponent_AddReposToCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.UpdateCollectionReposReq))
	})
	return _c
}

func (_c *MockCollectionComponent_AddReposToCollection_Call) Return(_a0 error) *MockCollectionComponent_AddReposToCollection_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCollectionComponent_AddReposToCollection_Call) RunAndReturn(run func(context.Context, types.UpdateCollectionReposReq) error) *MockCollectionComponent_AddReposToCollection_Call {
	_c.Call.Return(run)
	return _c
}

// CreateCollection provides a mock function with given fields: ctx, input
func (_m *MockCollectionComponent) CreateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for CreateCollection")
	}

	var r0 *database.Collection
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.CreateCollectionReq) (*database.Collection, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.CreateCollectionReq) *database.Collection); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.CreateCollectionReq) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCollectionComponent_CreateCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateCollection'
type MockCollectionComponent_CreateCollection_Call struct {
	*mock.Call
}

// CreateCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - input types.CreateCollectionReq
func (_e *MockCollectionComponent_Expecter) CreateCollection(ctx interface{}, input interface{}) *MockCollectionComponent_CreateCollection_Call {
	return &MockCollectionComponent_CreateCollection_Call{Call: _e.mock.On("CreateCollection", ctx, input)}
}

func (_c *MockCollectionComponent_CreateCollection_Call) Run(run func(ctx context.Context, input types.CreateCollectionReq)) *MockCollectionComponent_CreateCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.CreateCollectionReq))
	})
	return _c
}

func (_c *MockCollectionComponent_CreateCollection_Call) Return(_a0 *database.Collection, _a1 error) *MockCollectionComponent_CreateCollection_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCollectionComponent_CreateCollection_Call) RunAndReturn(run func(context.Context, types.CreateCollectionReq) (*database.Collection, error)) *MockCollectionComponent_CreateCollection_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteCollection provides a mock function with given fields: ctx, id, userName
func (_m *MockCollectionComponent) DeleteCollection(ctx context.Context, id int64, userName string) error {
	ret := _m.Called(ctx, id, userName)

	if len(ret) == 0 {
		panic("no return value specified for DeleteCollection")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) error); ok {
		r0 = rf(ctx, id, userName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCollectionComponent_DeleteCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteCollection'
type MockCollectionComponent_DeleteCollection_Call struct {
	*mock.Call
}

// DeleteCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
//   - userName string
func (_e *MockCollectionComponent_Expecter) DeleteCollection(ctx interface{}, id interface{}, userName interface{}) *MockCollectionComponent_DeleteCollection_Call {
	return &MockCollectionComponent_DeleteCollection_Call{Call: _e.mock.On("DeleteCollection", ctx, id, userName)}
}

func (_c *MockCollectionComponent_DeleteCollection_Call) Run(run func(ctx context.Context, id int64, userName string)) *MockCollectionComponent_DeleteCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64), args[2].(string))
	})
	return _c
}

func (_c *MockCollectionComponent_DeleteCollection_Call) Return(_a0 error) *MockCollectionComponent_DeleteCollection_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCollectionComponent_DeleteCollection_Call) RunAndReturn(run func(context.Context, int64, string) error) *MockCollectionComponent_DeleteCollection_Call {
	_c.Call.Return(run)
	return _c
}

// GetCollection provides a mock function with given fields: ctx, currentUser, id
func (_m *MockCollectionComponent) GetCollection(ctx context.Context, currentUser string, id int64) (*types.Collection, error) {
	ret := _m.Called(ctx, currentUser, id)

	if len(ret) == 0 {
		panic("no return value specified for GetCollection")
	}

	var r0 *types.Collection
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) (*types.Collection, error)); ok {
		return rf(ctx, currentUser, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) *types.Collection); ok {
		r0 = rf(ctx, currentUser, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, int64) error); ok {
		r1 = rf(ctx, currentUser, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCollectionComponent_GetCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCollection'
type MockCollectionComponent_GetCollection_Call struct {
	*mock.Call
}

// GetCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - currentUser string
//   - id int64
func (_e *MockCollectionComponent_Expecter) GetCollection(ctx interface{}, currentUser interface{}, id interface{}) *MockCollectionComponent_GetCollection_Call {
	return &MockCollectionComponent_GetCollection_Call{Call: _e.mock.On("GetCollection", ctx, currentUser, id)}
}

func (_c *MockCollectionComponent_GetCollection_Call) Run(run func(ctx context.Context, currentUser string, id int64)) *MockCollectionComponent_GetCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(int64))
	})
	return _c
}

func (_c *MockCollectionComponent_GetCollection_Call) Return(_a0 *types.Collection, _a1 error) *MockCollectionComponent_GetCollection_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCollectionComponent_GetCollection_Call) RunAndReturn(run func(context.Context, string, int64) (*types.Collection, error)) *MockCollectionComponent_GetCollection_Call {
	_c.Call.Return(run)
	return _c
}

// GetCollections provides a mock function with given fields: ctx, filter, per, page
func (_m *MockCollectionComponent) GetCollections(ctx context.Context, filter *types.CollectionFilter, per int, page int) ([]types.Collection, int, error) {
	ret := _m.Called(ctx, filter, per, page)

	if len(ret) == 0 {
		panic("no return value specified for GetCollections")
	}

	var r0 []types.Collection
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.CollectionFilter, int, int) ([]types.Collection, int, error)); ok {
		return rf(ctx, filter, per, page)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.CollectionFilter, int, int) []types.Collection); ok {
		r0 = rf(ctx, filter, per, page)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.CollectionFilter, int, int) int); ok {
		r1 = rf(ctx, filter, per, page)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.CollectionFilter, int, int) error); ok {
		r2 = rf(ctx, filter, per, page)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockCollectionComponent_GetCollections_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCollections'
type MockCollectionComponent_GetCollections_Call struct {
	*mock.Call
}

// GetCollections is a helper method to define mock.On call
//   - ctx context.Context
//   - filter *types.CollectionFilter
//   - per int
//   - page int
func (_e *MockCollectionComponent_Expecter) GetCollections(ctx interface{}, filter interface{}, per interface{}, page interface{}) *MockCollectionComponent_GetCollections_Call {
	return &MockCollectionComponent_GetCollections_Call{Call: _e.mock.On("GetCollections", ctx, filter, per, page)}
}

func (_c *MockCollectionComponent_GetCollections_Call) Run(run func(ctx context.Context, filter *types.CollectionFilter, per int, page int)) *MockCollectionComponent_GetCollections_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.CollectionFilter), args[2].(int), args[3].(int))
	})
	return _c
}

func (_c *MockCollectionComponent_GetCollections_Call) Return(_a0 []types.Collection, _a1 int, _a2 error) *MockCollectionComponent_GetCollections_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockCollectionComponent_GetCollections_Call) RunAndReturn(run func(context.Context, *types.CollectionFilter, int, int) ([]types.Collection, int, error)) *MockCollectionComponent_GetCollections_Call {
	_c.Call.Return(run)
	return _c
}

// GetPublicRepos provides a mock function with given fields: collection
func (_m *MockCollectionComponent) GetPublicRepos(collection types.Collection) []types.CollectionRepository {
	ret := _m.Called(collection)

	if len(ret) == 0 {
		panic("no return value specified for GetPublicRepos")
	}

	var r0 []types.CollectionRepository
	if rf, ok := ret.Get(0).(func(types.Collection) []types.CollectionRepository); ok {
		r0 = rf(collection)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.CollectionRepository)
		}
	}

	return r0
}

// MockCollectionComponent_GetPublicRepos_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetPublicRepos'
type MockCollectionComponent_GetPublicRepos_Call struct {
	*mock.Call
}

// GetPublicRepos is a helper method to define mock.On call
//   - collection types.Collection
func (_e *MockCollectionComponent_Expecter) GetPublicRepos(collection interface{}) *MockCollectionComponent_GetPublicRepos_Call {
	return &MockCollectionComponent_GetPublicRepos_Call{Call: _e.mock.On("GetPublicRepos", collection)}
}

func (_c *MockCollectionComponent_GetPublicRepos_Call) Run(run func(collection types.Collection)) *MockCollectionComponent_GetPublicRepos_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(types.Collection))
	})
	return _c
}

func (_c *MockCollectionComponent_GetPublicRepos_Call) Return(_a0 []types.CollectionRepository) *MockCollectionComponent_GetPublicRepos_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCollectionComponent_GetPublicRepos_Call) RunAndReturn(run func(types.Collection) []types.CollectionRepository) *MockCollectionComponent_GetPublicRepos_Call {
	_c.Call.Return(run)
	return _c
}

// OrgCollections provides a mock function with given fields: ctx, req
func (_m *MockCollectionComponent) OrgCollections(ctx context.Context, req *types.OrgCollectionsReq) ([]types.Collection, int, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for OrgCollections")
	}

	var r0 []types.Collection
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.OrgCollectionsReq) ([]types.Collection, int, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.OrgCollectionsReq) []types.Collection); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.OrgCollectionsReq) int); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.OrgCollectionsReq) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockCollectionComponent_OrgCollections_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'OrgCollections'
type MockCollectionComponent_OrgCollections_Call struct {
	*mock.Call
}

// OrgCollections is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.OrgCollectionsReq
func (_e *MockCollectionComponent_Expecter) OrgCollections(ctx interface{}, req interface{}) *MockCollectionComponent_OrgCollections_Call {
	return &MockCollectionComponent_OrgCollections_Call{Call: _e.mock.On("OrgCollections", ctx, req)}
}

func (_c *MockCollectionComponent_OrgCollections_Call) Run(run func(ctx context.Context, req *types.OrgCollectionsReq)) *MockCollectionComponent_OrgCollections_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.OrgCollectionsReq))
	})
	return _c
}

func (_c *MockCollectionComponent_OrgCollections_Call) Return(_a0 []types.Collection, _a1 int, _a2 error) *MockCollectionComponent_OrgCollections_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockCollectionComponent_OrgCollections_Call) RunAndReturn(run func(context.Context, *types.OrgCollectionsReq) ([]types.Collection, int, error)) *MockCollectionComponent_OrgCollections_Call {
	_c.Call.Return(run)
	return _c
}

// RemoveReposFromCollection provides a mock function with given fields: ctx, req
func (_m *MockCollectionComponent) RemoveReposFromCollection(ctx context.Context, req types.UpdateCollectionReposReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for RemoveReposFromCollection")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, types.UpdateCollectionReposReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCollectionComponent_RemoveReposFromCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RemoveReposFromCollection'
type MockCollectionComponent_RemoveReposFromCollection_Call struct {
	*mock.Call
}

// RemoveReposFromCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - req types.UpdateCollectionReposReq
func (_e *MockCollectionComponent_Expecter) RemoveReposFromCollection(ctx interface{}, req interface{}) *MockCollectionComponent_RemoveReposFromCollection_Call {
	return &MockCollectionComponent_RemoveReposFromCollection_Call{Call: _e.mock.On("RemoveReposFromCollection", ctx, req)}
}

func (_c *MockCollectionComponent_RemoveReposFromCollection_Call) Run(run func(ctx context.Context, req types.UpdateCollectionReposReq)) *MockCollectionComponent_RemoveReposFromCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.UpdateCollectionReposReq))
	})
	return _c
}

func (_c *MockCollectionComponent_RemoveReposFromCollection_Call) Return(_a0 error) *MockCollectionComponent_RemoveReposFromCollection_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCollectionComponent_RemoveReposFromCollection_Call) RunAndReturn(run func(context.Context, types.UpdateCollectionReposReq) error) *MockCollectionComponent_RemoveReposFromCollection_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateCollection provides a mock function with given fields: ctx, input
func (_m *MockCollectionComponent) UpdateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for UpdateCollection")
	}

	var r0 *database.Collection
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.CreateCollectionReq) (*database.Collection, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.CreateCollectionReq) *database.Collection); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.CreateCollectionReq) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCollectionComponent_UpdateCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateCollection'
type MockCollectionComponent_UpdateCollection_Call struct {
	*mock.Call
}

// UpdateCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - input types.CreateCollectionReq
func (_e *MockCollectionComponent_Expecter) UpdateCollection(ctx interface{}, input interface{}) *MockCollectionComponent_UpdateCollection_Call {
	return &MockCollectionComponent_UpdateCollection_Call{Call: _e.mock.On("UpdateCollection", ctx, input)}
}

func (_c *MockCollectionComponent_UpdateCollection_Call) Run(run func(ctx context.Context, input types.CreateCollectionReq)) *MockCollectionComponent_UpdateCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.CreateCollectionReq))
	})
	return _c
}

func (_c *MockCollectionComponent_UpdateCollection_Call) Return(_a0 *database.Collection, _a1 error) *MockCollectionComponent_UpdateCollection_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCollectionComponent_UpdateCollection_Call) RunAndReturn(run func(context.Context, types.CreateCollectionReq) (*database.Collection, error)) *MockCollectionComponent_UpdateCollection_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockCollectionComponent creates a new instance of MockCollectionComponent. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockCollectionComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCollectionComponent {
	mock := &MockCollectionComponent{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
