// Code generated by mockery v2.53.0. DO NOT EDIT.

package database

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	database "opencsg.com/csghub-server/builder/store/database"
)

// MockRuntimeFrameworksStore is an autogenerated mock type for the RuntimeFrameworksStore type
type MockRuntimeFrameworksStore struct {
	mock.Mock
}

type MockRuntimeFrameworksStore_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRuntimeFrameworksStore) EXPECT() *MockRuntimeFrameworksStore_Expecter {
	return &MockRuntimeFrameworksStore_Expecter{mock: &_m.Mock}
}

// Add provides a mock function with given fields: ctx, frame
func (_m *MockRuntimeFrameworksStore) Add(ctx context.Context, frame database.RuntimeFramework) (*database.RuntimeFramework, error) {
	ret := _m.Called(ctx, frame)

	if len(ret) == 0 {
		panic("no return value specified for Add")
	}

	var r0 *database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.RuntimeFramework) (*database.RuntimeFramework, error)); ok {
		return rf(ctx, frame)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.RuntimeFramework) *database.RuntimeFramework); ok {
		r0 = rf(ctx, frame)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.RuntimeFramework) error); ok {
		r1 = rf(ctx, frame)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_Add_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Add'
type MockRuntimeFrameworksStore_Add_Call struct {
	*mock.Call
}

// Add is a helper method to define mock.On call
//   - ctx context.Context
//   - frame database.RuntimeFramework
func (_e *MockRuntimeFrameworksStore_Expecter) Add(ctx interface{}, frame interface{}) *MockRuntimeFrameworksStore_Add_Call {
	return &MockRuntimeFrameworksStore_Add_Call{Call: _e.mock.On("Add", ctx, frame)}
}

func (_c *MockRuntimeFrameworksStore_Add_Call) Run(run func(ctx context.Context, frame database.RuntimeFramework)) *MockRuntimeFrameworksStore_Add_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.RuntimeFramework))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_Add_Call) Return(_a0 *database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_Add_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_Add_Call) RunAndReturn(run func(context.Context, database.RuntimeFramework) (*database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_Add_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, frame
func (_m *MockRuntimeFrameworksStore) Delete(ctx context.Context, frame database.RuntimeFramework) error {
	ret := _m.Called(ctx, frame)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.RuntimeFramework) error); ok {
		r0 = rf(ctx, frame)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRuntimeFrameworksStore_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type MockRuntimeFrameworksStore_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - frame database.RuntimeFramework
func (_e *MockRuntimeFrameworksStore_Expecter) Delete(ctx interface{}, frame interface{}) *MockRuntimeFrameworksStore_Delete_Call {
	return &MockRuntimeFrameworksStore_Delete_Call{Call: _e.mock.On("Delete", ctx, frame)}
}

func (_c *MockRuntimeFrameworksStore_Delete_Call) Run(run func(ctx context.Context, frame database.RuntimeFramework)) *MockRuntimeFrameworksStore_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.RuntimeFramework))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_Delete_Call) Return(_a0 error) *MockRuntimeFrameworksStore_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRuntimeFrameworksStore_Delete_Call) RunAndReturn(run func(context.Context, database.RuntimeFramework) error) *MockRuntimeFrameworksStore_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// FindByFrameImageAndComputeType provides a mock function with given fields: ctx, frameImage, ComputeType
func (_m *MockRuntimeFrameworksStore) FindByFrameImageAndComputeType(ctx context.Context, frameImage string, ComputeType string) (*database.RuntimeFramework, error) {
	ret := _m.Called(ctx, frameImage, ComputeType)

	if len(ret) == 0 {
		panic("no return value specified for FindByFrameImageAndComputeType")
	}

	var r0 *database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*database.RuntimeFramework, error)); ok {
		return rf(ctx, frameImage, ComputeType)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *database.RuntimeFramework); ok {
		r0 = rf(ctx, frameImage, ComputeType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, frameImage, ComputeType)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByFrameImageAndComputeType'
type MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call struct {
	*mock.Call
}

// FindByFrameImageAndComputeType is a helper method to define mock.On call
//   - ctx context.Context
//   - frameImage string
//   - ComputeType string
func (_e *MockRuntimeFrameworksStore_Expecter) FindByFrameImageAndComputeType(ctx interface{}, frameImage interface{}, ComputeType interface{}) *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call {
	return &MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call{Call: _e.mock.On("FindByFrameImageAndComputeType", ctx, frameImage, ComputeType)}
}

func (_c *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call) Run(run func(ctx context.Context, frameImage string, ComputeType string)) *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call) Return(_a0 *database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call) RunAndReturn(run func(context.Context, string, string) (*database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_FindByFrameImageAndComputeType_Call {
	_c.Call.Return(run)
	return _c
}

// FindByFrameName provides a mock function with given fields: ctx, name
func (_m *MockRuntimeFrameworksStore) FindByFrameName(ctx context.Context, name string) ([]database.RuntimeFramework, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for FindByFrameName")
	}

	var r0 []database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]database.RuntimeFramework, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []database.RuntimeFramework); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_FindByFrameName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByFrameName'
