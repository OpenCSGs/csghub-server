// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockClusterInfoStore is an autogenerated mock type for the ClusterInfoStore type
type MockClusterInfoStore struct {
	mock.Mock
}

type MockClusterInfoStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClusterInfoStore) EXPECT() *MockClusterInfoStore_Expecter {
	return &MockClusterInfoStore_Expecter{mock: &_m.Mock}
}

// Add provides a mock function with given fields: ctx, clusterConfig, region
func (_m *MockClusterInfoStore) Add(ctx context.Context, clusterConfig string, region string) error {
	ret := _m.Called(ctx, clusterConfig, region)

	if len(ret) == 0 {
		panic("no return value specified for Add")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, clusterConfig, region)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterInfoStore_Add_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Add'
type MockClusterInfoStore_Add_Call struct {
	*mock.Call
}

// Add is a helper method to define mock.On call
//   - ctx context.Context
//   - clusterConfig string
//   - region string
func (_e *MockClusterInfoStore_Expecter) Add(ctx interface{}, clusterConfig interface{}, region interface{}) *MockClusterInfoStore_Add_Call {
	return &MockClusterInfoStore_Add_Call{Call: _e.mock.On("Add", ctx, clusterConfig, region)}
}

func (_c *MockClusterInfoStore_Add_Call) Run(run func(ctx context.Context, clusterConfig string, region string)) *MockClusterInfoStore_Add_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockClusterInfoStore_Add_Call) Return(_a0 error) *MockClusterInfoStore_Add_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterInfoStore_Add_Call) RunAndReturn(run func(context.Context, string, string) error) *MockClusterInfoStore_Add_Call {
	_c.Call.Return(run)
	return _c
}

// ByClusterConfig provides a mock function with given fields: ctx, clusterConfig
func (_m *MockClusterInfoStore) ByClusterConfig(ctx context.Context, clusterConfig string) (database.ClusterInfo, error) {
	ret := _m.Called(ctx, clusterConfig)

	if len(ret) == 0 {
		panic("no return value specified for ByClusterConfig")
	}

	var r0 database.ClusterInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (database.ClusterInfo, error)); ok {
		return rf(ctx, clusterConfig)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) database.ClusterInfo); ok {
		r0 = rf(ctx, clusterConfig)
	} else {
		r0 = ret.Get(0).(database.ClusterInfo)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, clusterConfig)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClusterInfoStore_ByClusterConfig_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByClusterConfig'
type MockClusterInfoStore_ByClusterConfig_Call struct {
	*mock.Call
}

// ByClusterConfig is a helper method to define mock.On call
//   - ctx context.Context
//   - clusterConfig string
func (_e *MockClusterInfoStore_Expecter) ByClusterConfig(ctx interface{}, clusterConfig interface{}) *MockClusterInfoStore_ByClusterConfig_Call {
	return &MockClusterInfoStore_ByClusterConfig_Call{Call: _e.mock.On("ByClusterConfig", ctx, clusterConfig)}
}

func (_c *MockClusterInfoStore_ByClusterConfig_Call) Run(run func(ctx context.Context, clusterConfig string)) *MockClusterInfoStore_ByClusterConfig_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockClusterInfoStore_ByClusterConfig_Call) Return(clusterInfo database.ClusterInfo, err error) *MockClusterInfoStore_ByClusterConfig_Call {
	_c.Call.Return(clusterInfo, err)
	return _c
}

func (_c *MockClusterInfoStore_ByClusterConfig_Call) RunAndReturn(run func(context.Context, string) (database.ClusterInfo, error)) *MockClusterInfoStore_ByClusterConfig_Call {
	_c.Call.Return(run)
	return _c
}

