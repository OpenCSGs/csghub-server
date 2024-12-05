// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockSpaceResourceStore is an autogenerated mock type for the SpaceResourceStore type
type MockSpaceResourceStore struct {
	mock.Mock
}

type MockSpaceResourceStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSpaceResourceStore) EXPECT() *MockSpaceResourceStore_Expecter {
	return &MockSpaceResourceStore_Expecter{mock: &_m.Mock}
}

// Create provides a mock function with given fields: ctx, input
func (_m *MockSpaceResourceStore) Create(ctx context.Context, input database.SpaceResource) (*database.SpaceResource, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *database.SpaceResource
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceResource) (*database.SpaceResource, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceResource) *database.SpaceResource); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceResource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.SpaceResource) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceResourceStore_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockSpaceResourceStore_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.SpaceResource
func (_e *MockSpaceResourceStore_Expecter) Create(ctx interface{}, input interface{}) *MockSpaceResourceStore_Create_Call {
	return &MockSpaceResourceStore_Create_Call{Call: _e.mock.On("Create", ctx, input)}
}

func (_c *MockSpaceResourceStore_Create_Call) Run(run func(ctx context.Context, input database.SpaceResource)) *MockSpaceResourceStore_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.SpaceResource))
	})
	return _c
}

func (_c *MockSpaceResourceStore_Create_Call) Return(_a0 *database.SpaceResource, _a1 error) *MockSpaceResourceStore_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceResourceStore_Create_Call) RunAndReturn(run func(context.Context, database.SpaceResource) (*database.SpaceResource, error)) *MockSpaceResourceStore_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, input
func (_m *MockSpaceResourceStore) Delete(ctx context.Context, input database.SpaceResource) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceResource) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceResourceStore_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockSpaceResourceStore_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.SpaceResource
func (_e *MockSpaceResourceStore_Expecter) Delete(ctx interface{}, input interface{}) *MockSpaceResourceStore_Delete_Call {
	return &MockSpaceResourceStore_Delete_Call{Call: _e.mock.On("Delete", ctx, input)}
}

func (_c *MockSpaceResourceStore_Delete_Call) Run(run func(ctx context.Context, input database.SpaceResource)) *MockSpaceResourceStore_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.SpaceResource))
	})
	return _c
}

func (_c *MockSpaceResourceStore_Delete_Call) Return(_a0 error) *MockSpaceResourceStore_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceResourceStore_Delete_Call) RunAndReturn(run func(context.Context, database.SpaceResource) error) *MockSpaceResourceStore_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// FindAll provides a mock function with given fields: ctx
func (_m *MockSpaceResourceStore) FindAll(ctx context.Context) ([]database.SpaceResource, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for FindAll")
	}

	var r0 []database.SpaceResource
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]database.SpaceResource, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []database.SpaceResource); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.SpaceResource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceResourceStore_FindAll_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindAll'
type MockSpaceResourceStore_FindAll_Call struct {
	*mock.Call
}

// FindAll is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockSpaceResourceStore_Expecter) FindAll(ctx interface{}) *MockSpaceResourceStore_FindAll_Call {
	return &MockSpaceResourceStore_FindAll_Call{Call: _e.mock.On("FindAll", ctx)}
}

func (_c *MockSpaceResourceStore_FindAll_Call) Run(run func(ctx context.Context)) *MockSpaceResourceStore_FindAll_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockSpaceResourceStore_FindAll_Call) Return(_a0 []database.SpaceResource, _a1 error) *MockSpaceResourceStore_FindAll_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceResourceStore_FindAll_Call) RunAndReturn(run func(context.Context) ([]database.SpaceResource, error)) *MockSpaceResourceStore_FindAll_Call {
	_c.Call.Return(run)
	return _c
}

// FindByID provides a mock function with given fields: ctx, id
func (_m *MockSpaceResourceStore) FindByID(ctx context.Context, id int64) (*database.SpaceResource, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for FindByID")
	}

	var r0 *database.SpaceResource
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.SpaceResource, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.SpaceResource); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceResource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceResourceStore_FindByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByID'
type MockSpaceResourceStore_FindByID_Call struct {
	*mock.Call
}

// FindByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockSpaceResourceStore_Expecter) FindByID(ctx interface{}, id interface{}) *MockSpaceResourceStore_FindByID_Call {
	return &MockSpaceResourceStore_FindByID_Call{Call: _e.mock.On("FindByID", ctx, id)}
}

