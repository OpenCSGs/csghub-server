// Code generated by mockery v2.49.1. DO NOT EDIT.

package component

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"

	types "opencsg.com/csghub-server/common/types"
)

// MockInternalComponent is an autogenerated mock type for the InternalComponent type
type MockInternalComponent struct {
	mock.Mock
}

type MockInternalComponent_Expecter struct {
	mock *mock.Mock
}

func (_m *MockInternalComponent) EXPECT() *MockInternalComponent_Expecter {
	return &MockInternalComponent_Expecter{mock: &_m.Mock}
}

// Allowed provides a mock function with given fields: ctx
func (_m *MockInternalComponent) Allowed(ctx context.Context) (bool, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Allowed")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (bool, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) bool); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInternalComponent_Allowed_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Allowed'
type MockInternalComponent_Allowed_Call struct {
	*mock.Call
}

// Allowed is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockInternalComponent_Expecter) Allowed(ctx interface{}) *MockInternalComponent_Allowed_Call {
	return &MockInternalComponent_Allowed_Call{Call: _e.mock.On("Allowed", ctx)}
}

func (_c *MockInternalComponent_Allowed_Call) Run(run func(ctx context.Context)) *MockInternalComponent_Allowed_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockInternalComponent_Allowed_Call) Return(_a0 bool, _a1 error) *MockInternalComponent_Allowed_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInternalComponent_Allowed_Call) RunAndReturn(run func(context.Context) (bool, error)) *MockInternalComponent_Allowed_Call {
	_c.Call.Return(run)
	return _c
}

// GetAuthorizedKeys provides a mock function with given fields: ctx, key
func (_m *MockInternalComponent) GetAuthorizedKeys(ctx context.Context, key string) (*database.SSHKey, error) {
	ret := _m.Called(ctx, key)

	if len(ret) == 0 {
		panic("no return value specified for GetAuthorizedKeys")
	}

	var r0 *database.SSHKey
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*database.SSHKey, error)); ok {
		return rf(ctx, key)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *database.SSHKey); ok {
		r0 = rf(ctx, key)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.SSHKey)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, key)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInternalComponent_GetAuthorizedKeys_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetAuthorizedKeys'
type MockInternalComponent_GetAuthorizedKeys_Call struct {
	*mock.Call
}

// GetAuthorizedKeys is a helper method to define mock.On call
//   - ctx context.Context
//   - key string
func (_e *MockInternalComponent_Expecter) GetAuthorizedKeys(ctx interface{}, key interface{}) *MockInternalComponent_GetAuthorizedKeys_Call {
	return &MockInternalComponent_GetAuthorizedKeys_Call{Call: _e.mock.On("GetAuthorizedKeys", ctx, key)}
}

func (_c *MockInternalComponent_GetAuthorizedKeys_Call) Run(run func(ctx context.Context, key string)) *MockInternalComponent_GetAuthorizedKeys_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockInternalComponent_GetAuthorizedKeys_Call) Return(_a0 *database.SSHKey, _a1 error) *MockInternalComponent_GetAuthorizedKeys_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInternalComponent_GetAuthorizedKeys_Call) RunAndReturn(run func(context.Context, string) (*database.SSHKey, error)) *MockInternalComponent_GetAuthorizedKeys_Call {
	_c.Call.Return(run)
	return _c
}

// GetCommitDiff provides a mock function with given fields: ctx, req
func (_m *MockInternalComponent) GetCommitDiff(ctx context.Context, req types.GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for GetCommitDiff")
	}

	var r0 *types.GiteaCallbackPushReq
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.GetDiffBetweenTwoCommitsReq) *types.GiteaCallbackPushReq); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.GiteaCallbackPushReq)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.GetDiffBetweenTwoCommitsReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInternalComponent_GetCommitDiff_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCommitDiff'
type MockInternalComponent_GetCommitDiff_Call struct {
	*mock.Call
}

// GetCommitDiff is a helper method to define mock.On call
//   - ctx context.Context
//   - req types.GetDiffBetweenTwoCommitsReq
func (_e *MockInternalComponent_Expecter) GetCommitDiff(ctx interface{}, req interface{}) *MockInternalComponent_GetCommitDiff_Call {
	return &MockInternalComponent_GetCommitDiff_Call{Call: _e.mock.On("GetCommitDiff", ctx, req)}
}

