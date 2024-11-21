// Code generated by mockery v2.49.1. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockPromptConversationStore is an autogenerated mock type for the PromptConversationStore type
type MockPromptConversationStore struct {
	mock.Mock
}

type MockPromptConversationStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockPromptConversationStore) EXPECT() *MockPromptConversationStore_Expecter {
	return &MockPromptConversationStore_Expecter{mock: &_m.Mock}
}

// CreateConversation provides a mock function with given fields: ctx, conversation
func (_m *MockPromptConversationStore) CreateConversation(ctx context.Context, conversation database.PromptConversation) error {
	ret := _m.Called(ctx, conversation)

	if len(ret) == 0 {
		panic("no return value specified for CreateConversation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.PromptConversation) error); ok {
		r0 = rf(ctx, conversation)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockPromptConversationStore_CreateConversation_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateConversation'
type MockPromptConversationStore_CreateConversation_Call struct {
	*mock.Call
}

// CreateConversation is a helper method to define mock.On call
//   - ctx context.Context
//   - conversation database.PromptConversation
func (_e *MockPromptConversationStore_Expecter) CreateConversation(ctx interface{}, conversation interface{}) *MockPromptConversationStore_CreateConversation_Call {
	return &MockPromptConversationStore_CreateConversation_Call{Call: _e.mock.On("CreateConversation", ctx, conversation)}
}

func (_c *MockPromptConversationStore_CreateConversation_Call) Run(run func(ctx context.Context, conversation database.PromptConversation)) *MockPromptConversationStore_CreateConversation_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.PromptConversation))
	})
	return _c
}

func (_c *MockPromptConversationStore_CreateConversation_Call) Return(_a0 error) *MockPromptConversationStore_CreateConversation_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockPromptConversationStore_CreateConversation_Call) RunAndReturn(run func(context.Context, database.PromptConversation) error) *MockPromptConversationStore_CreateConversation_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteConversationsByID provides a mock function with given fields: ctx, userID, uuid
func (_m *MockPromptConversationStore) DeleteConversationsByID(ctx context.Context, userID int64, uuid string) error {
	ret := _m.Called(ctx, userID, uuid)

	if len(ret) == 0 {
		panic("no return value specified for DeleteConversationsByID")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) error); ok {
		r0 = rf(ctx, userID, uuid)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockPromptConversationStore_DeleteConversationsByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteConversationsByID'
type MockPromptConversationStore_DeleteConversationsByID_Call struct {
	*mock.Call
}

// DeleteConversationsByID is a helper method to define mock.On call
//   - ctx context.Context
//   - userID int64
//   - uuid string
func (_e *MockPromptConversationStore_Expecter) DeleteConversationsByID(ctx interface{}, userID interface{}, uuid interface{}) *MockPromptConversationStore_DeleteConversationsByID_Call {
	return &MockPromptConversationStore_DeleteConversationsByID_Call{Call: _e.mock.On("DeleteConversationsByID", ctx, userID, uuid)}
}

func (_c *MockPromptConversationStore_DeleteConversationsByID_Call) Run(run func(ctx context.Context, userID int64, uuid string)) *MockPromptConversationStore_DeleteConversationsByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64), args[2].(string))
	})
	return _c
}

func (_c *MockPromptConversationStore_DeleteConversationsByID_Call) Return(_a0 error) *MockPromptConversationStore_DeleteConversationsByID_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockPromptConversationStore_DeleteConversationsByID_Call) RunAndReturn(run func(context.Context, int64, string) error) *MockPromptConversationStore_DeleteConversationsByID_Call {
	_c.Call.Return(run)
	return _c
}

// FindConversationsByUserID provides a mock function with given fields: ctx, userID
func (_m *MockPromptConversationStore) FindConversationsByUserID(ctx context.Context, userID int64) ([]database.PromptConversation, error) {
	ret := _m.Called(ctx, userID)

	if len(ret) == 0 {
		panic("no return value specified for FindConversationsByUserID")
	}

	var r0 []database.PromptConversation
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) ([]database.PromptConversation, error)); ok {
		return rf(ctx, userID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) []database.PromptConversation); ok {
		r0 = rf(ctx, userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.PromptConversation)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockPromptConversationStore_FindConversationsByUserID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindConversationsByUserID'
type MockPromptConversationStore_FindConversationsByUserID_Call struct {
	*mock.Call
}

// FindConversationsByUserID is a helper method to define mock.On call
//   - ctx context.Context
//   - userID int64
func (_e *MockPromptConversationStore_Expecter) FindConversationsByUserID(ctx interface{}, userID interface{}) *MockPromptConversationStore_FindConversationsByUserID_Call {
	return &MockPromptConversationStore_FindConversationsByUserID_Call{Call: _e.mock.On("FindConversationsByUserID", ctx, userID)}
}

func (_c *MockPromptConversationStore_FindConversationsByUserID_Call) Run(run func(ctx context.Context, userID int64)) *MockPromptConversationStore_FindConversationsByUserID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockPromptConversationStore_FindConversationsByUserID_Call) Return(_a0 []database.PromptConversation, _a1 error) *MockPromptConversationStore_FindConversationsByUserID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockPromptConversationStore_FindConversationsByUserID_Call) RunAndReturn(run func(context.Context, int64) ([]database.PromptConversation, error)) *MockPromptConversationStore_FindConversationsByUserID_Call {
	_c.Call.Return(run)
	return _c
}

