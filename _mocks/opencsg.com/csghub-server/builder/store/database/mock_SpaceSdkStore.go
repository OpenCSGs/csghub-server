// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockSpaceSdkStore is an autogenerated mock type for the SpaceSdkStore type
type MockSpaceSdkStore struct {
	mock.Mock
}

type MockSpaceSdkStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSpaceSdkStore) EXPECT() *MockSpaceSdkStore_Expecter {
	return &MockSpaceSdkStore_Expecter{mock: &_m.Mock}
}

// Create provides a mock function with given fields: ctx, input
func (_m *MockSpaceSdkStore) Create(ctx context.Context, input database.SpaceSdk) (*database.SpaceSdk, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *database.SpaceSdk
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceSdk) (*database.SpaceSdk, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceSdk) *database.SpaceSdk); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceSdk)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.SpaceSdk) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceSdkStore_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockSpaceSdkStore_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.SpaceSdk
func (_e *MockSpaceSdkStore_Expecter) Create(ctx interface{}, input interface{}) *MockSpaceSdkStore_Create_Call {
	return &MockSpaceSdkStore_Create_Call{Call: _e.mock.On("Create", ctx, input)}
}

func (_c *MockSpaceSdkStore_Create_Call) Run(run func(ctx context.Context, input database.SpaceSdk)) *MockSpaceSdkStore_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.SpaceSdk))
	})
	return _c
}

func (_c *MockSpaceSdkStore_Create_Call) Return(_a0 *database.SpaceSdk, _a1 error) *MockSpaceSdkStore_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceSdkStore_Create_Call) RunAndReturn(run func(context.Context, database.SpaceSdk) (*database.SpaceSdk, error)) *MockSpaceSdkStore_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, input
func (_m *MockSpaceSdkStore) Delete(ctx context.Context, input database.SpaceSdk) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceSdk) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceSdkStore_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockSpaceSdkStore_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.SpaceSdk
func (_e *MockSpaceSdkStore_Expecter) Delete(ctx interface{}, input interface{}) *MockSpaceSdkStore_Delete_Call {
	return &MockSpaceSdkStore_Delete_Call{Call: _e.mock.On("Delete", ctx, input)}
}

func (_c *MockSpaceSdkStore_Delete_Call) Run(run func(ctx context.Context, input database.SpaceSdk)) *MockSpaceSdkStore_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.SpaceSdk))
	})
	return _c
}

func (_c *MockSpaceSdkStore_Delete_Call) Return(_a0 error) *MockSpaceSdkStore_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceSdkStore_Delete_Call) RunAndReturn(run func(context.Context, database.SpaceSdk) error) *MockSpaceSdkStore_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// FindByID provides a mock function with given fields: ctx, id
func (_m *MockSpaceSdkStore) FindByID(ctx context.Context, id int64) (*database.SpaceSdk, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for FindByID")
	}

	var r0 *database.SpaceSdk
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.SpaceSdk, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.SpaceSdk); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceSdk)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceSdkStore_FindByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByID'
type MockSpaceSdkStore_FindByID_Call struct {
	*mock.Call
}

// FindByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockSpaceSdkStore_Expecter) FindByID(ctx interface{}, id interface{}) *MockSpaceSdkStore_FindByID_Call {
	return &MockSpaceSdkStore_FindByID_Call{Call: _e.mock.On("FindByID", ctx, id)}
}

func (_c *MockSpaceSdkStore_FindByID_Call) Run(run func(ctx context.Context, id int64)) *MockSpaceSdkStore_FindByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockSpaceSdkStore_FindByID_Call) Return(_a0 *database.SpaceSdk, _a1 error) *MockSpaceSdkStore_FindByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceSdkStore_FindByID_Call) RunAndReturn(run func(context.Context, int64) (*database.SpaceSdk, error)) *MockSpaceSdkStore_FindByID_Call {
	_c.Call.Return(run)
	return _c
}

// Index provides a mock function with given fields: ctx
func (_m *MockSpaceSdkStore) Index(ctx context.Context) ([]database.SpaceSdk, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 []database.SpaceSdk
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]database.SpaceSdk, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []database.SpaceSdk); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.SpaceSdk)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceSdkStore_Index_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Index'
type MockSpaceSdkStore_Index_Call struct {
	*mock.Call
}

// Index is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockSpaceSdkStore_Expecter) Index(ctx interface{}) *MockSpaceSdkStore_Index_Call {
	return &MockSpaceSdkStore_Index_Call{Call: _e.mock.On("Index", ctx)}
}

func (_c *MockSpaceSdkStore_Index_Call) Run(run func(ctx context.Context)) *MockSpaceSdkStore_Index_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockSpaceSdkStore_Index_Call) Return(_a0 []database.SpaceSdk, _a1 error) *MockSpaceSdkStore_Index_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceSdkStore_Index_Call) RunAndReturn(run func(context.Context) ([]database.SpaceSdk, error)) *MockSpaceSdkStore_Index_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, input
func (_m *MockSpaceSdkStore) Update(ctx context.Context, input database.SpaceSdk) (*database.SpaceSdk, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *database.SpaceSdk
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceSdk) (*database.SpaceSdk, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.SpaceSdk) *database.SpaceSdk); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SpaceSdk)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.SpaceSdk) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceSdkStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockSpaceSdkStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.SpaceSdk
func (_e *MockSpaceSdkStore_Expecter) Update(ctx interface{}, input interface{}) *MockSpaceSdkStore_Update_Call {
	return &MockSpaceSdkStore_Update_Call{Call: _e.mock.On("Update", ctx, input)}
}

func (_c *MockSpaceSdkStore_Update_Call) Run(run func(ctx context.Context, input database.SpaceSdk)) *MockSpaceSdkStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.SpaceSdk))
	})
	return _c
}

func (_c *MockSpaceSdkStore_Update_Call) Return(_a0 *database.SpaceSdk, _a1 error) *MockSpaceSdkStore_Update_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceSdkStore_Update_Call) RunAndReturn(run func(context.Context, database.SpaceSdk) (*database.SpaceSdk, error)) *MockSpaceSdkStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSpaceSdkStore creates a new instance of MockSpaceSdkStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSpaceSdkStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSpaceSdkStore {
	mock := &MockSpaceSdkStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
