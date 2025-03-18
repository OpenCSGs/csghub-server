// Code generated by mockery v2.53.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockCodeStore is an autogenerated mock type for the CodeStore type
type MockCodeStore struct {
	mock.Mock
}

type MockCodeStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockCodeStore) EXPECT() *MockCodeStore_Expecter {
	return &MockCodeStore_Expecter{mock: &_m.Mock}
}

// ByOrgPath provides a mock function with given fields: ctx, namespace, per, page, onlyPublic
func (_m *MockCodeStore) ByOrgPath(ctx context.Context, namespace string, per int, page int, onlyPublic bool) ([]database.Code, int, error) {
	ret := _m.Called(ctx, namespace, per, page, onlyPublic)

	if len(ret) == 0 {
		panic("no return value specified for ByOrgPath")
	}

	var r0 []database.Code
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int, int, bool) ([]database.Code, int, error)); ok {
		return rf(ctx, namespace, per, page, onlyPublic)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int, int, bool) []database.Code); ok {
		r0 = rf(ctx, namespace, per, page, onlyPublic)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, int, int, bool) int); ok {
		r1 = rf(ctx, namespace, per, page, onlyPublic)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, int, int, bool) error); ok {
		r2 = rf(ctx, namespace, per, page, onlyPublic)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockCodeStore_ByOrgPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByOrgPath'
type MockCodeStore_ByOrgPath_Call struct {
	*mock.Call
}

// ByOrgPath is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - per int
//   - page int
//   - onlyPublic bool
func (_e *MockCodeStore_Expecter) ByOrgPath(ctx interface{}, namespace interface{}, per interface{}, page interface{}, onlyPublic interface{}) *MockCodeStore_ByOrgPath_Call {
	return &MockCodeStore_ByOrgPath_Call{Call: _e.mock.On("ByOrgPath", ctx, namespace, per, page, onlyPublic)}
}

func (_c *MockCodeStore_ByOrgPath_Call) Run(run func(ctx context.Context, namespace string, per int, page int, onlyPublic bool)) *MockCodeStore_ByOrgPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(int), args[3].(int), args[4].(bool))
	})
	return _c
}

func (_c *MockCodeStore_ByOrgPath_Call) Return(codes []database.Code, total int, err error) *MockCodeStore_ByOrgPath_Call {
	_c.Call.Return(codes, total, err)
	return _c
}

func (_c *MockCodeStore_ByOrgPath_Call) RunAndReturn(run func(context.Context, string, int, int, bool) ([]database.Code, int, error)) *MockCodeStore_ByOrgPath_Call {
	_c.Call.Return(run)
	return _c
}

