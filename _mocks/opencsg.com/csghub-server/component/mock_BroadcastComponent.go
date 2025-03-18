// Code generated by mockery v2.53.0. DO NOT EDIT.

package component

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	types "opencsg.com/csghub-server/common/types"
)

// MockBroadcastComponent is an autogenerated mock type for the BroadcastComponent type
type MockBroadcastComponent struct {
	mock.Mock
}

type MockBroadcastComponent_Expecter struct {
	mock *mock.Mock
}

func (_m *MockBroadcastComponent) EXPECT() *MockBroadcastComponent_Expecter {
	return &MockBroadcastComponent_Expecter{mock: &_m.Mock}
}

// ActiveBroadcast provides a mock function with given fields: ctx
func (_m *MockBroadcastComponent) ActiveBroadcast(ctx context.Context) (*types.Broadcast, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ActiveBroadcast")
	}

	var r0 *types.Broadcast
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*types.Broadcast, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *types.Broadcast); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Broadcast)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBroadcastComponent_ActiveBroadcast_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ActiveBroadcast'
type MockBroadcastComponent_ActiveBroadcast_Call struct {
	*mock.Call
}

// ActiveBroadcast is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockBroadcastComponent_Expecter) ActiveBroadcast(ctx interface{}) *MockBroadcastComponent_ActiveBroadcast_Call {
	return &MockBroadcastComponent_ActiveBroadcast_Call{Call: _e.mock.On("ActiveBroadcast", ctx)}
}

func (_c *MockBroadcastComponent_ActiveBroadcast_Call) Run(run func(ctx context.Context)) *MockBroadcastComponent_ActiveBroadcast_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockBroadcastComponent_ActiveBroadcast_Call) Return(_a0 *types.Broadcast, _a1 error) *MockBroadcastComponent_ActiveBroadcast_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBroadcastComponent_ActiveBroadcast_Call) RunAndReturn(run func(context.Context) (*types.Broadcast, error)) *MockBroadcastComponent_ActiveBroadcast_Call {
	_c.Call.Return(run)
	return _c
}

// AllBroadcasts provides a mock function with given fields: ctx
func (_m *MockBroadcastComponent) AllBroadcasts(ctx context.Context) ([]types.Broadcast, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for AllBroadcasts")
	}

	var r0 []types.Broadcast
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]types.Broadcast, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []types.Broadcast); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Broadcast)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBroadcastComponent_AllBroadcasts_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AllBroadcasts'
type MockBroadcastComponent_AllBroadcasts_Call struct {
	*mock.Call
}

// AllBroadcasts is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockBroadcastComponent_Expecter) AllBroadcasts(ctx interface{}) *MockBroadcastComponent_AllBroadcasts_Call {
	return &MockBroadcastComponent_AllBroadcasts_Call{Call: _e.mock.On("AllBroadcasts", ctx)}
}

func (_c *MockBroadcastComponent_AllBroadcasts_Call) Run(run func(ctx context.Context)) *MockBroadcastComponent_AllBroadcasts_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockBroadcastComponent_AllBroadcasts_Call) Return(_a0 []types.Broadcast, _a1 error) *MockBroadcastComponent_AllBroadcasts_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBroadcastComponent_AllBroadcasts_Call) RunAndReturn(run func(context.Context) ([]types.Broadcast, error)) *MockBroadcastComponent_AllBroadcasts_Call {
	_c.Call.Return(run)
	return _c
}

// GetBroadcast provides a mock function with given fields: ctx, id
func (_m *MockBroadcastComponent) GetBroadcast(ctx context.Context, id int64) (*types.Broadcast, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetBroadcast")
	}

	var r0 *types.Broadcast
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*types.Broadcast, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *types.Broadcast); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Broadcast)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBroadcastComponent_GetBroadcast_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetBroadcast'
type MockBroadcastComponent_GetBroadcast_Call struct {
	*mock.Call
}

// GetBroadcast is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockBroadcastComponent_Expecter) GetBroadcast(ctx interface{}, id interface{}) *MockBroadcastComponent_GetBroadcast_Call {
	return &MockBroadcastComponent_GetBroadcast_Call{Call: _e.mock.On("GetBroadcast", ctx, id)}
}

