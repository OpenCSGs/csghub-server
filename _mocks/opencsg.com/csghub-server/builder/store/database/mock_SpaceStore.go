// Code generated by mockery v2.53.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"

	types "opencsg.com/csghub-server/common/types"
)

// MockSpaceStore is an autogenerated mock type for the SpaceStore type
type MockSpaceStore struct {
	mock.Mock
}

type MockSpaceStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSpaceStore) EXPECT() *MockSpaceStore_Expecter {
	return &MockSpaceStore_Expecter{mock: &_m.Mock}
}

// ByID provides a mock function with given fields: ctx, id
func (_m *MockSpaceStore) ByID(ctx context.Context, id int64) (*database.Space, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for ByID")
	}

	var r0 *database.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.Space, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.Space); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceStore_ByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByID'
type MockSpaceStore_ByID_Call struct {
	*mock.Call
}

// ByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockSpaceStore_Expecter) ByID(ctx interface{}, id interface{}) *MockSpaceStore_ByID_Call {
	return &MockSpaceStore_ByID_Call{Call: _e.mock.On("ByID", ctx, id)}
}

func (_c *MockSpaceStore_ByID_Call) Run(run func(ctx context.Context, id int64)) *MockSpaceStore_ByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockSpaceStore_ByID_Call) Return(_a0 *database.Space, _a1 error) *MockSpaceStore_ByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceStore_ByID_Call) RunAndReturn(run func(context.Context, int64) (*database.Space, error)) *MockSpaceStore_ByID_Call {
	_c.Call.Return(run)
	return _c
}

// ByOrgPath provides a mock function with given fields: ctx, namespace, per, page, onlyPublic
func (_m *MockSpaceStore) ByOrgPath(ctx context.Context, namespace string, per int, page int, onlyPublic bool) ([]database.Space, int, error) {
	ret := _m.Called(ctx, namespace, per, page, onlyPublic)

	if len(ret) == 0 {
		panic("no return value specified for ByOrgPath")
	}

	var r0 []database.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, int, int, bool) ([]database.Space, int, error)); ok {
		return rf(ctx, namespace, per, page, onlyPublic)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, int, int, bool) []database.Space); ok {
		r0 = rf(ctx, namespace, per, page, onlyPublic)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Space)
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

// MockSpaceStore_ByOrgPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByOrgPath'
type MockSpaceStore_ByOrgPath_Call struct {
	*mock.Call
}

// ByOrgPath is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - per int
//   - page int
//   - onlyPublic bool
func (_e *MockSpaceStore_Expecter) ByOrgPath(ctx interface{}, namespace interface{}, per interface{}, page interface{}, onlyPublic interface{}) *MockSpaceStore_ByOrgPath_Call {
	return &MockSpaceStore_ByOrgPath_Call{Call: _e.mock.On("ByOrgPath", ctx, namespace, per, page, onlyPublic)}
}

func (_c *MockSpaceStore_ByOrgPath_Call) Run(run func(ctx context.Context, namespace string, per int, page int, onlyPublic bool)) *MockSpaceStore_ByOrgPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(int), args[3].(int), args[4].(bool))
	})
	return _c
}

func (_c *MockSpaceStore_ByOrgPath_Call) Return(spaces []database.Space, total int, err error) *MockSpaceStore_ByOrgPath_Call {
	_c.Call.Return(spaces, total, err)
	return _c
}

func (_c *MockSpaceStore_ByOrgPath_Call) RunAndReturn(run func(context.Context, string, int, int, bool) ([]database.Space, int, error)) *MockSpaceStore_ByOrgPath_Call {
	_c.Call.Return(run)
	return _c
}