type MockRuntimeFrameworksStore_FindByFrameName_Call struct {
	*mock.Call
}

// FindByFrameName is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
func (_e *MockRuntimeFrameworksStore_Expecter) FindByFrameName(ctx interface{}, name interface{}) *MockRuntimeFrameworksStore_FindByFrameName_Call {
	return &MockRuntimeFrameworksStore_FindByFrameName_Call{Call: _e.mock.On("FindByFrameName", ctx, name)}
}

func (_c *MockRuntimeFrameworksStore_FindByFrameName_Call) Run(run func(ctx context.Context, name string)) *MockRuntimeFrameworksStore_FindByFrameName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindByFrameName_Call) Return(_a0 []database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_FindByFrameName_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindByFrameName_Call) RunAndReturn(run func(context.Context, string) ([]database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_FindByFrameName_Call {
	_c.Call.Return(run)
	return _c
}

// FindByID provides a mock function with given fields: ctx, id
func (_m *MockRuntimeFrameworksStore) FindByID(ctx context.Context, id int64) (*database.RuntimeFramework, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for FindByID")
	}

	var r0 *database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.RuntimeFramework, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.RuntimeFramework); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_FindByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindByID'
type MockRuntimeFrameworksStore_FindByID_Call struct {
	*mock.Call
}

// FindByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockRuntimeFrameworksStore_Expecter) FindByID(ctx interface{}, id interface{}) *MockRuntimeFrameworksStore_FindByID_Call {
	return &MockRuntimeFrameworksStore_FindByID_Call{Call: _e.mock.On("FindByID", ctx, id)}
}

func (_c *MockRuntimeFrameworksStore_FindByID_Call) Run(run func(ctx context.Context, id int64)) *MockRuntimeFrameworksStore_FindByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindByID_Call) Return(_a0 *database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_FindByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindByID_Call) RunAndReturn(run func(context.Context, int64) (*database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_FindByID_Call {
	_c.Call.Return(run)
	return _c
}

// FindEnabledByID provides a mock function with given fields: ctx, id
func (_m *MockRuntimeFrameworksStore) FindEnabledByID(ctx context.Context, id int64) (*database.RuntimeFramework, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for FindEnabledByID")
	}

	var r0 *database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) (*database.RuntimeFramework, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int64) *database.RuntimeFramework); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_FindEnabledByID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindEnabledByID'
type MockRuntimeFrameworksStore_FindEnabledByID_Call struct {
	*mock.Call
}

// FindEnabledByID is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockRuntimeFrameworksStore_Expecter) FindEnabledByID(ctx interface{}, id interface{}) *MockRuntimeFrameworksStore_FindEnabledByID_Call {
	return &MockRuntimeFrameworksStore_FindEnabledByID_Call{Call: _e.mock.On("FindEnabledByID", ctx, id)}
}

