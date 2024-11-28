// Code generated by mockery v2.49.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockAccountPresentStore is an autogenerated mock type for the AccountPresentStore type
type MockAccountPresentStore struct {
	mock.Mock
}

type MockAccountPresentStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAccountPresentStore) EXPECT() *MockAccountPresentStore_Expecter {
	return &MockAccountPresentStore_Expecter{mock: &_m.Mock}
}

// AddPresent provides a mock function with given fields: ctx, input, statement
func (_m *MockAccountPresentStore) AddPresent(ctx context.Context, input database.AccountPresent, statement database.AccountStatement) error {
	ret := _m.Called(ctx, input, statement)

	if len(ret) == 0 {
		panic("no return value specified for AddPresent")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.AccountPresent, database.AccountStatement) error); ok {
		r0 = rf(ctx, input, statement)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockAccountPresentStore_AddPresent_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AddPresent'
type MockAccountPresentStore_AddPresent_Call struct {
	*mock.Call
}

// AddPresent is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.AccountPresent
//   - statement database.AccountStatement
func (_e *MockAccountPresentStore_Expecter) AddPresent(ctx interface{}, input interface{}, statement interface{}) *MockAccountPresentStore_AddPresent_Call {
	return &MockAccountPresentStore_AddPresent_Call{Call: _e.mock.On("AddPresent", ctx, input, statement)}
}

func (_c *MockAccountPresentStore_AddPresent_Call) Run(run func(ctx context.Context, input database.AccountPresent, statement database.AccountStatement)) *MockAccountPresentStore_AddPresent_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.AccountPresent), args[2].(database.AccountStatement))
	})
	return _c
}

func (_c *MockAccountPresentStore_AddPresent_Call) Return(_a0 error) *MockAccountPresentStore_AddPresent_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAccountPresentStore_AddPresent_Call) RunAndReturn(run func(context.Context, database.AccountPresent, database.AccountStatement) error) *MockAccountPresentStore_AddPresent_Call {
	_c.Call.Return(run)
	return _c
}

// FindPresentByUserIDAndScene provides a mock function with given fields: ctx, userID, activityID
func (_m *MockAccountPresentStore) FindPresentByUserIDAndScene(ctx context.Context, userID string, activityID int64) (*database.AccountPresent, error) {
	ret := _m.Called(ctx, userID, activityID)

	if len(ret) == 0 {
		panic("no return value specified for FindPresentByUserIDAndScene")
	}

	var r0 *database.AccountPresent
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) (*database.AccountPresent, error)); ok {
		return rf(ctx, userID, activityID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int64) *database.AccountPresent); ok {
		r0 = rf(ctx, userID, activityID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.AccountPresent)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, int64) error); ok {
		r1 = rf(ctx, userID, activityID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAccountPresentStore_FindPresentByUserIDAndScene_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindPresentByUserIDAndScene'
type MockAccountPresentStore_FindPresentByUserIDAndScene_Call struct {
	*mock.Call
}

// FindPresentByUserIDAndScene is a helper method to define mock.On call
//   - ctx context.Context
//   - userID string
//   - activityID int64
func (_e *MockAccountPresentStore_Expecter) FindPresentByUserIDAndScene(ctx interface{}, userID interface{}, activityID interface{}) *MockAccountPresentStore_FindPresentByUserIDAndScene_Call {
	return &MockAccountPresentStore_FindPresentByUserIDAndScene_Call{Call: _e.mock.On("FindPresentByUserIDAndScene", ctx, userID, activityID)}
}

func (_c *MockAccountPresentStore_FindPresentByUserIDAndScene_Call) Run(run func(ctx context.Context, userID string, activityID int64)) *MockAccountPresentStore_FindPresentByUserIDAndScene_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(int64))
	})
	return _c
}

func (_c *MockAccountPresentStore_FindPresentByUserIDAndScene_Call) Return(_a0 *database.AccountPresent, _a1 error) *MockAccountPresentStore_FindPresentByUserIDAndScene_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAccountPresentStore_FindPresentByUserIDAndScene_Call) RunAndReturn(run func(context.Context, string, int64) (*database.AccountPresent, error)) *MockAccountPresentStore_FindPresentByUserIDAndScene_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAccountPresentStore creates a new instance of MockAccountPresentStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAccountPresentStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAccountPresentStore {
	mock := &MockAccountPresentStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}