// ByRepoID provides a mock function with given fields: ctx, repoID
func (_m *MockSpaceStore) ByRepoID(ctx context.Context, repoID int64) (*database.Space, error) {
	ret := _m.Called(ctx, repoID)

	if len(ret) == 0 {
		panic("no return value specified for ByRepoID")
	}

	var r0 *database.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.Space, error)); ok {
		return rf(ctx, repoID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.Space); ok {
		r0 = rf(ctx, repoID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, repoID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceStore_ByRepoID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByRepoID'
type MockSpaceStore_ByRepoID_Call struct {
	*mock.Call
}

// ByRepoID is a helper method to define mock.On call
//   - ctx context.Context
//   - repoID int64
func (_e *MockSpaceStore_Expecter) ByRepoID(ctx interface{}, repoID interface{}) *MockSpaceStore_ByRepoID_Call {
	return &MockSpaceStore_ByRepoID_Call{Call: _e.mock.On("ByRepoID", ctx, repoID)}
}

func (_c *MockSpaceStore_ByRepoID_Call) Run(run func(ctx context.Context, repoID int64)) *MockSpaceStore_ByRepoID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockSpaceStore_ByRepoID_Call) Return(_a0 *database.Space, _a1 error) *MockSpaceStore_ByRepoID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceStore_ByRepoID_Call) RunAndReturn(run func(context.Context, int64) (*database.Space, error)) *MockSpaceStore_ByRepoID_Call {
	_c.Call.Return(run)
	return _c
}

// ByRepoIDs provides a mock function with given fields: ctx, repoIDs
func (_m *MockSpaceStore) ByRepoIDs(ctx context.Context, repoIDs []int64) ([]database.Space, error) {
	ret := _m.Called(ctx, repoIDs)

	if len(ret) == 0 {
		panic("no return value specified for ByRepoIDs")
	}

	var r0 []database.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []int64) ([]database.Space, error)); ok {
		return rf(ctx, repoIDs)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []int64) []database.Space); ok {
		r0 = rf(ctx, repoIDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []int64) error); ok {
		r1 = rf(ctx, repoIDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceStore_ByRepoIDs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByRepoIDs'
type MockSpaceStore_ByRepoIDs_Call struct {
	*mock.Call
}

// ByRepoIDs is a helper method to define mock.On call
//   - ctx context.Context
//   - repoIDs []int64
func (_e *MockSpaceStore_Expecter) ByRepoIDs(ctx interface{}, repoIDs interface{}) *MockSpaceStore_ByRepoIDs_Call {
	return &MockSpaceStore_ByRepoIDs_Call{Call: _e.mock.On("ByRepoIDs", ctx, repoIDs)}
}

func (_c *MockSpaceStore_ByRepoIDs_Call) Run(run func(ctx context.Context, repoIDs []int64)) *MockSpaceStore_ByRepoIDs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]int64))
	})
	return _c
}

func (_c *MockSpaceStore_ByRepoIDs_Call) Return(spaces []database.Space, err error) *MockSpaceStore_ByRepoIDs_Call {
	_c.Call.Return(spaces, err)
	return _c
}

func (_c *MockSpaceStore_ByRepoIDs_Call) RunAndReturn(run func(context.Context, []int64) ([]database.Space, error)) *MockSpaceStore_ByRepoIDs_Call {
	_c.Call.Return(run)
	return _c
}

// ByUserLikes provides a mock function with given fields: ctx, userID, per, page
func (_m *MockSpaceStore) ByUserLikes(ctx context.Context, userID int64, per int, page int) ([]database.Space, int, error) {
	ret := _m.Called(ctx, userID, per, page)

	if len(ret) == 0 {
		panic("no return value specified for ByUserLikes")
	}

	var r0 []database.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, int, int) ([]database.Space, int, error)); ok {
		return rf(ctx, userID, per, page)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64, int, int) []database.Space); ok {
		r0 = rf(ctx, userID, per, page)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Space)
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

// MockSpaceStore_ByUserLikes_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByUserLikes'
type MockSpaceStore_ByUserLikes_Call struct {
	*mock.Call
}

// ByUserLikes is a helper method to define mock.On call
//   - ctx context.Context
//   - userID int64
//   - per int
//   - page int
func (_e *MockSpaceStore_Expecter) ByUserLikes(ctx interface{}, userID interface{}, per interface{}, page interface{}) *MockSpaceStore_ByUserLikes_Call {
	return &MockSpaceStore_ByUserLikes_Call{Call: _e.mock.On("ByUserLikes", ctx, userID, per, page)}
}

func (_c *MockSpaceStore_ByUserLikes_Call) Run(run func(ctx context.Context, userID int64, per int, page int)) *MockSpaceStore_ByUserLikes_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64), args[2].(int), args[3].(int))
	})
	return _c
}