func (_c *MockRuntimeFrameworksStore_FindEnabledByID_Call) Run(run func(ctx context.Context, id int64)) *MockRuntimeFrameworksStore_FindEnabledByID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindEnabledByID_Call) Return(_a0 *database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_FindEnabledByID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindEnabledByID_Call) RunAndReturn(run func(context.Context, int64) (*database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_FindEnabledByID_Call {
	_c.Call.Return(run)
	return _c
}

// FindEnabledByName provides a mock function with given fields: ctx, name
func (_m *MockRuntimeFrameworksStore) FindEnabledByName(ctx context.Context, name string) (*database.RuntimeFramework, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for FindEnabledByName")
	}

	var r0 *database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*database.RuntimeFramework, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *database.RuntimeFramework); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_FindEnabledByName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindEnabledByName'
type MockRuntimeFrameworksStore_FindEnabledByName_Call struct {
	*mock.Call
}

// FindEnabledByName is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
func (_e *MockRuntimeFrameworksStore_Expecter) FindEnabledByName(ctx interface{}, name interface{}) *MockRuntimeFrameworksStore_FindEnabledByName_Call {
	return &MockRuntimeFrameworksStore_FindEnabledByName_Call{Call: _e.mock.On("FindEnabledByName", ctx, name)}
}

func (_c *MockRuntimeFrameworksStore_FindEnabledByName_Call) Run(run func(ctx context.Context, name string)) *MockRuntimeFrameworksStore_FindEnabledByName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindEnabledByName_Call) Return(_a0 *database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_FindEnabledByName_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_FindEnabledByName_Call) RunAndReturn(run func(context.Context, string) (*database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_FindEnabledByName_Call {
	_c.Call.Return(run)
	return _c
}

// List provides a mock function with given fields: ctx, deployType
func (_m *MockRuntimeFrameworksStore) List(ctx context.Context, deployType int) ([]database.RuntimeFramework, error) {
	ret := _m.Called(ctx, deployType)

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 []database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) ([]database.RuntimeFramework, error)); ok {
		return rf(ctx, deployType)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) []database.RuntimeFramework); ok {
		r0 = rf(ctx, deployType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, deployType)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_List_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'List'
type MockRuntimeFrameworksStore_List_Call struct {
	*mock.Call
}

// List is a helper method to define mock.On call
//   - ctx context.Context
//   - deployType int
func (_e *MockRuntimeFrameworksStore_Expecter) List(ctx interface{}, deployType interface{}) *MockRuntimeFrameworksStore_List_Call {
	return &MockRuntimeFrameworksStore_List_Call{Call: _e.mock.On("List", ctx, deployType)}
}

func (_c *MockRuntimeFrameworksStore_List_Call) Run(run func(ctx context.Context, deployType int)) *MockRuntimeFrameworksStore_List_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_List_Call) Return(_a0 []database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_List_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_List_Call) RunAndReturn(run func(context.Context, int) ([]database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_List_Call {
	_c.Call.Return(run)
	return _c
}

// ListAll provides a mock function with given fields: ctx
func (_m *MockRuntimeFrameworksStore) ListAll(ctx context.Context) ([]database.RuntimeFramework, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListAll")
	}

	var r0 []database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]database.RuntimeFramework, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []database.RuntimeFramework); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_ListAll_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListAll'
type MockRuntimeFrameworksStore_ListAll_Call struct {
	*mock.Call
}

// ListAll is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockRuntimeFrameworksStore_Expecter) ListAll(ctx interface{}) *MockRuntimeFrameworksStore_ListAll_Call {
	return &MockRuntimeFrameworksStore_ListAll_Call{Call: _e.mock.On("ListAll", ctx)}
}

