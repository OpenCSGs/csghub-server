// Code generated by mockery v2.49.1. DO NOT EDIT.

package callback

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	types "opencsg.com/csghub-server/common/types"
)

// MockGitCallbackComponent is an autogenerated mock type for the GitCallbackComponent type
type MockGitCallbackComponent struct {
	mock.Mock
}

type MockGitCallbackComponent_Expecter struct {
	mock *mock.Mock
}

func (_m *MockGitCallbackComponent) EXPECT() *MockGitCallbackComponent_Expecter {
	return &MockGitCallbackComponent_Expecter{mock: &_m.Mock}
}

// SensitiveCheck provides a mock function with given fields: ctx, req
func (_m *MockGitCallbackComponent) SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for SensitiveCheck")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.GiteaCallbackPushReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockGitCallbackComponent_SensitiveCheck_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SensitiveCheck'
type MockGitCallbackComponent_SensitiveCheck_Call struct {
	*mock.Call
}

// SensitiveCheck is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.GiteaCallbackPushReq
func (_e *MockGitCallbackComponent_Expecter) SensitiveCheck(ctx interface{}, req interface{}) *MockGitCallbackComponent_SensitiveCheck_Call {
	return &MockGitCallbackComponent_SensitiveCheck_Call{Call: _e.mock.On("SensitiveCheck", ctx, req)}
}

func (_c *MockGitCallbackComponent_SensitiveCheck_Call) Run(run func(ctx context.Context, req *types.GiteaCallbackPushReq)) *MockGitCallbackComponent_SensitiveCheck_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.GiteaCallbackPushReq))
	})
	return _c
}

func (_c *MockGitCallbackComponent_SensitiveCheck_Call) Return(_a0 error) *MockGitCallbackComponent_SensitiveCheck_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockGitCallbackComponent_SensitiveCheck_Call) RunAndReturn(run func(context.Context, *types.GiteaCallbackPushReq) error) *MockGitCallbackComponent_SensitiveCheck_Call {
	_c.Call.Return(run)
	return _c
}

// SetRepoUpdateTime provides a mock function with given fields: ctx, req
func (_m *MockGitCallbackComponent) SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for SetRepoUpdateTime")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.GiteaCallbackPushReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockGitCallbackComponent_SetRepoUpdateTime_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetRepoUpdateTime'
type MockGitCallbackComponent_SetRepoUpdateTime_Call struct {
	*mock.Call
}

// SetRepoUpdateTime is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.GiteaCallbackPushReq
func (_e *MockGitCallbackComponent_Expecter) SetRepoUpdateTime(ctx interface{}, req interface{}) *MockGitCallbackComponent_SetRepoUpdateTime_Call {
	return &MockGitCallbackComponent_SetRepoUpdateTime_Call{Call: _e.mock.On("SetRepoUpdateTime", ctx, req)}
}

func (_c *MockGitCallbackComponent_SetRepoUpdateTime_Call) Run(run func(ctx context.Context, req *types.GiteaCallbackPushReq)) *MockGitCallbackComponent_SetRepoUpdateTime_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.GiteaCallbackPushReq))
	})
	return _c
}

func (_c *MockGitCallbackComponent_SetRepoUpdateTime_Call) Return(_a0 error) *MockGitCallbackComponent_SetRepoUpdateTime_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockGitCallbackComponent_SetRepoUpdateTime_Call) RunAndReturn(run func(context.Context, *types.GiteaCallbackPushReq) error) *MockGitCallbackComponent_SetRepoUpdateTime_Call {
	_c.Call.Return(run)
	return _c
}

// SetRepoVisibility provides a mock function with given fields: yes
func (_m *MockGitCallbackComponent) SetRepoVisibility(yes bool) {
	_m.Called(yes)
}

// MockGitCallbackComponent_SetRepoVisibility_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SetRepoVisibility'
type MockGitCallbackComponent_SetRepoVisibility_Call struct {
	*mock.Call
}

// SetRepoVisibility is a helper method to define mock.On call
//   - yes bool
func (_e *MockGitCallbackComponent_Expecter) SetRepoVisibility(yes interface{}) *MockGitCallbackComponent_SetRepoVisibility_Call {
	return &MockGitCallbackComponent_SetRepoVisibility_Call{Call: _e.mock.On("SetRepoVisibility", yes)}
}

func (_c *MockGitCallbackComponent_SetRepoVisibility_Call) Run(run func(yes bool)) *MockGitCallbackComponent_SetRepoVisibility_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(bool))
	})
	return _c
}

