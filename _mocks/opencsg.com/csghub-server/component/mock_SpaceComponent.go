// Code generated by mockery v2.53.0. DO NOT EDIT.

package component

import (
	context "context"

	deploy "opencsg.com/csghub-server/builder/deploy"
	database "opencsg.com/csghub-server/builder/store/database"

	mock "github.com/stretchr/testify/mock"

	types "opencsg.com/csghub-server/common/types"
)

// MockSpaceComponent is an autogenerated mock type for the SpaceComponent type
type MockSpaceComponent struct {
	mock.Mock
}

type MockSpaceComponent_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSpaceComponent) EXPECT() *MockSpaceComponent_Expecter {
	return &MockSpaceComponent_Expecter{mock: &_m.Mock}
}

// AllowCallApi provides a mock function with given fields: ctx, spaceID, username
func (_m *MockSpaceComponent) AllowCallApi(ctx context.Context, spaceID int64, username string) (bool, error) {
	ret := _m.Called(ctx, spaceID, username)

	if len(ret) == 0 {
		panic("no return value specified for AllowCallApi")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) (bool, error)); ok {
		return rf(ctx, spaceID, username)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64, string) bool); ok {
		r0 = rf(ctx, spaceID, username)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64, string) error); ok {
		r1 = rf(ctx, spaceID, username)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_AllowCallApi_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AllowCallApi'
type MockSpaceComponent_AllowCallApi_Call struct {
	*mock.Call
}

// AllowCallApi is a helper method to define mock.On call
//   - ctx context.Context
//   - spaceID int64
//   - username string
func (_e *MockSpaceComponent_Expecter) AllowCallApi(ctx interface{}, spaceID interface{}, username interface{}) *MockSpaceComponent_AllowCallApi_Call {
	return &MockSpaceComponent_AllowCallApi_Call{Call: _e.mock.On("AllowCallApi", ctx, spaceID, username)}
}

func (_c *MockSpaceComponent_AllowCallApi_Call) Run(run func(ctx context.Context, spaceID int64, username string)) *MockSpaceComponent_AllowCallApi_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64), args[2].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_AllowCallApi_Call) Return(_a0 bool, _a1 error) *MockSpaceComponent_AllowCallApi_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_AllowCallApi_Call) RunAndReturn(run func(context.Context, int64, string) (bool, error)) *MockSpaceComponent_AllowCallApi_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function with given fields: ctx, req
func (_m *MockSpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *types.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.CreateSpaceReq) (*types.Space, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.CreateSpaceReq) *types.Space); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.CreateSpaceReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockSpaceComponent_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - req types.CreateSpaceReq
func (_e *MockSpaceComponent_Expecter) Create(ctx interface{}, req interface{}) *MockSpaceComponent_Create_Call {
	return &MockSpaceComponent_Create_Call{Call: _e.mock.On("Create", ctx, req)}
}

func (_c *MockSpaceComponent_Create_Call) Run(run func(ctx context.Context, req types.CreateSpaceReq)) *MockSpaceComponent_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(types.CreateSpaceReq))
	})
	return _c
}

func (_c *MockSpaceComponent_Create_Call) Return(_a0 *types.Space, _a1 error) *MockSpaceComponent_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_Create_Call) RunAndReturn(run func(context.Context, types.CreateSpaceReq) (*types.Space, error)) *MockSpaceComponent_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, namespace, name, currentUser
func (_m *MockSpaceComponent) Delete(ctx context.Context, namespace string, name string, currentUser string) error {
	ret := _m.Called(ctx, namespace, name, currentUser)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) error); ok {
		r0 = rf(ctx, namespace, name, currentUser)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceComponent_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockSpaceComponent_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
//   - currentUser string
func (_e *MockSpaceComponent_Expecter) Delete(ctx interface{}, namespace interface{}, name interface{}, currentUser interface{}) *MockSpaceComponent_Delete_Call {
	return &MockSpaceComponent_Delete_Call{Call: _e.mock.On("Delete", ctx, namespace, name, currentUser)}
}

func (_c *MockSpaceComponent_Delete_Call) Run(run func(ctx context.Context, namespace string, name string, currentUser string)) *MockSpaceComponent_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_Delete_Call) Return(_a0 error) *MockSpaceComponent_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceComponent_Delete_Call) RunAndReturn(run func(context.Context, string, string, string) error) *MockSpaceComponent_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// Deploy provides a mock function with given fields: ctx, namespace, name, currentUser
func (_m *MockSpaceComponent) Deploy(ctx context.Context, namespace string, name string, currentUser string) (int64, error) {
	ret := _m.Called(ctx, namespace, name, currentUser)

	if len(ret) == 0 {
		panic("no return value specified for Deploy")
	}

	var r0 int64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) (int64, error)); ok {
		return rf(ctx, namespace, name, currentUser)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) int64); ok {
		r0 = rf(ctx, namespace, name, currentUser)
	} else {
		r0 = ret.Get(0).(int64)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, namespace, name, currentUser)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_Deploy_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Deploy'