// ByRepoID provides a mock function with given fields: ctx, repoID
func (_m *MockCodeStore) ByRepoID(ctx context.Context, repoID int64) (*database.Code, error) {
	ret := _m.Called(ctx, repoID)

	if len(ret) == 0 {
		panic("no return value specified for ByRepoID")
	}

	var r0 *database.Code
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.Code, error)); ok {
		return rf(ctx, repoID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.Code); ok {
		r0 = rf(ctx, repoID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, repoID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCodeStore_ByRepoID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByRepoID'
type MockCodeStore_ByRepoID_Call struct {
	*mock.Call
}

// ByRepoID is a helper method to define mock.On call
//   - ctx context.Context
//   - repoID int64
func (_e *MockCodeStore_Expecter) ByRepoID(ctx interface{}, repoID interface{}) *MockCodeStore_ByRepoID_Call {
	return &MockCodeStore_ByRepoID_Call{Call: _e.mock.On("ByRepoID", ctx, repoID)}
}

func (_c *MockCodeStore_ByRepoID_Call) Run(run func(ctx context.Context, repoID int64)) *MockCodeStore_ByRepoID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockCodeStore_ByRepoID_Call) Return(_a0 *database.Code, _a1 error) *MockCodeStore_ByRepoID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCodeStore_ByRepoID_Call) RunAndReturn(run func(context.Context, int64) (*database.Code, error)) *MockCodeStore_ByRepoID_Call {
	_c.Call.Return(run)
	return _c
}

// ByRepoIDs provides a mock function with given fields: ctx, repoIDs
func (_m *MockCodeStore) ByRepoIDs(ctx context.Context, repoIDs []int64) ([]database.Code, error) {
	ret := _m.Called(ctx, repoIDs)

	if len(ret) == 0 {
		panic("no return value specified for ByRepoIDs")
	}

	var r0 []database.Code
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []int64) ([]database.Code, error)); ok {
		return rf(ctx, repoIDs)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []int64) []database.Code); ok {
		r0 = rf(ctx, repoIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []int64) error); ok {
		r1 = rf(ctx, repoIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCodeStore_ByRepoIDs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByRepoIDs'
type MockCodeStore_ByRepoIDs_Call struct {
	*mock.Call
}

// ByRepoIDs is a helper method to define mock.On call
//   - ctx context.Context
//   - repoIDs []int64
func (_e *MockCodeStore_Expecter) ByRepoIDs(ctx interface{}, repoIDs interface{}) *MockCodeStore_ByRepoIDs_Call {
	return &MockCodeStore_ByRepoIDs_Call{Call: _e.mock.On("ByRepoIDs", ctx, repoIDs)}
}

func (_c *MockCodeStore_ByRepoIDs_Call) Run(run func(ctx context.Context, repoIDs []int64)) *MockCodeStore_ByRepoIDs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]int64))
	})
	return _c
}

func (_c *MockCodeStore_ByRepoIDs_Call) Return(codes []database.Code, err error) *MockCodeStore_ByRepoIDs_Call {
	_c.Call.Return(codes, err)
	return _c
}

func (_c *MockCodeStore_ByRepoIDs_Call) RunAndReturn(run func(context.Context, []int64) ([]database.Code, error)) *MockCodeStore_ByRepoIDs_Call {
	_c.Call.Return(run)
	return _c
}

// ByUsername provides a mock function with given fields: ctx, username, per, page, onlyPublic
func (_m *MockCodeStore) ByUsername(ctx context.Context, username string, per int, page int, onlyPublic bool) ([]database.Code, int, error) {
	ret := _m.Called(ctx, username, per, page, onlyPublic)

	if len(ret) == 0 {
		panic("no return value specified for ByUsername")
	}

	var r0 []database.Code
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int, int, bool) ([]database.Code, int, error)); ok {
		return rf(ctx, username, per, page, onlyPublic)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int, int, bool) []database.Code); ok {
		r0 = rf(ctx, username, per, page, onlyPublic)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, int, int, bool) int); ok {
		r1 = rf(ctx, username, per, page, onlyPublic)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, int, int, bool) error); ok {
		r2 = rf(ctx, username, per, page, onlyPublic)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockCodeStore_ByUsername_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByUsername'
type MockCodeStore_ByUsername_Call struct {
	*mock.Call
}

// ByUsername is a helper method to define mock.On call
//   - ctx context.Context
//   - username string
//   - per int
//   - page int
//   - onlyPublic bool
func (_e *MockCodeStore_Expecter) ByUsername(ctx interface{}, username interface{}, per interface{}, page interface{}, onlyPublic interface{}) *MockCodeStore_ByUsername_Call {
	return &MockCodeStore_ByUsername_Call{Call: _e.mock.On("ByUsername", ctx, username, per, page, onlyPublic)}
}

func (_c *MockCodeStore_ByUsername_Call) Run(run func(ctx context.Context, username string, per int, page int, onlyPublic bool)) *MockCodeStore_ByUsername_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(int), args[3].(int), args[4].(bool))
	})
	return _c
}

func (_c *MockCodeStore_ByUsername_Call) Return(codes []database.Code, total int, err error) *MockCodeStore_ByUsername_Call {
	_c.Call.Return(codes, total, err)
	return _c
}

