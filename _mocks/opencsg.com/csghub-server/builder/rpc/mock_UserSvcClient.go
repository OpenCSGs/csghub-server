// Code generated by mockery v2.53.0. DO NOT EDIT.

package rpc

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	membership "opencsg.com/csghub-server/builder/git/membership"

	rpc "opencsg.com/csghub-server/builder/rpc"

	types "opencsg.com/csghub-server/common/types"
)

// MockUserSvcClient is an autogenerated mock type for the UserSvcClient type
type MockUserSvcClient struct {
	mock.Mock
}

type MockUserSvcClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockUserSvcClient) EXPECT() *MockUserSvcClient_Expecter {
	return &MockUserSvcClient_Expecter{mock: &_m.Mock}
}

// GetMemberRole provides a mock function with given fields: ctx, orgName, userName
func (_m *MockUserSvcClient) GetMemberRole(ctx context.Context, orgName string, userName string) (membership.Role, error) {
	ret := _m.Called(ctx, orgName, userName)

	if len(ret) == 0 {
		panic("no return value specified for GetMemberRole")
	}

	var r0 membership.Role
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (membership.Role, error)); ok {
		return rf(ctx, orgName, userName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) membership.Role); ok {
		r0 = rf(ctx, orgName, userName)
	} else {
		r0 = ret.Get(0).(membership.Role)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, orgName, userName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockUserSvcClient_GetMemberRole_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetMemberRole'
type MockUserSvcClient_GetMemberRole_Call struct {
	*mock.Call
}

// GetMemberRole is a helper method to define mock.On call
//   - ctx context.Context
//   - orgName string
//   - userName string
func (_e *MockUserSvcClient_Expecter) GetMemberRole(ctx interface{}, orgName interface{}, userName interface{}) *MockUserSvcClient_GetMemberRole_Call {
	return &MockUserSvcClient_GetMemberRole_Call{Call: _e.mock.On("GetMemberRole", ctx, orgName, userName)}
}

func (_c *MockUserSvcClient_GetMemberRole_Call) Run(run func(ctx context.Context, orgName string, userName string)) *MockUserSvcClient_GetMemberRole_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockUserSvcClient_GetMemberRole_Call) Return(_a0 membership.Role, _a1 error) *MockUserSvcClient_GetMemberRole_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockUserSvcClient_GetMemberRole_Call) RunAndReturn(run func(context.Context, string, string) (membership.Role, error)) *MockUserSvcClient_GetMemberRole_Call {
	_c.Call.Return(run)
	return _c
}

// GetNameSpaceInfo provides a mock function with given fields: ctx, path
func (_m *MockUserSvcClient) GetNameSpaceInfo(ctx context.Context, path string) (*rpc.Namespace, error) {
	ret := _m.Called(ctx, path)

	if len(ret) == 0 {
		panic("no return value specified for GetNameSpaceInfo")
	}

	var r0 *rpc.Namespace
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*rpc.Namespace, error)); ok {
		return rf(ctx, path)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *rpc.Namespace); ok {
		r0 = rf(ctx, path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*rpc.Namespace)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockUserSvcClient_GetNameSpaceInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetNameSpaceInfo'
type MockUserSvcClient_GetNameSpaceInfo_Call struct {
	*mock.Call
}

// GetNameSpaceInfo is a helper method to define mock.On call
//   - ctx context.Context
//   - path string
func (_e *MockUserSvcClient_Expecter) GetNameSpaceInfo(ctx interface{}, path interface{}) *MockUserSvcClient_GetNameSpaceInfo_Call {
	return &MockUserSvcClient_GetNameSpaceInfo_Call{Call: _e.mock.On("GetNameSpaceInfo", ctx, path)}
}

func (_c *MockUserSvcClient_GetNameSpaceInfo_Call) Run(run func(ctx context.Context, path string)) *MockUserSvcClient_GetNameSpaceInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockUserSvcClient_GetNameSpaceInfo_Call) Return(_a0 *rpc.Namespace, _a1 error) *MockUserSvcClient_GetNameSpaceInfo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockUserSvcClient_GetNameSpaceInfo_Call) RunAndReturn(run func(context.Context, string) (*rpc.Namespace, error)) *MockUserSvcClient_GetNameSpaceInfo_Call {
	_c.Call.Return(run)
	return _c
}

