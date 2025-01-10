// Code generated by mockery v2.49.1. DO NOT EDIT.

package component

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"

	types "opencsg.com/csghub-server/common/types"
)

// MockOrganizationComponent is an autogenerated mock type for the OrganizationComponent type
type MockOrganizationComponent struct {
	mock.Mock
}

type MockOrganizationComponent_Expecter struct {
	mock *mock.Mock
}

func (_m *MockOrganizationComponent) EXPECT() *MockOrganizationComponent_Expecter {
	return &MockOrganizationComponent_Expecter{mock: &_m.Mock}
}

// Create provides a mock function with given fields: ctx, req
func (_m *MockOrganizationComponent) Create(ctx context.Context, req *types.CreateOrgReq) (*types.Organization, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *types.Organization
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.CreateOrgReq) (*types.Organization, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.CreateOrgReq) *types.Organization); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Organization)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.CreateOrgReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockOrganizationComponent_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockOrganizationComponent_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.CreateOrgReq
func (_e *MockOrganizationComponent_Expecter) Create(ctx interface{}, req interface{}) *MockOrganizationComponent_Create_Call {
	return &MockOrganizationComponent_Create_Call{Call: _e.mock.On("Create", ctx, req)}
}

func (_c *MockOrganizationComponent_Create_Call) Run(run func(ctx context.Context, req *types.CreateOrgReq)) *MockOrganizationComponent_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.CreateOrgReq))
	})
	return _c
}

func (_c *MockOrganizationComponent_Create_Call) Return(_a0 *types.Organization, _a1 error) *MockOrganizationComponent_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockOrganizationComponent_Create_Call) RunAndReturn(run func(context.Context, *types.CreateOrgReq) (*types.Organization, error)) *MockOrganizationComponent_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, req
func (_m *MockOrganizationComponent) Delete(ctx context.Context, req *types.DeleteOrgReq) error {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeleteOrgReq) error); ok {
		r0 = rf(ctx, req)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockOrganizationComponent_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockOrganizationComponent_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.DeleteOrgReq
func (_e *MockOrganizationComponent_Expecter) Delete(ctx interface{}, req interface{}) *MockOrganizationComponent_Delete_Call {
	return &MockOrganizationComponent_Delete_Call{Call: _e.mock.On("Delete", ctx, req)}
}

func (_c *MockOrganizationComponent_Delete_Call) Run(run func(ctx context.Context, req *types.DeleteOrgReq)) *MockOrganizationComponent_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.DeleteOrgReq))
	})
	return _c
}

func (_c *MockOrganizationComponent_Delete_Call) Return(_a0 error) *MockOrganizationComponent_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockOrganizationComponent_Delete_Call) RunAndReturn(run func(context.Context, *types.DeleteOrgReq) error) *MockOrganizationComponent_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// FixOrgData provides a mock function with given fields: ctx, org
func (_m *MockOrganizationComponent) FixOrgData(ctx context.Context, org *database.Organization) (*database.Organization, error) {
	ret := _m.Called(ctx, org)

	if len(ret) == 0 {
		panic("no return value specified for FixOrgData")
	}

	var r0 *database.Organization
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *database.Organization) (*database.Organization, error)); ok {
		return rf(ctx, org)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *database.Organization) *database.Organization); ok {
		r0 = rf(ctx, org)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Organization)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *database.Organization) error); ok {
		r1 = rf(ctx, org)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockOrganizationComponent_FixOrgData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FixOrgData'
type MockOrganizationComponent_FixOrgData_Call struct {
	*mock.Call
}

// FixOrgData is a helper method to define mock.On call
//   - ctx context.Context
//   - org *database.Organization
func (_e *MockOrganizationComponent_Expecter) FixOrgData(ctx interface{}, org interface{}) *MockOrganizationComponent_FixOrgData_Call {
	return &MockOrganizationComponent_FixOrgData_Call{Call: _e.mock.On("FixOrgData", ctx, org)}
}

func (_c *MockOrganizationComponent_FixOrgData_Call) Run(run func(ctx context.Context, org *database.Organization)) *MockOrganizationComponent_FixOrgData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*database.Organization))
	})
	return _c
}

func (_c *MockOrganizationComponent_FixOrgData_Call) Return(_a0 *database.Organization, _a1 error) *MockOrganizationComponent_FixOrgData_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockOrganizationComponent_FixOrgData_Call) RunAndReturn(run func(context.Context, *database.Organization) (*database.Organization, error)) *MockOrganizationComponent_FixOrgData_Call {
	_c.Call.Return(run)
	return _c
}