type MockSpaceComponent_Deploy_Call struct {
	*mock.Call
}

// Deploy is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
//   - currentUser string
func (_e *MockSpaceComponent_Expecter) Deploy(ctx interface{}, namespace interface{}, name interface{}, currentUser interface{}) *MockSpaceComponent_Deploy_Call {
	return &MockSpaceComponent_Deploy_Call{Call: _e.mock.On("Deploy", ctx, namespace, name, currentUser)}
}

func (_c *MockSpaceComponent_Deploy_Call) Run(run func(ctx context.Context, namespace string, name string, currentUser string)) *MockSpaceComponent_Deploy_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_Deploy_Call) Return(_a0 int64, _a1 error) *MockSpaceComponent_Deploy_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_Deploy_Call) RunAndReturn(run func(context.Context, string, string, string) (int64, error)) *MockSpaceComponent_Deploy_Call {
	_c.Call.Return(run)
	return _c
}

// FixHasEntryFile provides a mock function with given fields: ctx, s
func (_m *MockSpaceComponent) FixHasEntryFile(ctx context.Context, s *database.Space) *database.Space {
	ret := _m.Called(ctx, s)

	if len(ret) == 0 {
		panic("no return value specified for FixHasEntryFile")
	}

	var r0 *database.Space
	if rf, ok := ret.Get(0).(func(context.Context, *database.Space) *database.Space); ok {
		r0 = rf(ctx, s)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Space)
		}
	}

	return r0
}

// MockSpaceComponent_FixHasEntryFile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FixHasEntryFile'
type MockSpaceComponent_FixHasEntryFile_Call struct {
	*mock.Call
}

// FixHasEntryFile is a helper method to define mock.On call
//   - ctx context.Context
//   - s *database.Space
func (_e *MockSpaceComponent_Expecter) FixHasEntryFile(ctx interface{}, s interface{}) *MockSpaceComponent_FixHasEntryFile_Call {
	return &MockSpaceComponent_FixHasEntryFile_Call{Call: _e.mock.On("FixHasEntryFile", ctx, s)}
}

func (_c *MockSpaceComponent_FixHasEntryFile_Call) Run(run func(ctx context.Context, s *database.Space)) *MockSpaceComponent_FixHasEntryFile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*database.Space))
	})
	return _c
}

func (_c *MockSpaceComponent_FixHasEntryFile_Call) Return(_a0 *database.Space) *MockSpaceComponent_FixHasEntryFile_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceComponent_FixHasEntryFile_Call) RunAndReturn(run func(context.Context, *database.Space) *database.Space) *MockSpaceComponent_FixHasEntryFile_Call {
	_c.Call.Return(run)
	return _c
}

// HasEntryFile provides a mock function with given fields: ctx, space
func (_m *MockSpaceComponent) HasEntryFile(ctx context.Context, space *database.Space) bool {
	ret := _m.Called(ctx, space)

	if len(ret) == 0 {
		panic("no return value specified for HasEntryFile")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, *database.Space) bool); ok {
		r0 = rf(ctx, space)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// MockSpaceComponent_HasEntryFile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'HasEntryFile'
type MockSpaceComponent_HasEntryFile_Call struct {
	*mock.Call
}

// HasEntryFile is a helper method to define mock.On call
//   - ctx context.Context
//   - space *database.Space
func (_e *MockSpaceComponent_Expecter) HasEntryFile(ctx interface{}, space interface{}) *MockSpaceComponent_HasEntryFile_Call {
	return &MockSpaceComponent_HasEntryFile_Call{Call: _e.mock.On("HasEntryFile", ctx, space)}
}

func (_c *MockSpaceComponent_HasEntryFile_Call) Run(run func(ctx context.Context, space *database.Space)) *MockSpaceComponent_HasEntryFile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*database.Space))
	})
	return _c
}