// GetConversationByID provides a mock function with given fields: ctx, userID, uuid, hasDetail
func (_m *MockPromptConversationStore) GetConversationByID(ctx context.Context, userID int64, uuid string, hasDetail bool) (*database.PromptConversation, error) {
	ret := _m.Called(ctx, userID, uuid, hasDetail)

	if len(ret) == 0 {
		panic("no return value specified for GetConversationByID")
	}

	var r0 *database.PromptConversation
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, string, bool) (*database.PromptConversation, error)); ok {
		return rf(ctx, userID, uuid, hasDetail)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64, string, bool) *database.PromptConversation); ok {
		r0 = rf(ctx, userID, uuid, hasDetail)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.PromptConversation)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64, string, bool) error); ok {
		r1 = rf(ctx, userID, uuid, hasDetail)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockPromptConversationStore_GetConversationByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetConversationByID'
type MockPromptConversationStore_GetConversationByID_Call struct {
	*mock.Call
}

// GetConversationByID is a helper method to define mock.On call
//   - ctx context.Context
//   - userID int64
//   - uuid string
//   - hasDetail bool
func (_e *MockPromptConversationStore_Expecter) GetConversationByID(ctx interface{}, userID interface{}, uuid interface{}, hasDetail interface{}) *MockPromptConversationStore_GetConversationByID_Call {
	return &MockPromptConversationStore_GetConversationByID_Call{Call: _e.mock.On("GetConversationByID", ctx, userID, uuid, hasDetail)}
}

func (_c *MockPromptConversationStore_GetConversationByID_Call) Run(run func(ctx context.Context, userID int64, uuid string, hasDetail bool)) *MockPromptConversationStore_GetConversationByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64), args[2].(string), args[3].(bool))
	})
	return _c
}

func (_c *MockPromptConversationStore_GetConversationByID_Call) Return(_a0 *database.PromptConversation, _a1 error) *MockPromptConversationStore_GetConversationByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockPromptConversationStore_GetConversationByID_Call) RunAndReturn(run func(context.Context, int64, string, bool) (*database.PromptConversation, error)) *MockPromptConversationStore_GetConversationByID_Call {
	_c.Call.Return(run)
	return _c
}

// HateMessageByID provides a mock function with given fields: ctx, id
func (_m *MockPromptConversationStore) HateMessageByID(ctx context.Context, id int64) error {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for HateMessageByID")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockPromptConversationStore_HateMessageByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'HateMessageByID'
type MockPromptConversationStore_HateMessageByID_Call struct {
	*mock.Call
}

// HateMessageByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockPromptConversationStore_Expecter) HateMessageByID(ctx interface{}, id interface{}) *MockPromptConversationStore_HateMessageByID_Call {
	return &MockPromptConversationStore_HateMessageByID_Call{Call: _e.mock.On("HateMessageByID", ctx, id)}
}

func (_c *MockPromptConversationStore_HateMessageByID_Call) Run(run func(ctx context.Context, id int64)) *MockPromptConversationStore_HateMessageByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockPromptConversationStore_HateMessageByID_Call) Return(_a0 error) *MockPromptConversationStore_HateMessageByID_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockPromptConversationStore_HateMessageByID_Call) RunAndReturn(run func(context.Context, int64) error) *MockPromptConversationStore_HateMessageByID_Call {
	_c.Call.Return(run)
	return _c
}

// LikeMessageByID provides a mock function with given fields: ctx, id
func (_m *MockPromptConversationStore) LikeMessageByID(ctx context.Context, id int64) error {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for LikeMessageByID")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockPromptConversationStore_LikeMessageByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'LikeMessageByID'
type MockPromptConversationStore_LikeMessageByID_Call struct {
	*mock.Call
}

// LikeMessageByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockPromptConversationStore_Expecter) LikeMessageByID(ctx interface{}, id interface{}) *MockPromptConversationStore_LikeMessageByID_Call {
	return &MockPromptConversationStore_LikeMessageByID_Call{Call: _e.mock.On("LikeMessageByID", ctx, id)}
}