func (_c *MockSpaceResourceStore_FindByID_Call) Run(run func(ctx context.Context, id int64)) *MockSpaceResourceStore_FindByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockSpaceResourceStore_FindByID_Call) Return(_a0 *database.SpaceResource, _a1 error) *MockSpaceResourceStore_FindByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceResourceStore_FindByID_Call) RunAndReturn(run func(context.Context, int64) (*database.SpaceResource, error)) *MockSpaceResourceStore_FindByID_Call {
	_c.Call.Return(run)
	return _c
}

// FindByName provides a mock function with given fields: ctx, name
func (_m *MockSpaceResourceStore) FindByName(ctx context.Context, name string) (*database.SpaceResource, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for FindByName")
	}

	var r0 *database.SpaceResource
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*database.SpaceResource, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *database.SpaceResource); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceResource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceResourceStore_FindByName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByName'
type MockSpaceResourceStore_FindByName_Call struct {
	*mock.Call
}

// FindByName is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
func (_e *MockSpaceResourceStore_Expecter) FindByName(ctx interface{}, name interface{}) *MockSpaceResourceStore_FindByName_Call {
	return &MockSpaceResourceStore_FindByName_Call{Call: _e.mock.On("FindByName", ctx, name)}
}

func (_c *MockSpaceResourceStore_FindByName_Call) Run(run func(ctx context.Context, name string)) *MockSpaceResourceStore_FindByName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockSpaceResourceStore_FindByName_Call) Return(_a0 *database.SpaceResource, _a1 error) *MockSpaceResourceStore_FindByName_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceResourceStore_FindByName_Call) RunAndReturn(run func(context.Context, string) (*database.SpaceResource, error)) *MockSpaceResourceStore_FindByName_Call {
	_c.Call.Return(run)
	return _c
}

// Index provides a mock function with given fields: ctx, clusterId
func (_m *MockSpaceResourceStore) Index(ctx context.Context, clusterId string) ([]database.SpaceResource, error) {
	ret := _m.Called(ctx, clusterId)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 []database.SpaceResource
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]database.SpaceResource, error)); ok {
		return rf(ctx, clusterId)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []database.SpaceResource); ok {
		r0 = rf(ctx, clusterId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.SpaceResource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, clusterId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceResourceStore_Index_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Index'
type MockSpaceResourceStore_Index_Call struct {
	*mock.Call
}

// Index is a helper method to define mock.On call
//   - ctx context.Context
//   - clusterId string
func (_e *MockSpaceResourceStore_Expecter) Index(ctx interface{}, clusterId interface{}) *MockSpaceResourceStore_Index_Call {
	return &MockSpaceResourceStore_Index_Call{Call: _e.mock.On("Index", ctx, clusterId)}
}

func (_c *MockSpaceResourceStore_Index_Call) Run(run func(ctx context.Context, clusterId string)) *MockSpaceResourceStore_Index_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockSpaceResourceStore_Index_Call) Return(_a0 []database.SpaceResource, _a1 error) *MockSpaceResourceStore_Index_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceResourceStore_Index_Call) RunAndReturn(run func(context.Context, string) ([]database.SpaceResource, error)) *MockSpaceResourceStore_Index_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, input
func (_m *MockSpaceResourceStore) Update(ctx context.Context, input database.SpaceResource) (*database.SpaceResource, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *database.SpaceResource
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceResource) (*database.SpaceResource, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceResource) *database.SpaceResource); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceResource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.SpaceResource) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceResourceStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockSpaceResourceStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.SpaceResource
func (_e *MockSpaceResourceStore_Expecter) Update(ctx interface{}, input interface{}) *MockSpaceResourceStore_Update_Call {
	return &MockSpaceResourceStore_Update_Call{Call: _e.mock.On("Update", ctx, input)}
}

func (_c *MockSpaceResourceStore_Update_Call) Run(run func(ctx context.Context, input database.SpaceResource)) *MockSpaceResourceStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.SpaceResource))
	})
	return _c
}

func (_c *MockSpaceResourceStore_Update_Call) Return(_a0 *database.SpaceResource, _a1 error) *MockSpaceResourceStore_Update_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceResourceStore_Update_Call) RunAndReturn(run func(context.Context, database.SpaceResource) (*database.SpaceResource, error)) *MockSpaceResourceStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSpaceResourceStore creates a new instance of MockSpaceResourceStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSpaceResourceStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSpaceResourceStore {
	mock := &MockSpaceResourceStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