func (_c *MockSpaceComponent_HasEntryFile_Call) Return(_a0 bool) *MockSpaceComponent_HasEntryFile_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceComponent_HasEntryFile_Call) RunAndReturn(run func(context.Context, *database.Space) bool) *MockSpaceComponent_HasEntryFile_Call {
	_c.Call.Return(run)
	return _c
}

// Index provides a mock function with given fields: ctx, repoFilter, per, page, needOpWeight
func (_m *MockSpaceComponent) Index(ctx context.Context, repoFilter *types.RepoFilter, per int, page int, needOpWeight bool) ([]*types.Space, int, error) {
	ret := _m.Called(ctx, repoFilter, per, page, needOpWeight)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 []*types.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.RepoFilter, int, int, bool) ([]*types.Space, int, error)); ok {
		return rf(ctx, repoFilter, per, page, needOpWeight)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.RepoFilter, int, int, bool) []*types.Space); ok {
		r0 = rf(ctx, repoFilter, per, page, needOpWeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.RepoFilter, int, int, bool) int); ok {
		r1 = rf(ctx, repoFilter, per, page, needOpWeight)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.RepoFilter, int, int, bool) error); ok {
		r2 = rf(ctx, repoFilter, per, page, needOpWeight)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockSpaceComponent_Index_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Index'
type MockSpaceComponent_Index_Call struct {
	*mock.Call
}

// Index is a helper method to define mock.On call
//   - ctx context.Context
//   - repoFilter *types.RepoFilter
//   - per int
//   - page int
//   - needOpWeight bool
func (_e *MockSpaceComponent_Expecter) Index(ctx interface{}, repoFilter interface{}, per interface{}, page interface{}, needOpWeight interface{}) *MockSpaceComponent_Index_Call {
	return &MockSpaceComponent_Index_Call{Call: _e.mock.On("Index", ctx, repoFilter, per, page, needOpWeight)}
}

func (_c *MockSpaceComponent_Index_Call) Run(run func(ctx context.Context, repoFilter *types.RepoFilter, per int, page int, needOpWeight bool)) *MockSpaceComponent_Index_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.RepoFilter), args[2].(int), args[3].(int), args[4].(bool))
	})
	return _c
}

func (_c *MockSpaceComponent_Index_Call) Return(_a0 []*types.Space, _a1 int, _a2 error) *MockSpaceComponent_Index_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockSpaceComponent_Index_Call) RunAndReturn(run func(context.Context, *types.RepoFilter, int, int, bool) ([]*types.Space, int, error)) *MockSpaceComponent_Index_Call {
	_c.Call.Return(run)
	return _c
}

// ListByPath provides a mock function with given fields: ctx, paths
func (_m *MockSpaceComponent) ListByPath(ctx context.Context, paths []string) ([]*types.Space, error) {
	ret := _m.Called(ctx, paths)

	if len(ret) == 0 {
		panic("no return value specified for ListByPath")
	}

	var r0 []*types.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]*types.Space, error)); ok {
		return rf(ctx, paths)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*types.Space); ok {
		r0 = rf(ctx, paths)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, paths)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_ListByPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListByPath'
type MockSpaceComponent_ListByPath_Call struct {
	*mock.Call
}

// ListByPath is a helper method to define mock.On call
//   - ctx context.Context
//   - paths []string
func (_e *MockSpaceComponent_Expecter) ListByPath(ctx interface{}, paths interface{}) *MockSpaceComponent_ListByPath_Call {
	return &MockSpaceComponent_ListByPath_Call{Call: _e.mock.On("ListByPath", ctx, paths)}
}

func (_c *MockSpaceComponent_ListByPath_Call) Run(run func(ctx context.Context, paths []string)) *MockSpaceComponent_ListByPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]string))
	})
	return _c
}

func (_c *MockSpaceComponent_ListByPath_Call) Return(_a0 []*types.Space, _a1 error) *MockSpaceComponent_ListByPath_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_ListByPath_Call) RunAndReturn(run func(context.Context, []string) ([]*types.Space, error)) *MockSpaceComponent_ListByPath_Call {
	_c.Call.Return(run)
	return _c
}

