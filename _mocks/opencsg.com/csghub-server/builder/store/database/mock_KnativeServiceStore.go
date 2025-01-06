// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockKnativeServiceStore is an autogenerated mock type for the KnativeServiceStore type
type MockKnativeServiceStore struct {
	mock.Mock
}

type MockKnativeServiceStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockKnativeServiceStore) EXPECT() *MockKnativeServiceStore_Expecter {
	return &MockKnativeServiceStore_Expecter{mock: &_m.Mock}
}

// Add provides a mock function with given fields: ctx, service
func (_m *MockKnativeServiceStore) Add(ctx context.Context, service *database.KnativeService) error {
	ret := _m.Called(ctx, service)

	if len(ret) == 0 {
		panic("no return value specified for Add")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *database.KnativeService) error); ok {
		r0 = rf(ctx, service)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockKnativeServiceStore_Add_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Add'
type MockKnativeServiceStore_Add_Call struct {
	*mock.Call
}

// Add is a helper method to define mock.On call
//   - ctx context.Context
//   - service *database.KnativeService
func (_e *MockKnativeServiceStore_Expecter) Add(ctx interface{}, service interface{}) *MockKnativeServiceStore_Add_Call {
	return &MockKnativeServiceStore_Add_Call{Call: _e.mock.On("Add", ctx, service)}
}

func (_c *MockKnativeServiceStore_Add_Call) Run(run func(ctx context.Context, service *database.KnativeService)) *MockKnativeServiceStore_Add_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*database.KnativeService))
	})
	return _c
}

func (_c *MockKnativeServiceStore_Add_Call) Return(_a0 error) *MockKnativeServiceStore_Add_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockKnativeServiceStore_Add_Call) RunAndReturn(run func(context.Context, *database.KnativeService) error) *MockKnativeServiceStore_Add_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, svcName, clusterID
func (_m *MockKnativeServiceStore) Delete(ctx context.Context, svcName string, clusterID string) error {
	ret := _m.Called(ctx, svcName, clusterID)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, svcName, clusterID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockKnativeServiceStore_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockKnativeServiceStore_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - svcName string
//   - clusterID string
func (_e *MockKnativeServiceStore_Expecter) Delete(ctx interface{}, svcName interface{}, clusterID interface{}) *MockKnativeServiceStore_Delete_Call {
	return &MockKnativeServiceStore_Delete_Call{Call: _e.mock.On("Delete", ctx, svcName, clusterID)}
}

func (_c *MockKnativeServiceStore_Delete_Call) Run(run func(ctx context.Context, svcName string, clusterID string)) *MockKnativeServiceStore_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockKnativeServiceStore_Delete_Call) Return(_a0 error) *MockKnativeServiceStore_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockKnativeServiceStore_Delete_Call) RunAndReturn(run func(context.Context, string, string) error) *MockKnativeServiceStore_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// Get provides a mock function with given fields: ctx, svcName, clusterID
func (_m *MockKnativeServiceStore) Get(ctx context.Context, svcName string, clusterID string) (*database.KnativeService, error) {
	ret := _m.Called(ctx, svcName, clusterID)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 *database.KnativeService
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*database.KnativeService, error)); ok {
		return rf(ctx, svcName, clusterID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *database.KnativeService); ok {
		r0 = rf(ctx, svcName, clusterID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.KnativeService)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, svcName, clusterID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockKnativeServiceStore_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type MockKnativeServiceStore_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - ctx context.Context
//   - svcName string
//   - clusterID string
func (_e *MockKnativeServiceStore_Expecter) Get(ctx interface{}, svcName interface{}, clusterID interface{}) *MockKnativeServiceStore_Get_Call {
	return &MockKnativeServiceStore_Get_Call{Call: _e.mock.On("Get", ctx, svcName, clusterID)}
}

func (_c *MockKnativeServiceStore_Get_Call) Run(run func(ctx context.Context, svcName string, clusterID string)) *MockKnativeServiceStore_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockKnativeServiceStore_Get_Call) Return(_a0 *database.KnativeService, _a1 error) *MockKnativeServiceStore_Get_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockKnativeServiceStore_Get_Call) RunAndReturn(run func(context.Context, string, string) (*database.KnativeService, error)) *MockKnativeServiceStore_Get_Call {
	_c.Call.Return(run)
	return _c
}

// GetByCluster provides a mock function with given fields: ctx, clusterID
func (_m *MockKnativeServiceStore) GetByCluster(ctx context.Context, clusterID string) ([]database.KnativeService, error) {
	ret := _m.Called(ctx, clusterID)

	if len(ret) == 0 {
		panic("no return value specified for GetByCluster")
	}

	var r0 []database.KnativeService
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]database.KnativeService, error)); ok {
		return rf(ctx, clusterID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []database.KnativeService); ok {
		r0 = rf(ctx, clusterID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.KnativeService)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, clusterID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockKnativeServiceStore_GetByCluster_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetByCluster'
type MockKnativeServiceStore_GetByCluster_Call struct {
	*mock.Call
}

// GetByCluster is a helper method to define mock.On call
//   - ctx context.Context
//   - clusterID string
func (_e *MockKnativeServiceStore_Expecter) GetByCluster(ctx interface{}, clusterID interface{}) *MockKnativeServiceStore_GetByCluster_Call {
	return &MockKnativeServiceStore_GetByCluster_Call{Call: _e.mock.On("GetByCluster", ctx, clusterID)}
}

func (_c *MockKnativeServiceStore_GetByCluster_Call) Run(run func(ctx context.Context, clusterID string)) *MockKnativeServiceStore_GetByCluster_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockKnativeServiceStore_GetByCluster_Call) Return(_a0 []database.KnativeService, _a1 error) *MockKnativeServiceStore_GetByCluster_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockKnativeServiceStore_GetByCluster_Call) RunAndReturn(run func(context.Context, string) ([]database.KnativeService, error)) *MockKnativeServiceStore_GetByCluster_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, service
func (_m *MockKnativeServiceStore) Update(ctx context.Context, service *database.KnativeService) error {
	ret := _m.Called(ctx, service)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *database.KnativeService) error); ok {
		r0 = rf(ctx, service)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockKnativeServiceStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockKnativeServiceStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - service *database.KnativeService
func (_e *MockKnativeServiceStore_Expecter) Update(ctx interface{}, service interface{}) *MockKnativeServiceStore_Update_Call {
	return &MockKnativeServiceStore_Update_Call{Call: _e.mock.On("Update", ctx, service)}
}

func (_c *MockKnativeServiceStore_Update_Call) Run(run func(ctx context.Context, service *database.KnativeService)) *MockKnativeServiceStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*database.KnativeService))
	})
	return _c
}

func (_c *MockKnativeServiceStore_Update_Call) Return(_a0 error) *MockKnativeServiceStore_Update_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockKnativeServiceStore_Update_Call) RunAndReturn(run func(context.Context, *database.KnativeService) error) *MockKnativeServiceStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockKnativeServiceStore creates a new instance of MockKnativeServiceStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockKnativeServiceStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockKnativeServiceStore {
	mock := &MockKnativeServiceStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}