// GetOrCreateFirstAvaiTokens provides a mock function with given fields: ctx, userName, visitorName, app, tokenName
func (_m *MockUserSvcClient) GetOrCreateFirstAvaiTokens(ctx context.Context, userName string, visitorName string, app string, tokenName string) (string, error) {
	ret := _m.Called(ctx, userName, visitorName, app, tokenName)

	if len(ret) == 0 {
		panic("no return value specified for GetOrCreateFirstAvaiTokens")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) (string, error)); ok {
		return rf(ctx, userName, visitorName, app, tokenName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) string); ok {
		r0 = rf(ctx, userName, visitorName, app, tokenName)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string) error); ok {
		r1 = rf(ctx, userName, visitorName, app, tokenName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetOrCreateFirstAvaiTokens'
type MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call struct {
	*mock.Call
}

// GetOrCreateFirstAvaiTokens is a helper method to define mock.On call
//   - ctx context.Context
//   - userName string
//   - visitorName string
//   - app string
//   - tokenName string
func (_e *MockUserSvcClient_Expecter) GetOrCreateFirstAvaiTokens(ctx interface{}, userName interface{}, visitorName interface{}, app interface{}, tokenName interface{}) *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call {
	return &MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call{Call: _e.mock.On("GetOrCreateFirstAvaiTokens", ctx, userName, visitorName, app, tokenName)}
}

func (_c *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call) Run(run func(ctx context.Context, userName string, visitorName string, app string, tokenName string)) *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(string), args[4].(string))
	})
	return _c
}

func (_c *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call) Return(_a0 string, _a1 error) *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call) RunAndReturn(run func(context.Context, string, string, string, string) (string, error)) *MockUserSvcClient_GetOrCreateFirstAvaiTokens_Call {
	_c.Call.Return(run)
	return _c
}

// GetUserInfo provides a mock function with given fields: ctx, userName, visitorName
func (_m *MockUserSvcClient) GetUserInfo(ctx context.Context, userName string, visitorName string) (*rpc.User, error) {
	ret := _m.Called(ctx, userName, visitorName)

	if len(ret) == 0 {
		panic("no return value specified for GetUserInfo")
	}

	var r0 *rpc.User
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*rpc.User, error)); ok {
		return rf(ctx, userName, visitorName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *rpc.User); ok {
		r0 = rf(ctx, userName, visitorName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*rpc.User)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, userName, visitorName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockUserSvcClient_GetUserInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetUserInfo'
type MockUserSvcClient_GetUserInfo_Call struct {
	*mock.Call
}

// GetUserInfo is a helper method to define mock.On call
//   - ctx context.Context
//   - userName string
//   - visitorName string
func (_e *MockUserSvcClient_Expecter) GetUserInfo(ctx interface{}, userName interface{}, visitorName interface{}) *MockUserSvcClient_GetUserInfo_Call {
	return &MockUserSvcClient_GetUserInfo_Call{Call: _e.mock.On("GetUserInfo", ctx, userName, visitorName)}
}

func (_c *MockUserSvcClient_GetUserInfo_Call) Run(run func(ctx context.Context, userName string, visitorName string)) *MockUserSvcClient_GetUserInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockUserSvcClient_GetUserInfo_Call) Return(_a0 *rpc.User, _a1 error) *MockUserSvcClient_GetUserInfo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockUserSvcClient_GetUserInfo_Call) RunAndReturn(run func(context.Context, string, string) (*rpc.User, error)) *MockUserSvcClient_GetUserInfo_Call {
	_c.Call.Return(run)
	return _c
}

// VerifyByAccessToken provides a mock function with given fields: ctx, token
func (_m *MockUserSvcClient) VerifyByAccessToken(ctx context.Context, token string) (*types.CheckAccessTokenResp, error) {
	ret := _m.Called(ctx, token)

	if len(ret) == 0 {
		panic("no return value specified for VerifyByAccessToken")
	}

	var r0 *types.CheckAccessTokenResp
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*types.CheckAccessTokenResp, error)); ok {
		return rf(ctx, token)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.CheckAccessTokenResp); ok {
		r0 = rf(ctx, token)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.CheckAccessTokenResp)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, token)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockUserSvcClient_VerifyByAccessToken_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'VerifyByAccessToken'
type MockUserSvcClient_VerifyByAccessToken_Call struct {
	*mock.Call
}

// VerifyByAccessToken is a helper method to define mock.On call
//   - ctx context.Context
//   - token string
func (_e *MockUserSvcClient_Expecter) VerifyByAccessToken(ctx interface{}, token interface{}) *MockUserSvcClient_VerifyByAccessToken_Call {
	return &MockUserSvcClient_VerifyByAccessToken_Call{Call: _e.mock.On("VerifyByAccessToken", ctx, token)}
}

func (_c *MockUserSvcClient_VerifyByAccessToken_Call) Run(run func(ctx context.Context, token string)) *MockUserSvcClient_VerifyByAccessToken_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockUserSvcClient_VerifyByAccessToken_Call) Return(_a0 *types.CheckAccessTokenResp, _a1 error) *MockUserSvcClient_VerifyByAccessToken_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockUserSvcClient_VerifyByAccessToken_Call) RunAndReturn(run func(context.Context, string) (*types.CheckAccessTokenResp, error)) *MockUserSvcClient_VerifyByAccessToken_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockUserSvcClient creates a new instance of MockUserSvcClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUserSvcClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUserSvcClient {
	mock := &MockUserSvcClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