// Logs provides a mock function with given fields: ctx, namespace, name
func (_m *MockSpaceComponent) Logs(ctx context.Context, namespace string, name string) (*deploy.MultiLogReader, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for Logs")
	}

	var r0 *deploy.MultiLogReader
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*deploy.MultiLogReader, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *deploy.MultiLogReader); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*deploy.MultiLogReader)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_Logs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Logs'
type MockSpaceComponent_Logs_Call struct {
	*mock.Call
}

// Logs is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
func (_e *MockSpaceComponent_Expecter) Logs(ctx interface{}, namespace interface{}, name interface{}) *MockSpaceComponent_Logs_Call {
	return &MockSpaceComponent_Logs_Call{Call: _e.mock.On("Logs", ctx, namespace, name)}
}

func (_c *MockSpaceComponent_Logs_Call) Run(run func(ctx context.Context, namespace string, name string)) *MockSpaceComponent_Logs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_Logs_Call) Return(_a0 *deploy.MultiLogReader, _a1 error) *MockSpaceComponent_Logs_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_Logs_Call) RunAndReturn(run func(context.Context, string, string) (*deploy.MultiLogReader, error)) *MockSpaceComponent_Logs_Call {
	_c.Call.Return(run)
	return _c
}

// OrgSpaces provides a mock function with given fields: ctx, req
func (_m *MockSpaceComponent) OrgSpaces(ctx context.Context, req *types.OrgSpacesReq) ([]types.Space, int, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for OrgSpaces")
	}

	var r0 []types.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.OrgSpacesReq) ([]types.Space, int, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.OrgSpacesReq) []types.Space); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.OrgSpacesReq) int); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.OrgSpacesReq) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockSpaceComponent_OrgSpaces_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'OrgSpaces'
type MockSpaceComponent_OrgSpaces_Call struct {
	*mock.Call
}

// OrgSpaces is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.OrgSpacesReq
func (_e *MockSpaceComponent_Expecter) OrgSpaces(ctx interface{}, req interface{}) *MockSpaceComponent_OrgSpaces_Call {
	return &MockSpaceComponent_OrgSpaces_Call{Call: _e.mock.On("OrgSpaces", ctx, req)}
}

func (_c *MockSpaceComponent_OrgSpaces_Call) Run(run func(ctx context.Context, req *types.OrgSpacesReq)) *MockSpaceComponent_OrgSpaces_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.OrgSpacesReq))
	})
	return _c
}

func (_c *MockSpaceComponent_OrgSpaces_Call) Return(_a0 []types.Space, _a1 int, _a2 error) *MockSpaceComponent_OrgSpaces_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockSpaceComponent_OrgSpaces_Call) RunAndReturn(run func(context.Context, *types.OrgSpacesReq) ([]types.Space, int, error)) *MockSpaceComponent_OrgSpaces_Call {
	_c.Call.Return(run)
	return _c
}

// Show provides a mock function with given fields: ctx, namespace, name, currentUser
func (_m *MockSpaceComponent) Show(ctx context.Context, namespace string, name string, currentUser string) (*types.Space, error) {
	ret := _m.Called(ctx, namespace, name, currentUser)

	if len(ret) == 0 {
		panic("no return value specified for Show")
	}

	var r0 *types.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) (*types.Space, error)); ok {
		return rf(ctx, namespace, name, currentUser)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) *types.Space); ok {
		r0 = rf(ctx, namespace, name, currentUser)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, namespace, name, currentUser)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_Show_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Show'
type MockSpaceComponent_Show_Call struct {
	*mock.Call
}

// Show is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
//   - currentUser string
func (_e *MockSpaceComponent_Expecter) Show(ctx interface{}, namespace interface{}, name interface{}, currentUser interface{}) *MockSpaceComponent_Show_Call {
	return &MockSpaceComponent_Show_Call{Call: _e.mock.On("Show", ctx, namespace, name, currentUser)}
}

func (_c *MockSpaceComponent_Show_Call) Run(run func(ctx context.Context, namespace string, name string, currentUser string)) *MockSpaceComponent_Show_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_Show_Call) Return(_a0 *types.Space, _a1 error) *MockSpaceComponent_Show_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_Show_Call) RunAndReturn(run func(context.Context, string, string, string) (*types.Space, error)) *MockSpaceComponent_Show_Call {
	_c.Call.Return(run)
	return _c
}

