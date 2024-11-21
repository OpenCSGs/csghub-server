// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockTagRuleStore is an autogenerated mock type for the TagRuleStore type
type MockTagRuleStore struct {
	mock.Mock
}

type MockTagRuleStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockTagRuleStore) EXPECT() *MockTagRuleStore_Expecter {
	return &MockTagRuleStore_Expecter{mock: &_m.Mock}
}

// FindByRepo provides a mock function with given fields: ctx, category, namespace, repoName, repoType
func (_m *MockTagRuleStore) FindByRepo(ctx context.Context, category string, namespace string, repoName string, repoType string) (*database.TagRule, error) {
	ret := _m.Called(ctx, category, namespace, repoName, repoType)

	if len(ret) == 0 {
		panic("no return value specified for FindByRepo")
	}

	var r0 *database.TagRule
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) (*database.TagRule, error)); ok {
		return rf(ctx, category, namespace, repoName, repoType)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) *database.TagRule); ok {
		r0 = rf(ctx, category, namespace, repoName, repoType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.TagRule)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string) error); ok {
		r1 = rf(ctx, category, namespace, repoName, repoType)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTagRuleStore_FindByRepo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByRepo'
type MockTagRuleStore_FindByRepo_Call struct {
	*mock.Call
}

// FindByRepo is a helper method to define mock.On call
//   - ctx context.Context
//   - category string
//   - namespace string
//   - repoName string
//   - repoType string
func (_e *MockTagRuleStore_Expecter) FindByRepo(ctx interface{}, category interface{}, namespace interface{}, repoName interface{}, repoType interface{}) *MockTagRuleStore_FindByRepo_Call {
	return &MockTagRuleStore_FindByRepo_Call{Call: _e.mock.On("FindByRepo", ctx, category, namespace, repoName, repoType)}
}

func (_c *MockTagRuleStore_FindByRepo_Call) Run(run func(ctx context.Context, category string, namespace string, repoName string, repoType string)) *MockTagRuleStore_FindByRepo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(string), args[4].(string))
	})
	return _c
}

func (_c *MockTagRuleStore_FindByRepo_Call) Return(_a0 *database.TagRule, _a1 error) *MockTagRuleStore_FindByRepo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTagRuleStore_FindByRepo_Call) RunAndReturn(run func(context.Context, string, string, string, string) (*database.TagRule, error)) *MockTagRuleStore_FindByRepo_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockTagRuleStore creates a new instance of MockTagRuleStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTagRuleStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTagRuleStore {
	mock := &MockTagRuleStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}