// Code generated by mockery v2.49.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"

	uuid "github.com/google/uuid"
)

// MockAccountEventStore is an autogenerated mock type for the AccountEventStore type
type MockAccountEventStore struct {
	mock.Mock
}

type MockAccountEventStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAccountEventStore) EXPECT() *MockAccountEventStore_Expecter {
	return &MockAccountEventStore_Expecter{mock: &_m.Mock}
}

// Create provides a mock function with given fields: ctx, input
func (_m *MockAccountEventStore) Create(ctx context.Context, input database.AccountEvent) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.AccountEvent) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockAccountEventStore_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockAccountEventStore_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.AccountEvent
func (_e *MockAccountEventStore_Expecter) Create(ctx interface{}, input interface{}) *MockAccountEventStore_Create_Call {
	return &MockAccountEventStore_Create_Call{Call: _e.mock.On("Create", ctx, input)}
}

func (_c *MockAccountEventStore_Create_Call) Run(run func(ctx context.Context, input database.AccountEvent)) *MockAccountEventStore_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.AccountEvent))
	})
	return _c
}

func (_c *MockAccountEventStore_Create_Call) Return(_a0 error) *MockAccountEventStore_Create_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAccountEventStore_Create_Call) RunAndReturn(run func(context.Context, database.AccountEvent) error) *MockAccountEventStore_Create_Call {
	_c.Call.Return(run)
	return _c
}

// GetByEventID provides a mock function with given fields: ctx, eventID
func (_m *MockAccountEventStore) GetByEventID(ctx context.Context, eventID uuid.UUID) (*database.AccountEvent, error) {
	ret := _m.Called(ctx, eventID)

	if len(ret) == 0 {
		panic("no return value specified for GetByEventID")
	}

	var r0 *database.AccountEvent
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) (*database.AccountEvent, error)); ok {
		return rf(ctx, eventID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID) *database.AccountEvent); ok {
		r0 = rf(ctx, eventID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.AccountEvent)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID) error); ok {
		r1 = rf(ctx, eventID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAccountEventStore_GetByEventID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetByEventID'
type MockAccountEventStore_GetByEventID_Call struct {
	*mock.Call
}

// GetByEventID is a helper method to define mock.On call
//   - ctx context.Context
//   - eventID uuid.UUID
func (_e *MockAccountEventStore_Expecter) GetByEventID(ctx interface{}, eventID interface{}) *MockAccountEventStore_GetByEventID_Call {
	return &MockAccountEventStore_GetByEventID_Call{Call: _e.mock.On("GetByEventID", ctx, eventID)}
}

func (_c *MockAccountEventStore_GetByEventID_Call) Run(run func(ctx context.Context, eventID uuid.UUID)) *MockAccountEventStore_GetByEventID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(uuid.UUID))
	})
	return _c
}

func (_c *MockAccountEventStore_GetByEventID_Call) Return(_a0 *database.AccountEvent, _a1 error) *MockAccountEventStore_GetByEventID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAccountEventStore_GetByEventID_Call) RunAndReturn(run func(context.Context, uuid.UUID) (*database.AccountEvent, error)) *MockAccountEventStore_GetByEventID_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAccountEventStore creates a new instance of MockAccountEventStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAccountEventStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAccountEventStore {
	mock := &MockAccountEventStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