func (_c *MockRuntimeFrameworksStore_ListAll_Call) Run(run func(ctx context.Context)) *MockRuntimeFrameworksStore_ListAll_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_ListAll_Call) Return(_a0 []database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_ListAll_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_ListAll_Call) RunAndReturn(run func(context.Context) ([]database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_ListAll_Call {
	_c.Call.Return(run)
	return _c
}

// ListByArchsNameAndType provides a mock function with given fields: ctx, name, format, archs, deployType
func (_m *MockRuntimeFrameworksStore) ListByArchsNameAndType(ctx context.Context, name string, format string, archs []string, deployType int) ([]database.RuntimeFramework, error) {
	ret := _m.Called(ctx, name, format, archs, deployType)

	if len(ret) == 0 {
		panic("no return value specified for ListByArchsNameAndType")
	}

	var r0 []database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []string, int) ([]database.RuntimeFramework, error)); ok {
		return rf(ctx, name, format, archs, deployType)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []string, int) []database.RuntimeFramework); ok {
		r0 = rf(ctx, name, format, archs, deployType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, []string, int) error); ok {
		r1 = rf(ctx, name, format, archs, deployType)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_ListByArchsNameAndType_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListByArchsNameAndType'
type MockRuntimeFrameworksStore_ListByArchsNameAndType_Call struct {
	*mock.Call
}

// ListByArchsNameAndType is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
//   - format string
//   - archs []string
//   - deployType int
func (_e *MockRuntimeFrameworksStore_Expecter) ListByArchsNameAndType(ctx interface{}, name interface{}, format interface{}, archs interface{}, deployType interface{}) *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call {
	return &MockRuntimeFrameworksStore_ListByArchsNameAndType_Call{Call: _e.mock.On("ListByArchsNameAndType", ctx, name, format, archs, deployType)}
}

func (_c *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call) Run(run func(ctx context.Context, name string, format string, archs []string, deployType int)) *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].([]string), args[4].(int))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call) Return(_a0 []database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call) RunAndReturn(run func(context.Context, string, string, []string, int) ([]database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_ListByArchsNameAndType_Call {
	_c.Call.Return(run)
	return _c
}

// ListByIDs provides a mock function with given fields: ctx, ids
func (_m *MockRuntimeFrameworksStore) ListByIDs(ctx context.Context, ids []int64) ([]database.RuntimeFramework, error) {
	ret := _m.Called(ctx, ids)

	if len(ret) == 0 {
		panic("no return value specified for ListByIDs")
	}

	var r0 []database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []int64) ([]database.RuntimeFramework, error)); ok {
		return rf(ctx, ids)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []int64) []database.RuntimeFramework); ok {
		r0 = rf(ctx, ids)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []int64) error); ok {
		r1 = rf(ctx, ids)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_ListByIDs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListByIDs'
type MockRuntimeFrameworksStore_ListByIDs_Call struct {
	*mock.Call
}

// ListByIDs is a helper method to define mock.On call
//   - ctx context.Context
//   - ids []int64
func (_e *MockRuntimeFrameworksStore_Expecter) ListByIDs(ctx interface{}, ids interface{}) *MockRuntimeFrameworksStore_ListByIDs_Call {
	return &MockRuntimeFrameworksStore_ListByIDs_Call{Call: _e.mock.On("ListByIDs", ctx, ids)}
}

func (_c *MockRuntimeFrameworksStore_ListByIDs_Call) Run(run func(ctx context.Context, ids []int64)) *MockRuntimeFrameworksStore_ListByIDs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]int64))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_ListByIDs_Call) Return(_a0 []database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_ListByIDs_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_ListByIDs_Call) RunAndReturn(run func(context.Context, []int64) ([]database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_ListByIDs_Call {
	_c.Call.Return(run)
	return _c
}

// RemoveRuntimeFrameworkAndArch provides a mock function with given fields: ctx, id
func (_m *MockRuntimeFrameworksStore) RemoveRuntimeFrameworkAndArch(ctx context.Context, id int64) error {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for RemoveRuntimeFrameworkAndArch")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RemoveRuntimeFrameworkAndArch'
type MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call struct {
	*mock.Call
}

// RemoveRuntimeFrameworkAndArch is a helper method to define mock.On call
//   - ctx context.Context
//   - id int64
func (_e *MockRuntimeFrameworksStore_Expecter) RemoveRuntimeFrameworkAndArch(ctx interface{}, id interface{}) *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call {
	return &MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call{Call: _e.mock.On("RemoveRuntimeFrameworkAndArch", ctx, id)}
}

func (_c *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call) Run(run func(ctx context.Context, id int64)) *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int64))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call) Return(_a0 error) *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call) RunAndReturn(run func(context.Context, int64) error) *MockRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, frame
func (_m *MockRuntimeFrameworksStore) Update(ctx context.Context, frame database.RuntimeFramework) (*database.RuntimeFramework, error) {
	ret := _m.Called(ctx, frame)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *database.RuntimeFramework
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, database.RuntimeFramework) (*database.RuntimeFramework, error)); ok {
		return rf(ctx, frame)
	}
	if rf, ok := ret.Get(0).(func(context.Context, database.RuntimeFramework) *database.RuntimeFramework); ok {
		r0 = rf(ctx, frame)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*database.RuntimeFramework)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, database.RuntimeFramework) error); ok {
		r1 = rf(ctx, frame)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockRuntimeFrameworksStore_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type MockRuntimeFrameworksStore_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - frame database.RuntimeFramework
func (_e *MockRuntimeFrameworksStore_Expecter) Update(ctx interface{}, frame interface{}) *MockRuntimeFrameworksStore_Update_Call {
	return &MockRuntimeFrameworksStore_Update_Call{Call: _e.mock.On("Update", ctx, frame)}
}

func (_c *MockRuntimeFrameworksStore_Update_Call) Run(run func(ctx context.Context, frame database.RuntimeFramework)) *MockRuntimeFrameworksStore_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(database.RuntimeFramework))
	})
	return _c
}

func (_c *MockRuntimeFrameworksStore_Update_Call) Return(_a0 *database.RuntimeFramework, _a1 error) *MockRuntimeFrameworksStore_Update_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockRuntimeFrameworksStore_Update_Call) RunAndReturn(run func(context.Context, database.RuntimeFramework) (*database.RuntimeFramework, error)) *MockRuntimeFrameworksStore_Update_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockRuntimeFrameworksStore creates a new instance of MockRuntimeFrameworksStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRuntimeFrameworksStore(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRuntimeFrameworksStore {
	mock := &MockRuntimeFrameworksStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
