// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockResourceModelStore is an autogenerated mock type for the ResourceModelStore type
type MockResourceModelStore struct {
	mock.Mock
}

type MockResourceModelStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockResourceModelStore) EXPECT() *MockResourceModelStore_Expecter {
	return &MockResourceModelStore_Expecter{mock: &_m.Mock}
}

// CheckModelNameNotInRFRepo provides a mock function with given fields: ctx, modelName, repoId
func (_m *MockResourceModelStore) CheckModelNameNotInRFRepo(ctx context.Context, modelName string, repoId int64) (*database.ResourceModel, error) {
	ret := _m.Called(ctx, modelName, repoId)

	if len(ret) == 0 {
		panic("no return value specified for CheckModelNameNotInRFRepo")
	}

	var r0 *database.ResourceModel
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) (*database.ResourceModel, error)); ok {
		return rf(ctx, modelName, repoId)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) *database.ResourceModel); ok {
		r0 = rf(ctx, modelName, repoId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.ResourceModel)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, int64) error); ok {
		r1 = rf(ctx, modelName, repoId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockResourceModelStore_CheckModelNameNotInRFRepo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CheckModelNameNotInRFRepo'
type MockResourceModelStore_CheckModelNameNotInRFRepo_Call struct {
	*mock.Call
}

// CheckModelNameNotInRFRepo is a helper method to define mock.On call
//   - ctx context.Context
//   - modelName string
//   - repoId int64
func (_e *MockResourceModelStore_Expecter) CheckModelNameNotInRFRepo(ctx interface{}, modelName interface{}, repoId interface{}) *MockResourceModelStore_CheckModelNameNotInRFRepo_Call {
	return &MockResourceModelStore_CheckModelNameNotInRFRepo_Call{Call: _e.mock.On("CheckModelNameNotInRFRepo", ctx, modelName, repoId)}
}

func (_c *MockResourceModelStore_CheckModelNameNotInRFRepo_Call) Run(run func(ctx context.Context, modelName string, repoId int64)) *MockResourceModelStore_CheckModelNameNotInRFRepo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(int64))
	})
	return _c
}

func (_c *MockResourceModelStore_CheckModelNameNotInRFRepo_Call) Return(_a0 *database.ResourceModel, _a1 error) *MockResourceModelStore_CheckModelNameNotInRFRepo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockResourceModelStore_CheckModelNameNotInRFRepo_Call) RunAndReturn(run func(context.Context, string, int64) (*database.ResourceModel, error)) *MockResourceModelStore_CheckModelNameNotInRFRepo_Call {
	_c.Call.Return(run)
	return _c
}

// FindByModelName provides a mock function with given fields: ctx, modelName
func (_m *MockResourceModelStore) FindByModelName(ctx context.Context, modelName string) ([]*database.ResourceModel, error) {
	ret := _m.Called(ctx, modelName)

	if len(ret) == 0 {
		panic("no return value specified for FindByModelName")
	}

	var r0 []*database.ResourceModel
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]*database.ResourceModel, error)); ok {
		return rf(ctx, modelName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []*database.ResourceModel); ok {
		r0 = rf(ctx, modelName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*database.ResourceModel)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, modelName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockResourceModelStore_FindByModelName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByModelName'
type MockResourceModelStore_FindByModelName_Call struct {
	*mock.Call
}

// FindByModelName is a helper method to define mock.On call
//   - ctx context.Context
//   - modelName string
func (_e *MockResourceModelStore_Expecter) FindByModelName(ctx interface{}, modelName interface{}) *MockResourceModelStore_FindByModelName_Call {
	return &MockResourceModelStore_FindByModelName_Call{Call: _e.mock.On("FindByModelName", ctx, modelName)}
}

func (_c *MockResourceModelStore_FindByModelName_Call) Run(run func(ctx context.Context, modelName string)) *MockResourceModelStore_FindByModelName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockResourceModelStore_FindByModelName_Call) Return(_a0 []*database.ResourceModel, _a1 error) *MockResourceModelStore_FindByModelName_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockResourceModelStore_FindByModelName_Call) RunAndReturn(run func(context.Context, string) ([]*database.ResourceModel, error)) *MockResourceModelStore_FindByModelName_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockResourceModelStore creates a new instance of MockResourceModelStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockResourceModelStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockResourceModelStore {
	mock := &MockResourceModelStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}