// Status provides a mock function with given fields: ctx, namespace, name
func (_m *MockSpaceComponent) Status(ctx context.Context, namespace string, name string) (string, string, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for Status")
	}

	var r0 string
	var r1 string
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (string, string, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) string); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) string); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Get(1).(string)
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, string) error); ok {
		r2 = rf(ctx, namespace, name)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockSpaceComponent_Status_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Status'
type MockSpaceComponent_Status_Call struct {
	*mock.Call
}

// Status is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
func (_e *MockSpaceComponent_Expecter) Status(ctx interface{}, namespace interface{}, name interface{}) *MockSpaceComponent_Status_Call {
	return &MockSpaceComponent_Status_Call{Call: _e.mock.On("Status", ctx, namespace, name)}
}

func (_c *MockSpaceComponent_Status_Call) Run(run func(ctx context.Context, namespace string, name string)) *MockSpaceComponent_Status_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_Status_Call) Return(_a0 string, _a1 string, _a2 error) *MockSpaceComponent_Status_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockSpaceComponent_Status_Call) RunAndReturn(run func(context.Context, string, string) (string, string, error)) *MockSpaceComponent_Status_Call {
	_c.Call.Return(run)
	return _c
}

// Stop provides a mock function with given fields: ctx, namespace, name, deleteSpace
func (_m *MockSpaceComponent) Stop(ctx context.Context, namespace string, name string, deleteSpace bool) error {
	ret := _m.Called(ctx, namespace, name, deleteSpace)

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, bool) error); ok {
		r0 = rf(ctx, namespace, name, deleteSpace)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceComponent_Stop_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Stop'
type MockSpaceComponent_Stop_Call struct {
	*mock.Call
}

// Stop is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
//   - deleteSpace bool
func (_e *MockSpaceComponent_Expecter) Stop(ctx interface{}, namespace interface{}, name interface{}, deleteSpace interface{}) *MockSpaceComponent_Stop_Call {
	return &MockSpaceComponent_Stop_Call{Call: _e.mock.On("Stop", ctx, namespace, name, deleteSpace)}
}

func (_c *MockSpaceComponent_Stop_Call) Run(run func(ctx context.Context, namespace string, name string, deleteSpace bool)) *MockSpaceComponent_Stop_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(bool))
	})
	return _c
}

func (_c *MockSpaceComponent_Stop_Call) Return(_a0 error) *MockSpaceComponent_Stop_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceComponent_Stop_Call) RunAndReturn(run func(context.Context, string, string, bool) error) *MockSpaceComponent_Stop_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, req
func (_m *MockSpaceComponent) Update(ctx context.Context, req *types.UpdateSpaceReq) (*types.Space, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *types.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.UpdateSpaceReq) (*types.Space, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.UpdateSpaceReq) *types.Space); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.UpdateSpaceReq) error); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceComponent_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockSpaceComponent_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.UpdateSpaceReq
func (_e *MockSpaceComponent_Expecter) Update(ctx interface{}, req interface{}) *MockSpaceComponent_Update_Call {
	return &MockSpaceComponent_Update_Call{Call: _e.mock.On("Update", ctx, req)}
}

func (_c *MockSpaceComponent_Update_Call) Run(run func(ctx context.Context, req *types.UpdateSpaceReq)) *MockSpaceComponent_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.UpdateSpaceReq))
	})
	return _c
}

func (_c *MockSpaceComponent_Update_Call) Return(_a0 *types.Space, _a1 error) *MockSpaceComponent_Update_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceComponent_Update_Call) RunAndReturn(run func(context.Context, *types.UpdateSpaceReq) (*types.Space, error)) *MockSpaceComponent_Update_Call {
	_c.Call.Return(run)
	return _c
}