func (_c *MockInternalComponent_GetCommitDiff_Call) Run(run func(ctx context.Context, req types.GetDiffBetweenTwoCommitsReq)) *MockInternalComponent_GetCommitDiff_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.GetDiffBetweenTwoCommitsReq))
	})
	return _c
}

func (_c *MockInternalComponent_GetCommitDiff_Call) Return(_a0 *types.GiteaCallbackPushReq, _a1 error) *MockInternalComponent_GetCommitDiff_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInternalComponent_GetCommitDiff_Call) RunAndReturn(run func(context.Context, types.GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error)) *MockInternalComponent_GetCommitDiff_Call {
	_c.Call.Return(run)
	return _c
}

// LfsAuthenticate provides a mock function with given fields: ctx, req
func (_m *MockInternalComponent) LfsAuthenticate(ctx context.Context, req types.LfsAuthenticateReq) (*types.LfsAuthenticateResp, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for LfsAuthenticate")
	}

	var r0 *types.LfsAuthenticateResp
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.LfsAuthenticateReq) (*types.LfsAuthenticateResp, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.LfsAuthenticateReq) *types.LfsAuthenticateResp); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.LfsAuthenticateResp)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.LfsAuthenticateReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInternalComponent_LfsAuthenticate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'LfsAuthenticate'
type MockInternalComponent_LfsAuthenticate_Call struct {
	*mock.Call
}

// LfsAuthenticate is a helper method to define mock.On call
//   - ctx context.Context
//   - req types.LfsAuthenticateReq
func (_e *MockInternalComponent_Expecter) LfsAuthenticate(ctx interface{}, req interface{}) *MockInternalComponent_LfsAuthenticate_Call {
	return &MockInternalComponent_LfsAuthenticate_Call{Call: _e.mock.On("LfsAuthenticate", ctx, req)}
}

func (_c *MockInternalComponent_LfsAuthenticate_Call) Run(run func(ctx context.Context, req types.LfsAuthenticateReq)) *MockInternalComponent_LfsAuthenticate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.LfsAuthenticateReq))
	})
	return _c
}

func (_c *MockInternalComponent_LfsAuthenticate_Call) Return(_a0 *types.LfsAuthenticateResp, _a1 error) *MockInternalComponent_LfsAuthenticate_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInternalComponent_LfsAuthenticate_Call) RunAndReturn(run func(context.Context, types.LfsAuthenticateReq) (*types.LfsAuthenticateResp, error)) *MockInternalComponent_LfsAuthenticate_Call {
	_c.Call.Return(run)
	return _c
}

// SSHAllowed provides a mock function with given fields: ctx, req
func (_m *MockInternalComponent) SSHAllowed(ctx context.Context, req types.SSHAllowedReq) (*types.SSHAllowedResp, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for SSHAllowed")
	}

	var r0 *types.SSHAllowedResp
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.SSHAllowedReq) (*types.SSHAllowedResp, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.SSHAllowedReq) *types.SSHAllowedResp); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.SSHAllowedResp)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.SSHAllowedReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInternalComponent_SSHAllowed_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SSHAllowed'
type MockInternalComponent_SSHAllowed_Call struct {
	*mock.Call
}

// SSHAllowed is a helper method to define mock.On call
//   - ctx context.Context
//   - req types.SSHAllowedReq
func (_e *MockInternalComponent_Expecter) SSHAllowed(ctx interface{}, req interface{}) *MockInternalComponent_SSHAllowed_Call {
	return &MockInternalComponent_SSHAllowed_Call{Call: _e.mock.On("SSHAllowed", ctx, req)}
}

func (_c *MockInternalComponent_SSHAllowed_Call) Run(run func(ctx context.Context, req types.SSHAllowedReq)) *MockInternalComponent_SSHAllowed_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.SSHAllowedReq))
	})
	return _c
}

func (_c *MockInternalComponent_SSHAllowed_Call) Return(_a0 *types.SSHAllowedResp, _a1 error) *MockInternalComponent_SSHAllowed_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInternalComponent_SSHAllowed_Call) RunAndReturn(run func(context.Context, types.SSHAllowedReq) (*types.SSHAllowedResp, error)) *MockInternalComponent_SSHAllowed_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockInternalComponent creates a new instance of MockInternalComponent. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockInternalComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInternalComponent {
	mock := &MockInternalComponent{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}