func (_c *MockCodeStore_ByUsername_Call) RunAndReturn(run func(context.Context, string, int, int, bool) ([]database.Code, int, error)) *MockCodeStore_ByUsername_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function with given fields: ctx, input
func (_m *MockCodeStore) Create(ctx context.Context, input database.Code) (*database.Code, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *database.Code
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Code) (*database.Code, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.Code) *database.Code); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.Code) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCodeStore_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockCodeStore_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.Code
func (_e *MockCodeStore_Expecter) Create(ctx interface{}, input interface{}) *MockCodeStore_Create_Call {
	return &MockCodeStore_Create_Call{Call: _e.mock.On("Create", ctx, input)}
}

func (_c *MockCodeStore_Create_Call) Run(run func(ctx context.Context, input database.Code)) *MockCodeStore_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Code))
	})
	return _c
}

func (_c *MockCodeStore_Create_Call) Return(_a0 *database.Code, _a1 error) *MockCodeStore_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCodeStore_Create_Call) RunAndReturn(run func(context.Context, database.Code) (*database.Code, error)) *MockCodeStore_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, input
func (_m *MockCodeStore) Delete(ctx context.Context, input database.Code) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Code) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCodeStore_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockCodeStore_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.Code
func (_e *MockCodeStore_Expecter) Delete(ctx interface{}, input interface{}) *MockCodeStore_Delete_Call {
	return &MockCodeStore_Delete_Call{Call: _e.mock.On("Delete", ctx, input)}
}

func (_c *MockCodeStore_Delete_Call) Run(run func(ctx context.Context, input database.Code)) *MockCodeStore_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Code))
	})
	return _c
}

func (_c *MockCodeStore_Delete_Call) Return(_a0 error) *MockCodeStore_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCodeStore_Delete_Call) RunAndReturn(run func(context.Context, database.Code) error) *MockCodeStore_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// FindByPath provides a mock function with given fields: ctx, namespace, repoPath
func (_m *MockCodeStore) FindByPath(ctx context.Context, namespace string, repoPath string) (*database.Code, error) {
	ret := _m.Called(ctx, namespace, repoPath)

	if len(ret) == 0 {
		panic("no return value specified for FindByPath")
	}

	var r0 *database.Code
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*database.Code, error)); ok {
		return rf(ctx, namespace, repoPath)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *database.Code); ok {
		r0 = rf(ctx, namespace, repoPath)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, repoPath)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCodeStore_FindByPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByPath'
type MockCodeStore_FindByPath_Call struct {
	*mock.Call
}

// FindByPath is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - repoPath string
func (_e *MockCodeStore_Expecter) FindByPath(ctx interface{}, namespace interface{}, repoPath interface{}) *MockCodeStore_FindByPath_Call {
	return &MockCodeStore_FindByPath_Call{Call: _e.mock.On("FindByPath", ctx, namespace, repoPath)}
}

func (_c *MockCodeStore_FindByPath_Call) Run(run func(ctx context.Context, namespace string, repoPath string)) *MockCodeStore_FindByPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockCodeStore_FindByPath_Call) Return(code *database.Code, err error) *MockCodeStore_FindByPath_Call {
	_c.Call.Return(code, err)
	return _c
}

func (_c *MockCodeStore_FindByPath_Call) RunAndReturn(run func(context.Context, string, string) (*database.Code, error)) *MockCodeStore_FindByPath_Call {
	_c.Call.Return(run)
	return _c
}

// ListByPath provides a mock function with given fields: ctx, paths
func (_m *MockCodeStore) ListByPath(ctx context.Context, paths []string) ([]database.Code, error) {
	ret := _m.Called(ctx, paths)

	if len(ret) == 0 {
		panic("no return value specified for ListByPath")
	}

	var r0 []database.Code
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]database.Code, error)); ok {
		return rf(ctx, paths)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []database.Code); ok {
		r0 = rf(ctx, paths)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, paths)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockCodeStore_ListByPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListByPath'