func (_c *MockBroadcastComponent_GetBroadcast_Call) Run(run func(ctx context.Context, id int64)) *MockBroadcastComponent_GetBroadcast_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockBroadcastComponent_GetBroadcast_Call) Return(_a0 *types.Broadcast, _a1 error) *MockBroadcastComponent_GetBroadcast_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBroadcastComponent_GetBroadcast_Call) RunAndReturn(run func(context.Context, int64) (*types.Broadcast, error)) *MockBroadcastComponent_GetBroadcast_Call {
	_c.Call.Return(run)
	return _c
}

// NewBroadcast provides a mock function with given fields: ctx, broadcast
func (_m *MockBroadcastComponent) NewBroadcast(ctx context.Context, broadcast types.Broadcast) error {
	ret := _m.Called(ctx, broadcast)

	if len(ret) == 0 {
		panic("no return value specified for NewBroadcast")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, types.Broadcast) error); ok {
		r0 = rf(ctx, broadcast)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockBroadcastComponent_NewBroadcast_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewBroadcast'
type MockBroadcastComponent_NewBroadcast_Call struct {
	*mock.Call
}

// NewBroadcast is a helper method to define mock.On call
//   - ctx context.Context
//   - broadcast types.Broadcast
func (_e *MockBroadcastComponent_Expecter) NewBroadcast(ctx interface{}, broadcast interface{}) *MockBroadcastComponent_NewBroadcast_Call {
	return &MockBroadcastComponent_NewBroadcast_Call{Call: _e.mock.On("NewBroadcast", ctx, broadcast)}
}

func (_c *MockBroadcastComponent_NewBroadcast_Call) Run(run func(ctx context.Context, broadcast types.Broadcast)) *MockBroadcastComponent_NewBroadcast_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.Broadcast))
	})
	return _c
}

func (_c *MockBroadcastComponent_NewBroadcast_Call) Return(_a0 error) *MockBroadcastComponent_NewBroadcast_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockBroadcastComponent_NewBroadcast_Call) RunAndReturn(run func(context.Context, types.Broadcast) error) *MockBroadcastComponent_NewBroadcast_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateBroadcast provides a mock function with given fields: ctx, broadcast
func (_m *MockBroadcastComponent) UpdateBroadcast(ctx context.Context, broadcast types.Broadcast) (*types.Broadcast, error) {
	ret := _m.Called(ctx, broadcast)

	if len(ret) == 0 {
		panic("no return value specified for UpdateBroadcast")
	}

	var r0 *types.Broadcast
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.Broadcast) (*types.Broadcast, error)); ok {
		return rf(ctx, broadcast)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.Broadcast) *types.Broadcast); ok {
		r0 = rf(ctx, broadcast)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Broadcast)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.Broadcast) error); ok {
		r1 = rf(ctx, broadcast)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBroadcastComponent_UpdateBroadcast_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateBroadcast'
type MockBroadcastComponent_UpdateBroadcast_Call struct {
	*mock.Call
}

// UpdateBroadcast is a helper method to define mock.On call
//   - ctx context.Context
//   - broadcast types.Broadcast
func (_e *MockBroadcastComponent_Expecter) UpdateBroadcast(ctx interface{}, broadcast interface{}) *MockBroadcastComponent_UpdateBroadcast_Call {
	return &MockBroadcastComponent_UpdateBroadcast_Call{Call: _e.mock.On("UpdateBroadcast", ctx, broadcast)}
}

func (_c *MockBroadcastComponent_UpdateBroadcast_Call) Run(run func(ctx context.Context, broadcast types.Broadcast)) *MockBroadcastComponent_UpdateBroadcast_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.Broadcast))
	})
	return _c
}

func (_c *MockBroadcastComponent_UpdateBroadcast_Call) Return(_a0 *types.Broadcast, _a1 error) *MockBroadcastComponent_UpdateBroadcast_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBroadcastComponent_UpdateBroadcast_Call) RunAndReturn(run func(context.Context, types.Broadcast) (*types.Broadcast, error)) *MockBroadcastComponent_UpdateBroadcast_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockBroadcastComponent creates a new instance of MockBroadcastComponent. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockBroadcastComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBroadcastComponent {
	mock := &MockBroadcastComponent{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