// ByClusterID provides a mock function with given fields: ctx, clusterId
func (_m *MockClusterInfoStore) ByClusterID(ctx context.Context, clusterId string) (database.ClusterInfo, error) {
	ret := _m.Called(ctx, clusterId)

	if len(ret) == 0 {
		panic("no return value specified for ByClusterID")
	}

	var r0 database.ClusterInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (database.ClusterInfo, error)); ok {
		return rf(ctx, clusterId)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) database.ClusterInfo); ok {
		r0 = rf(ctx, clusterId)
	} else {
		r0 = ret.Get(0).(database.ClusterInfo)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, clusterId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClusterInfoStore_ByClusterID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByClusterID'
type MockClusterInfoStore_ByClusterID_Call struct {
	*mock.Call
}

// ByClusterID is a helper method to define mock.On call
//   - ctx context.Context
//   - clusterId string
func (_e *MockClusterInfoStore_Expecter) ByClusterID(ctx interface{}, clusterId interface{}) *MockClusterInfoStore_ByClusterID_Call {
	return &MockClusterInfoStore_ByClusterID_Call{Call: _e.mock.On("ByClusterID", ctx, clusterId)}
}

func (_c *MockClusterInfoStore_ByClusterID_Call) Run(run func(ctx context.Context, clusterId string)) *MockClusterInfoStore_ByClusterID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockClusterInfoStore_ByClusterID_Call) Return(clusterInfo database.ClusterInfo, err error) *MockClusterInfoStore_ByClusterID_Call {
	_c.Call.Return(clusterInfo, err)
	return _c
}

func (_c *MockClusterInfoStore_ByClusterID_Call) RunAndReturn(run func(context.Context, string) (database.ClusterInfo, error)) *MockClusterInfoStore_ByClusterID_Call {
	_c.Call.Return(run)
	return _c
}

// List provides a mock function with given fields: ctx
func (_m *MockClusterInfoStore) List(ctx context.Context) ([]database.ClusterInfo, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 []database.ClusterInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]database.ClusterInfo, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []database.ClusterInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.ClusterInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClusterInfoStore_List_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'List'
type MockClusterInfoStore_List_Call struct {
	*mock.Call
}

// List is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockClusterInfoStore_Expecter) List(ctx interface{}) *MockClusterInfoStore_List_Call {
	return &MockClusterInfoStore_List_Call{Call: _e.mock.On("List", ctx)}
}

func (_c *MockClusterInfoStore_List_Call) Run(run func(ctx context.Context)) *MockClusterInfoStore_List_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockClusterInfoStore_List_Call) Return(_a0 []database.ClusterInfo, _a1 error) *MockClusterInfoStore_List_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockClusterInfoStore_List_Call) RunAndReturn(run func(context.Context) ([]database.ClusterInfo, error)) *MockClusterInfoStore_List_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, clusterInfo
func (_m *MockClusterInfoStore) Update(ctx context.Context, clusterInfo database.ClusterInfo) error {
	ret := _m.Called(ctx, clusterInfo)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.ClusterInfo) error); ok {
		r0 = rf(ctx, clusterInfo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClusterInfoStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockClusterInfoStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - clusterInfo database.ClusterInfo
func (_e *MockClusterInfoStore_Expecter) Update(ctx interface{}, clusterInfo interface{}) *MockClusterInfoStore_Update_Call {
	return &MockClusterInfoStore_Update_Call{Call: _e.mock.On("Update", ctx, clusterInfo)}
}

func (_c *MockClusterInfoStore_Update_Call) Run(run func(ctx context.Context, clusterInfo database.ClusterInfo)) *MockClusterInfoStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.ClusterInfo))
	})
	return _c
}

func (_c *MockClusterInfoStore_Update_Call) Return(_a0 error) *MockClusterInfoStore_Update_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClusterInfoStore_Update_Call) RunAndReturn(run func(context.Context, database.ClusterInfo) error) *MockClusterInfoStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockClusterInfoStore creates a new instance of MockClusterInfoStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClusterInfoStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClusterInfoStore {
	mock := &MockClusterInfoStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}