type MockCodeStore_ListByPath_Call struct {
	*mock.Call
}

// ListByPath is a helper method to define mock.On call
//   - ctx context.Context
//   - paths []string
func (_e *MockCodeStore_Expecter) ListByPath(ctx interface{}, paths interface{}) *MockCodeStore_ListByPath_Call {
	return &MockCodeStore_ListByPath_Call{Call: _e.mock.On("ListByPath", ctx, paths)}
}

func (_c *MockCodeStore_ListByPath_Call) Run(run func(ctx context.Context, paths []string)) *MockCodeStore_ListByPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]string))
	})
	return _c
}

func (_c *MockCodeStore_ListByPath_Call) Return(_a0 []database.Code, _a1 error) *MockCodeStore_ListByPath_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockCodeStore_ListByPath_Call) RunAndReturn(run func(context.Context, []string) ([]database.Code, error)) *MockCodeStore_ListByPath_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, input
func (_m *MockCodeStore) Update(ctx context.Context, input database.Code) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Code) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockCodeStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockCodeStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.Code
func (_e *MockCodeStore_Expecter) Update(ctx interface{}, input interface{}) *MockCodeStore_Update_Call {
	return &MockCodeStore_Update_Call{Call: _e.mock.On("Update", ctx, input)}
}

func (_c *MockCodeStore_Update_Call) Run(run func(ctx context.Context, input database.Code)) *MockCodeStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Code))
	})
	return _c
}

func (_c *MockCodeStore_Update_Call) Return(err error) *MockCodeStore_Update_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockCodeStore_Update_Call) RunAndReturn(run func(context.Context, database.Code) error) *MockCodeStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// UserLikesCodes provides a mock function with given fields: ctx, userID, per, page
func (_m *MockCodeStore) UserLikesCodes(ctx context.Context, userID int64, per int, page int) ([]database.Code, int, error) {
	ret := _m.Called(ctx, userID, per, page)

	if len(ret) == 0 {
		panic("no return value specified for UserLikesCodes")
	}

	var r0 []database.Code
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, int, int) ([]database.Code, int, error)); ok {
		return rf(ctx, userID, per, page)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64, int, int) []database.Code); ok {
		r0 = rf(ctx, userID, per, page)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Code)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64, int, int) int); ok {
		r1 = rf(ctx, userID, per, page)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, int64, int, int) error); ok {
		r2 = rf(ctx, userID, per, page)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockCodeStore_UserLikesCodes_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UserLikesCodes'
type MockCodeStore_UserLikesCodes_Call struct {
	*mock.Call
}

// UserLikesCodes is a helper method to define mock.On call
//   - ctx context.Context
//   - userID int64
//   - per int
//   - page int
func (_e *MockCodeStore_Expecter) UserLikesCodes(ctx interface{}, userID interface{}, per interface{}, page interface{}) *MockCodeStore_UserLikesCodes_Call {
	return &MockCodeStore_UserLikesCodes_Call{Call: _e.mock.On("UserLikesCodes", ctx, userID, per, page)}
}

func (_c *MockCodeStore_UserLikesCodes_Call) Run(run func(ctx context.Context, userID int64, per int, page int)) *MockCodeStore_UserLikesCodes_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64), args[2].(int), args[3].(int))
	})
	return _c
}

func (_c *MockCodeStore_UserLikesCodes_Call) Return(codes []database.Code, total int, err error) *MockCodeStore_UserLikesCodes_Call {
	_c.Call.Return(codes, total, err)
	return _c
}

func (_c *MockCodeStore_UserLikesCodes_Call) RunAndReturn(run func(context.Context, int64, int, int) ([]database.Code, int, error)) *MockCodeStore_UserLikesCodes_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockCodeStore creates a new instance of MockCodeStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockCodeStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCodeStore {
	mock := &MockCodeStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