// UserLikesSpaces provides a mock function with given fields: ctx, req, userID
func (_m *MockSpaceComponent) UserLikesSpaces(ctx context.Context, req *types.UserSpacesReq, userID int64) ([]types.Space, int, error) {
	ret := _m.Called(ctx, req, userID)

	if len(ret) == 0 {
		panic("no return value specified for UserLikesSpaces")
	}

	var r0 []types.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.UserSpacesReq, int64) ([]types.Space, int, error)); ok {
		return rf(ctx, req, userID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.UserSpacesReq, int64) []types.Space); ok {
		r0 = rf(ctx, req, userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.UserSpacesReq, int64) int); ok {
		r1 = rf(ctx, req, userID)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.UserSpacesReq, int64) error); ok {
		r2 = rf(ctx, req, userID)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockSpaceComponent_UserLikesSpaces_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UserLikesSpaces'
type MockSpaceComponent_UserLikesSpaces_Call struct {
	*mock.Call
}

// UserLikesSpaces is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.UserSpacesReq
//   - userID int64
func (_e *MockSpaceComponent_Expecter) UserLikesSpaces(ctx interface{}, req interface{}, userID interface{}) *MockSpaceComponent_UserLikesSpaces_Call {
	return &MockSpaceComponent_UserLikesSpaces_Call{Call: _e.mock.On("UserLikesSpaces", ctx, req, userID)}
}

func (_c *MockSpaceComponent_UserLikesSpaces_Call) Run(run func(ctx context.Context, req *types.UserSpacesReq, userID int64)) *MockSpaceComponent_UserLikesSpaces_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.UserSpacesReq), args[2].(int64))
	})
	return _c
}

func (_c *MockSpaceComponent_UserLikesSpaces_Call) Return(_a0 []types.Space, _a1 int, _a2 error) *MockSpaceComponent_UserLikesSpaces_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockSpaceComponent_UserLikesSpaces_Call) RunAndReturn(run func(context.Context, *types.UserSpacesReq, int64) ([]types.Space, int, error)) *MockSpaceComponent_UserLikesSpaces_Call {
	_c.Call.Return(run)
	return _c
}

// UserSpaces provides a mock function with given fields: ctx, req
func (_m *MockSpaceComponent) UserSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	ret := _m.Called(ctx, req)

	if len(ret) == 0 {
		panic("no return value specified for UserSpaces")
	}

	var r0 []types.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.UserSpacesReq) ([]types.Space, int, error)); ok {
		return rf(ctx, req)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.UserSpacesReq) []types.Space); ok {
		r0 = rf(ctx, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.UserSpacesReq) int); ok {
		r1 = rf(ctx, req)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.UserSpacesReq) error); ok {
		r2 = rf(ctx, req)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockSpaceComponent_UserSpaces_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UserSpaces'
type MockSpaceComponent_UserSpaces_Call struct {
	*mock.Call
}

// UserSpaces is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.UserSpacesReq
func (_e *MockSpaceComponent_Expecter) UserSpaces(ctx interface{}, req interface{}) *MockSpaceComponent_UserSpaces_Call {
	return &MockSpaceComponent_UserSpaces_Call{Call: _e.mock.On("UserSpaces", ctx, req)}
}

func (_c *MockSpaceComponent_UserSpaces_Call) Run(run func(ctx context.Context, req *types.UserSpacesReq)) *MockSpaceComponent_UserSpaces_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.UserSpacesReq))
	})
	return _c
}

func (_c *MockSpaceComponent_UserSpaces_Call) Return(_a0 []types.Space, _a1 int, _a2 error) *MockSpaceComponent_UserSpaces_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockSpaceComponent_UserSpaces_Call) RunAndReturn(run func(context.Context, *types.UserSpacesReq) ([]types.Space, int, error)) *MockSpaceComponent_UserSpaces_Call {
	_c.Call.Return(run)
	return _c
}

// Wakeup provides a mock function with given fields: ctx, namespace, name
func (_m *MockSpaceComponent) Wakeup(ctx context.Context, namespace string, name string) error {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for Wakeup")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceComponent_Wakeup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Wakeup'
type MockSpaceComponent_Wakeup_Call struct {
	*mock.Call
}

// Wakeup is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
func (_e *MockSpaceComponent_Expecter) Wakeup(ctx interface{}, namespace interface{}, name interface{}) *MockSpaceComponent_Wakeup_Call {
	return &MockSpaceComponent_Wakeup_Call{Call: _e.mock.On("Wakeup", ctx, namespace, name)}
}

func (_c *MockSpaceComponent_Wakeup_Call) Run(run func(ctx context.Context, namespace string, name string)) *MockSpaceComponent_Wakeup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockSpaceComponent_Wakeup_Call) Return(_a0 error) *MockSpaceComponent_Wakeup_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceComponent_Wakeup_Call) RunAndReturn(run func(context.Context, string, string) error) *MockSpaceComponent_Wakeup_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSpaceComponent creates a new instance of MockSpaceComponent. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSpaceComponent(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSpaceComponent {
	mock := &MockSpaceComponent{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