func (_c *MockSpaceStore_ByUserLikes_Call) Return(spaces []database.Space, total int, err error) *MockSpaceStore_ByUserLikes_Call {
	_c.Call.Return(spaces, total, err)
	return _c
}

func (_c *MockSpaceStore_ByUserLikes_Call) RunAndReturn(run func(context.Context, int64, int, int) ([]database.Space, int, error)) *MockSpaceStore_ByUserLikes_Call {
	_c.Call.Return(run)
	return _c
}

// ByUsername provides a mock function with given fields: ctx, req, onlyPublic
func (_m *MockSpaceStore) ByUsername(ctx context.Context, req *types.UserSpacesReq, onlyPublic bool) ([]database.Space, int, error) {
	ret := _m.Called(ctx, req, onlyPublic)

	if len(ret) == 0 {
		panic("no return value specified for ByUsername")
	}

	var r0 []database.Space
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.UserSpacesReq, bool) ([]database.Space, int, error)); ok {
		return rf(ctx, req, onlyPublic)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.UserSpacesReq, bool) []database.Space); ok {
		r0 = rf(ctx, req, onlyPublic)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.UserSpacesReq, bool) int); ok {
		r1 = rf(ctx, req, onlyPublic)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.UserSpacesReq, bool) error); ok {
		r2 = rf(ctx, req, onlyPublic)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockSpaceStore_ByUsername_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ByUsername'
type MockSpaceStore_ByUsername_Call struct {
	*mock.Call
}

// ByUsername is a helper method to define mock.On call
//   - ctx context.Context
//   - req *types.UserSpacesReq
//   - onlyPublic bool
func (_e *MockSpaceStore_Expecter) ByUsername(ctx interface{}, req interface{}, onlyPublic interface{}) *MockSpaceStore_ByUsername_Call {
	return &MockSpaceStore_ByUsername_Call{Call: _e.mock.On("ByUsername", ctx, req, onlyPublic)}
}

func (_c *MockSpaceStore_ByUsername_Call) Run(run func(ctx context.Context, req *types.UserSpacesReq, onlyPublic bool)) *MockSpaceStore_ByUsername_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*types.UserSpacesReq), args[2].(bool))
	})
	return _c
}

func (_c *MockSpaceStore_ByUsername_Call) Return(spaces []database.Space, total int, err error) *MockSpaceStore_ByUsername_Call {
	_c.Call.Return(spaces, total, err)
	return _c
}

func (_c *MockSpaceStore_ByUsername_Call) RunAndReturn(run func(context.Context, *types.UserSpacesReq, bool) ([]database.Space, int, error)) *MockSpaceStore_ByUsername_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function with given fields: ctx, input
func (_m *MockSpaceStore) Create(ctx context.Context, input database.Space) (*database.Space, error) {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *database.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Space) (*database.Space, error)); ok {
		return rf(ctx, input)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.Space) *database.Space); ok {
		r0 = rf(ctx, input)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.Space) error); ok {
		r1 = rf(ctx, input)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceStore_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockSpaceStore_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.Space
func (_e *MockSpaceStore_Expecter) Create(ctx interface{}, input interface{}) *MockSpaceStore_Create_Call {
	return &MockSpaceStore_Create_Call{Call: _e.mock.On("Create", ctx, input)}
}

func (_c *MockSpaceStore_Create_Call) Run(run func(ctx context.Context, input database.Space)) *MockSpaceStore_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Space))
	})
	return _c
}

func (_c *MockSpaceStore_Create_Call) Return(_a0 *database.Space, _a1 error) *MockSpaceStore_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceStore_Create_Call) RunAndReturn(run func(context.Context, database.Space) (*database.Space, error)) *MockSpaceStore_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, input
func (_m *MockSpaceStore) Delete(ctx context.Context, input database.Space) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Space) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceStore_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockSpaceStore_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.Space
func (_e *MockSpaceStore_Expecter) Delete(ctx interface{}, input interface{}) *MockSpaceStore_Delete_Call {
	return &MockSpaceStore_Delete_Call{Call: _e.mock.On("Delete", ctx, input)}
}

func (_c *MockSpaceStore_Delete_Call) Run(run func(ctx context.Context, input database.Space)) *MockSpaceStore_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Space))
	})
	return _c
}