func (_c *MockGitCallbackComponent_SetRepoVisibility_Call) Return() *MockGitCallbackComponent_SetRepoVisibility_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockGitCallbackComponent_SetRepoVisibility_Call) RunAndReturn(run func(bool)) *MockGitCallbackComponent_SetRepoVisibility_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateRepoInfos provides a mock function with given fields: ctx, req
func (_m *MockGitCallbackComponent) UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for UpdateRepoInfos")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.GiteaCallbackPushReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockGitCallbackComponent_UpdateRepoInfos_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateRepoInfos'
type MockGitCallbackComponent_UpdateRepoInfos_Call struct {
	*mock.Call
}

// UpdateRepoInfos is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.GiteaCallbackPushReq
func (_e *MockGitCallbackComponent_Expecter) UpdateRepoInfos(ctx interface{}, req interface{}) *MockGitCallbackComponent_UpdateRepoInfos_Call {
	return &MockGitCallbackComponent_UpdateRepoInfos_Call{Call: _e.mock.On("UpdateRepoInfos", ctx, req)}
}

func (_c *MockGitCallbackComponent_UpdateRepoInfos_Call) Run(run func(ctx context.Context, req *types.GiteaCallbackPushReq)) *MockGitCallbackComponent_UpdateRepoInfos_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.GiteaCallbackPushReq))
	})
	return _c
}

func (_c *MockGitCallbackComponent_UpdateRepoInfos_Call) Return(_a0 error) *MockGitCallbackComponent_UpdateRepoInfos_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockGitCallbackComponent_UpdateRepoInfos_Call) RunAndReturn(run func(context.Context, *types.GiteaCallbackPushReq) error) *MockGitCallbackComponent_UpdateRepoInfos_Call {
	_c.Call.Return(run)
	return _c
}

// WatchRepoRelation provides a mock function with given fields: ctx, req
func (_m *MockGitCallbackComponent) WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for WatchRepoRelation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.GiteaCallbackPushReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockGitCallbackComponent_WatchRepoRelation_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WatchRepoRelation'
type MockGitCallbackComponent_WatchRepoRelation_Call struct {
	*mock.Call
}

// WatchRepoRelation is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.GiteaCallbackPushReq
func (_e *MockGitCallbackComponent_Expecter) WatchRepoRelation(ctx interface{}, req interface{}) *MockGitCallbackComponent_WatchRepoRelation_Call {
	return &MockGitCallbackComponent_WatchRepoRelation_Call{Call: _e.mock.On("WatchRepoRelation", ctx, req)}
}

func (_c *MockGitCallbackComponent_WatchRepoRelation_Call) Run(run func(ctx context.Context, req *types.GiteaCallbackPushReq)) *MockGitCallbackComponent_WatchRepoRelation_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.GiteaCallbackPushReq))
	})
	return _c
}

func (_c *MockGitCallbackComponent_WatchRepoRelation_Call) Return(_a0 error) *MockGitCallbackComponent_WatchRepoRelation_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockGitCallbackComponent_WatchRepoRelation_Call) RunAndReturn(run func(context.Context, *types.GiteaCallbackPushReq) error) *MockGitCallbackComponent_WatchRepoRelation_Call {
	_c.Call.Return(run)
	return _c
}

// WatchSpaceChange provides a mock function with given fields: ctx, req
func (_m *MockGitCallbackComponent) WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for WatchSpaceChange")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.GiteaCallbackPushReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockGitCallbackComponent_WatchSpaceChange_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WatchSpaceChange'
type MockGitCallbackComponent_WatchSpaceChange_Call struct {
	*mock.Call
}

// WatchSpaceChange is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.GiteaCallbackPushReq
func (_e *MockGitCallbackComponent_Expecter) WatchSpaceChange(ctx interface{}, req interface{}) *MockGitCallbackComponent_WatchSpaceChange_Call {
	return &MockGitCallbackComponent_WatchSpaceChange_Call{Call: _e.mock.On("WatchSpaceChange", ctx, req)}
}

func (_c *MockGitCallbackComponent_WatchSpaceChange_Call) Run(run func(ctx context.Context, req *types.GiteaCallbackPushReq)) *MockGitCallbackComponent_WatchSpaceChange_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.GiteaCallbackPushReq))
	})
	return _c
}

func (_c *MockGitCallbackComponent_WatchSpaceChange_Call) Return(_a0 error) *MockGitCallbackComponent_WatchSpaceChange_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockGitCallbackComponent_WatchSpaceChange_Call) RunAndReturn(run func(context.Context, *types.GiteaCallbackPushReq) error) *MockGitCallbackComponent_WatchSpaceChange_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockGitCallbackComponent creates a new instance of MockGitCallbackComponent. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockGitCallbackComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockGitCallbackComponent {
	mock := &MockGitCallbackComponent{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}