func (_c *MockPromptConversationStore_LikeMessageByID_Call) Run(run func(ctx context.Context, id int64)) *MockPromptConversationStore_LikeMessageByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockPromptConversationStore_LikeMessageByID_Call) Return(_a0 error) *MockPromptConversationStore_LikeMessageByID_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockPromptConversationStore_LikeMessageByID_Call) RunAndReturn(run func(context.Context, int64) error) *MockPromptConversationStore_LikeMessageByID_Call {
	_c.Call.Return(run)
	return _c
}

// SaveConversationMessage provides a mock function with given fields: ctx, message
func (_m *MockPromptConversationStore) SaveConversationMessage(ctx context.Context, message database.PromptConversationMessage) (*database.PromptConversationMessage, error) {
	ret := _m.Called(ctx, message)

	if len(ret) == 0 {
		panic("no return value specified for SaveConversationMessage")
	}

	var r0 *database.PromptConversationMessage
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.PromptConversationMessage) (*database.PromptConversationMessage, error)); ok {
		return rf(ctx, message)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.PromptConversationMessage) *database.PromptConversationMessage); ok {
		r0 = rf(ctx, message)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.PromptConversationMessage)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.PromptConversationMessage) error); ok {
		r1 = rf(ctx, message)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockPromptConversationStore_SaveConversationMessage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SaveConversationMessage'
type MockPromptConversationStore_SaveConversationMessage_Call struct {
	*mock.Call
}

// SaveConversationMessage is a helper method to define mock.On call
//   - ctx context.Context
//   - message database.PromptConversationMessage
func (_e *MockPromptConversationStore_Expecter) SaveConversationMessage(ctx interface{}, message interface{}) *MockPromptConversationStore_SaveConversationMessage_Call {
	return &MockPromptConversationStore_SaveConversationMessage_Call{Call: _e.mock.On("SaveConversationMessage", ctx, message)}
}

func (_c *MockPromptConversationStore_SaveConversationMessage_Call) Run(run func(ctx context.Context, message database.PromptConversationMessage)) *MockPromptConversationStore_SaveConversationMessage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.PromptConversationMessage))
	})
	return _c
}

func (_c *MockPromptConversationStore_SaveConversationMessage_Call) Return(_a0 *database.PromptConversationMessage, _a1 error) *MockPromptConversationStore_SaveConversationMessage_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockPromptConversationStore_SaveConversationMessage_Call) RunAndReturn(run func(context.Context, database.PromptConversationMessage) (*database.PromptConversationMessage, error)) *MockPromptConversationStore_SaveConversationMessage_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateConversation provides a mock function with given fields: ctx, conversation
func (_m *MockPromptConversationStore) UpdateConversation(ctx context.Context, conversation database.PromptConversation) error {
	ret := _m.Called(ctx, conversation)

	if len(ret) == 0 {
		panic("no return value specified for UpdateConversation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.PromptConversation) error); ok {
		r0 = rf(ctx, conversation)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockPromptConversationStore_UpdateConversation_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateConversation'
type MockPromptConversationStore_UpdateConversation_Call struct {
	*mock.Call
}

// UpdateConversation is a helper method to define mock.On call
//   - ctx context.Context
//   - conversation database.PromptConversation
func (_e *MockPromptConversationStore_Expecter) UpdateConversation(ctx interface{}, conversation interface{}) *MockPromptConversationStore_UpdateConversation_Call {
	return &MockPromptConversationStore_UpdateConversation_Call{Call: _e.mock.On("UpdateConversation", ctx, conversation)}
}

func (_c *MockPromptConversationStore_UpdateConversation_Call) Run(run func(ctx context.Context, conversation database.PromptConversation)) *MockPromptConversationStore_UpdateConversation_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.PromptConversation))
	})
	return _c
}

func (_c *MockPromptConversationStore_UpdateConversation_Call) Return(_a0 error) *MockPromptConversationStore_UpdateConversation_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockPromptConversationStore_UpdateConversation_Call) RunAndReturn(run func(context.Context, database.PromptConversation) error) *MockPromptConversationStore_UpdateConversation_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockPromptConversationStore creates a new instance of MockPromptConversationStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockPromptConversationStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockPromptConversationStore {
	mock := &MockPromptConversationStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}