func (_c *MockSpaceStore_Delete_Call) Return(_a0 error) *MockSpaceStore_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSpaceStore_Delete_Call) RunAndReturn(run func(context.Context, database.Space) error) *MockSpaceStore_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// FindByPath provides a mock function with given fields: ctx, namespace, name
func (_m *MockSpaceStore) FindByPath(ctx context.Context, namespace string, name string) (*database.Space, error) {
	ret := _m.Called(ctx, namespace, name)

	if len(ret) == 0 {
		panic("no return value specified for FindByPath")
	}

	var r0 *database.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*database.Space, error)); ok {
		return rf(ctx, namespace, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *database.Space); ok {
		r0 = rf(ctx, namespace, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, namespace, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceStore_FindByPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByPath'
type MockSpaceStore_FindByPath_Call struct {
	*mock.Call
}

// FindByPath is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - name string
func (_e *MockSpaceStore_Expecter) FindByPath(ctx interface{}, namespace interface{}, name interface{}) *MockSpaceStore_FindByPath_Call {
	return &MockSpaceStore_FindByPath_Call{Call: _e.mock.On("FindByPath", ctx, namespace, name)}
}

func (_c *MockSpaceStore_FindByPath_Call) Run(run func(ctx context.Context, namespace string, name string)) *MockSpaceStore_FindByPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockSpaceStore_FindByPath_Call) Return(_a0 *database.Space, _a1 error) *MockSpaceStore_FindByPath_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceStore_FindByPath_Call) RunAndReturn(run func(context.Context, string, string) (*database.Space, error)) *MockSpaceStore_FindByPath_Call {
	_c.Call.Return(run)
	return _c
}

// ListByPath provides a mock function with given fields: ctx, paths
func (_m *MockSpaceStore) ListByPath(ctx context.Context, paths []string) ([]database.Space, error) {
	ret := _m.Called(ctx, paths)

	if len(ret) == 0 {
		panic("no return value specified for ListByPath")
	}

	var r0 []database.Space
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]database.Space, error)); ok {
		return rf(ctx, paths)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []database.Space); ok {
		r0 = rf(ctx, paths)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.Space)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, paths)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSpaceStore_ListByPath_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListByPath'
type MockSpaceStore_ListByPath_Call struct {
	*mock.Call
}

// ListByPath is a helper method to define mock.On call
//   - ctx context.Context
//   - paths []string
func (_e *MockSpaceStore_Expecter) ListByPath(ctx interface{}, paths interface{}) *MockSpaceStore_ListByPath_Call {
	return &MockSpaceStore_ListByPath_Call{Call: _e.mock.On("ListByPath", ctx, paths)}
}

func (_c *MockSpaceStore_ListByPath_Call) Run(run func(ctx context.Context, paths []string)) *MockSpaceStore_ListByPath_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]string))
	})
	return _c
}

func (_c *MockSpaceStore_ListByPath_Call) Return(_a0 []database.Space, _a1 error) *MockSpaceStore_ListByPath_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSpaceStore_ListByPath_Call) RunAndReturn(run func(context.Context, []string) ([]database.Space, error)) *MockSpaceStore_ListByPath_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, input
func (_m *MockSpaceStore) Update(ctx context.Context, input database.Space) error {
	ret := _m.Called(ctx, input)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.Space) error); ok {
		r0 = rf(ctx, input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSpaceStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockSpaceStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - input database.Space
func (_e *MockSpaceStore_Expecter) Update(ctx interface{}, input interface{}) *MockSpaceStore_Update_Call {
	return &MockSpaceStore_Update_Call{Call: _e.mock.On("Update", ctx, input)}
}

func (_c *MockSpaceStore_Update_Call) Run(run func(ctx context.Context, input database.Space)) *MockSpaceStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.Space))
	})
	return _c
}

func (_c *MockSpaceStore_Update_Call) Return(err error) *MockSpaceStore_Update_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockSpaceStore_Update_Call) RunAndReturn(run func(context.Context, database.Space) error) *MockSpaceStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSpaceStore creates a new instance of MockSpaceStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSpaceStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSpaceStore {
	mock := &MockSpaceStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
