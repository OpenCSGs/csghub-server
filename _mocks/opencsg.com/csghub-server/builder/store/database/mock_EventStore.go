// Code generated by mockery v2.53.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockEventStore is an autogenerated mock type for the EventStore type
type MockEventStore struct {
	mock.Mock
}

type MockEventStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockEventStore) EXPECT() *MockEventStore_Expecter {
	return &MockEventStore_Expecter{mock: &_m.Mock}
}

// BatchSave provides a mock function with given fields: ctx, events
func (_m *MockEventStore) BatchSave(ctx context.Context, events []database.Event) error {
	ret := _m.Called(ctx, events)

	if len(ret) == 0 {
		panic("no return value specified for BatchSave")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []database.Event) error); ok {
		r0 = rf(ctx, events)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockEventStore_BatchSave_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BatchSave'
type MockEventStore_BatchSave_Call struct {
	*mock.Call
}

// BatchSave is a helper method to define mock.On call
//   - ctx context.Context
//   - events []database.Event
func (_e *MockEventStore_Expecter) BatchSave(ctx interface{}, events interface{}) *MockEventStore_BatchSave_Call {
	return &MockEventStore_BatchSave_Call{Call: _e.mock.On("BatchSave", ctx, events)}
}

func (_c *MockEventStore_BatchSave_Call) Run(run func(ctx context.Context, events []database.Event)) *MockEventStore_BatchSave_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]database.Event))
	})
	return _c
}

func (_c *MockEventStore_BatchSave_Call) Return(_a0 error) *MockEventStore_BatchSave_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockEventStore_BatchSave_Call) RunAndReturn(run func(context.Context, []database.Event) error) *MockEventStore_BatchSave_Call {
	_c.Call.Return(run)
	return _c
}

// Save provides a mock function with given fields: ctx, event
func (_m *MockEventStore) Save(ctx context.Context, event database.Event) error {
	ret := _m.Called(ctx, event)

	if len(ret) == 0 {
		panic("no return value specified for Save")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Event) error); ok {
		r0 = rf(ctx, event)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockEventStore_Save_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Save'
type MockEventStore_Save_Call struct {
	*mock.Call
}

// Save is a helper method to define mock.On call
//   - ctx context.Context
//   - event database.Event
func (_e *MockEventStore_Expecter) Save(ctx interface{}, event interface{}) *MockEventStore_Save_Call {
	return &MockEventStore_Save_Call{Call: _e.mock.On("Save", ctx, event)}
}

func (_c *MockEventStore_Save_Call) Run(run func(ctx context.Context, event database.Event)) *MockEventStore_Save_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Event))
	})
	return _c
}

func (_c *MockEventStore_Save_Call) Return(_a0 error) *MockEventStore_Save_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockEventStore_Save_Call) RunAndReturn(run func(context.Context, database.Event) error) *MockEventStore_Save_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockEventStore creates a new instance of MockEventStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockEventStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockEventStore {
	mock := &MockEventStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
