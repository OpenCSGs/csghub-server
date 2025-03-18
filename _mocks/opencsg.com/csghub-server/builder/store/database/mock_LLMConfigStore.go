// Code generated by mockery v2.53.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockLLMConfigStore is an autogenerated mock type for the LLMConfigStore type
type MockLLMConfigStore struct {
	mock.Mock
}

type MockLLMConfigStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockLLMConfigStore) EXPECT() *MockLLMConfigStore_Expecter {
	return &MockLLMConfigStore_Expecter{mock: &_m.Mock}
}

// GetOptimization provides a mock function with given fields: ctx
func (_m *MockLLMConfigStore) GetOptimization(ctx context.Context) (*database.LLMConfig, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetOptimization")
	}

	var r0 *database.LLMConfig
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*database.LLMConfig, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *database.LLMConfig); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.LLMConfig)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockLLMConfigStore_GetOptimization_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetOptimization'
type MockLLMConfigStore_GetOptimization_Call struct {
	*mock.Call
}

// GetOptimization is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockLLMConfigStore_Expecter) GetOptimization(ctx interface{}) *MockLLMConfigStore_GetOptimization_Call {
	return &MockLLMConfigStore_GetOptimization_Call{Call: _e.mock.On("GetOptimization", ctx)}
}

func (_c *MockLLMConfigStore_GetOptimization_Call) Run(run func(ctx context.Context)) *MockLLMConfigStore_GetOptimization_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockLLMConfigStore_GetOptimization_Call) Return(_a0 *database.LLMConfig, _a1 error) *MockLLMConfigStore_GetOptimization_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockLLMConfigStore_GetOptimization_Call) RunAndReturn(run func(context.Context) (*database.LLMConfig, error)) *MockLLMConfigStore_GetOptimization_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockLLMConfigStore creates a new instance of MockLLMConfigStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockLLMConfigStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockLLMConfigStore {
	mock := &MockLLMConfigStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