// Get provides a mock function with given fields: ctx, orgName
func (_m *MockOrganizationComponent) Get(ctx context.Context, orgName string) (*types.Organization, error) {
	ret := _m.Called(ctx, orgName)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 *types.Organization
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*types.Organization, error)); ok {
		return rf(ctx, orgName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.Organization); ok {
		r0 = rf(ctx, orgName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Organization)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, orgName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockOrganizationComponent_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type MockOrganizationComponent_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - ctx context.Context
//   - orgName string
func (_e *MockOrganizationComponent_Expecter) Get(ctx interface{}, orgName interface{}) *MockOrganizationComponent_Get_Call {
	return &MockOrganizationComponent_Get_Call{Call: _e.mock.On("Get", ctx, orgName)}
}

func (_c *MockOrganizationComponent_Get_Call) Run(run func(ctx context.Context, orgName string)) *MockOrganizationComponent_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockOrganizationComponent_Get_Call) Return(_a0 *types.Organization, _a1 error) *MockOrganizationComponent_Get_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockOrganizationComponent_Get_Call) RunAndReturn(run func(context.Context, string) (*types.Organization, error)) *MockOrganizationComponent_Get_Call {
	_c.Call.Return(run)
	return _c
}

// Index provides a mock function with given fields: ctx, username, search, per, page
func (_m *MockOrganizationComponent) Index(ctx context.Context, username string, search string, per int, page int) ([]types.Organization, int, error) {
	ret := _m.Called(ctx, username, search, per, page)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 []types.Organization
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int, int) ([]types.Organization, int, error)); ok {
		return rf(ctx, username, search, per, page)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int, int) []types.Organization); ok {
		r0 = rf(ctx, username, search, per, page)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Organization)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, int, int) int); ok {
		r1 = rf(ctx, username, search, per, page)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, string, int, int) error); ok {
		r2 = rf(ctx, username, search, per, page)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockOrganizationComponent_Index_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Index'
type MockOrganizationComponent_Index_Call struct {
	*mock.Call
}

// Index is a helper method to define mock.On call
//   - ctx context.Context
//   - username string
//   - search string
//   - per int
//   - page int
func (_e *MockOrganizationComponent_Expecter) Index(ctx interface{}, username interface{}, search interface{}, per interface{}, page interface{}) *MockOrganizationComponent_Index_Call {
	return &MockOrganizationComponent_Index_Call{Call: _e.mock.On("Index", ctx, username, search, per, page)}
}

func (_c *MockOrganizationComponent_Index_Call) Run(run func(ctx context.Context, username string, search string, per int, page int)) *MockOrganizationComponent_Index_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(int), args[4].(int))
	})
	return _c
}

func (_c *MockOrganizationComponent_Index_Call) Return(_a0 []types.Organization, _a1 int, _a2 error) *MockOrganizationComponent_Index_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockOrganizationComponent_Index_Call) RunAndReturn(run func(context.Context, string, string, int, int) ([]types.Organization, int, error)) *MockOrganizationComponent_Index_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, req
func (_m *MockOrganizationComponent) Update(ctx context.Context, req *types.EditOrgReq) (*database.Organization, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *database.Organization
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.EditOrgReq) (*database.Organization, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.EditOrgReq) *database.Organization); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Organization)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.EditOrgReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockOrganizationComponent_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockOrganizationComponent_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.EditOrgReq
func (_e *MockOrganizationComponent_Expecter) Update(ctx interface{}, req interface{}) *MockOrganizationComponent_Update_Call {
	return &MockOrganizationComponent_Update_Call{Call: _e.mock.On("Update", ctx, req)}
}

func (_c *MockOrganizationComponent_Update_Call) Run(run func(ctx context.Context, req *types.EditOrgReq)) *MockOrganizationComponent_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.EditOrgReq))
	})
	return _c
}

func (_c *MockOrganizationComponent_Update_Call) Return(_a0 *database.Organization, _a1 error) *MockOrganizationComponent_Update_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockOrganizationComponent_Update_Call) RunAndReturn(run func(context.Context, *types.EditOrgReq) (*database.Organization, error)) *MockOrganizationComponent_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockOrganizationComponent creates a new instance of MockOrganizationComponent. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockOrganizationComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockOrganizationComponent {
	mock := &MockOrganizationComponent{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
