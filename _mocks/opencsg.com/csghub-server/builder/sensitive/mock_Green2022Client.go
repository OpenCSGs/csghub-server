// Code generated by mockery v2.53.0. DO NOT EDIT.

package sensitive

import (
	client "github.com/alibabacloud-go/green-20220302/client"
	mock "github.com/stretchr/testify/mock"
)

// MockGreen2022Client is an autogenerated mock type for the Green2022Client type
type MockGreen2022Client struct {
	mock.Mock
}

type MockGreen2022Client_Expecter struct {
	mock *mock.Mock
}

func (_m *MockGreen2022Client) EXPECT() *MockGreen2022Client_Expecter {
	return &MockGreen2022Client_Expecter{mock: &_m.Mock}
}

// GetRegionId provides a mock function with no fields
func (_m *MockGreen2022Client) GetRegionId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetRegionId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockGreen2022Client_GetRegionId_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRegionId'
type MockGreen2022Client_GetRegionId_Call struct {
	*mock.Call
}

// GetRegionId is a helper method to define mock.On call
func (_e *MockGreen2022Client_Expecter) GetRegionId() *MockGreen2022Client_GetRegionId_Call {
	return &MockGreen2022Client_GetRegionId_Call{Call: _e.mock.On("GetRegionId")}
}

func (_c *MockGreen2022Client_GetRegionId_Call) Run(run func()) *MockGreen2022Client_GetRegionId_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockGreen2022Client_GetRegionId_Call) Return(_a0 string) *MockGreen2022Client_GetRegionId_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockGreen2022Client_GetRegionId_Call) RunAndReturn(run func() string) *MockGreen2022Client_GetRegionId_Call {
	_c.Call.Return(run)
	return _c
}

// ImageModeration provides a mock function with given fields: request
func (_m *MockGreen2022Client) ImageModeration(request *client.ImageModerationRequest) (*client.ImageModerationResponse, error) {
	ret := _m.Called(request)

	if len(ret) == 0 {
		panic("no return value specified for ImageModeration")
	}

	var r0 *client.ImageModerationResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(*client.ImageModerationRequest) (*client.ImageModerationResponse, error)); ok {
		return rf(request)
	}
	if rf, ok := ret.Get(0).(func(*client.ImageModerationRequest) *client.ImageModerationResponse); ok {
		r0 = rf(request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*client.ImageModerationResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(*client.ImageModerationRequest) error); ok {
		r1 = rf(request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockGreen2022Client_ImageModeration_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ImageModeration'
type MockGreen2022Client_ImageModeration_Call struct {
	*mock.Call
}

// ImageModeration is a helper method to define mock.On call
//   - request *client.ImageModerationRequest
func (_e *MockGreen2022Client_Expecter) ImageModeration(request interface{}) *MockGreen2022Client_ImageModeration_Call {
	return &MockGreen2022Client_ImageModeration_Call{Call: _e.mock.On("ImageModeration", request)}
}

func (_c *MockGreen2022Client_ImageModeration_Call) Run(run func(request *client.ImageModerationRequest)) *MockGreen2022Client_ImageModeration_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*client.ImageModerationRequest))
	})
	return _c
}

func (_c *MockGreen2022Client_ImageModeration_Call) Return(_result *client.ImageModerationResponse, _err error) *MockGreen2022Client_ImageModeration_Call {
	_c.Call.Return(_result, _err)
	return _c
}

func (_c *MockGreen2022Client_ImageModeration_Call) RunAndReturn(run func(*client.ImageModerationRequest) (*client.ImageModerationResponse, error)) *MockGreen2022Client_ImageModeration_Call {
	_c.Call.Return(run)
	return _c
}

// TextModeration provides a mock function with given fields: request
func (_m *MockGreen2022Client) TextModeration(request *client.TextModerationRequest) (*client.TextModerationResponse, error) {
	ret := _m.Called(request)

	if len(ret) == 0 {
		panic("no return value specified for TextModeration")
	}

	var r0 *client.TextModerationResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(*client.TextModerationRequest) (*client.TextModerationResponse, error)); ok {
		return rf(request)
	}
	if rf, ok := ret.Get(0).(func(*client.TextModerationRequest) *client.TextModerationResponse); ok {
		r0 = rf(request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*client.TextModerationResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(*client.TextModerationRequest) error); ok {
		r1 = rf(request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockGreen2022Client_TextModeration_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'TextModeration'
type MockGreen2022Client_TextModeration_Call struct {
	*mock.Call
}

// TextModeration is a helper method to define mock.On call
//   - request *client.TextModerationRequest
func (_e *MockGreen2022Client_Expecter) TextModeration(request interface{}) *MockGreen2022Client_TextModeration_Call {
	return &MockGreen2022Client_TextModeration_Call{Call: _e.mock.On("TextModeration", request)}
}

func (_c *MockGreen2022Client_TextModeration_Call) Run(run func(request *client.TextModerationRequest)) *MockGreen2022Client_TextModeration_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*client.TextModerationRequest))
	})
	return _c
}

func (_c *MockGreen2022Client_TextModeration_Call) Return(_result *client.TextModerationResponse, _err error) *MockGreen2022Client_TextModeration_Call {
	_c.Call.Return(_result, _err)
	return _c
}

func (_c *MockGreen2022Client_TextModeration_Call) RunAndReturn(run func(*client.TextModerationRequest) (*client.TextModerationResponse, error)) *MockGreen2022Client_TextModeration_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockGreen2022Client creates a new instance of MockGreen2022Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockGreen2022Client(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockGreen2022Client {
	mock := &MockGreen2022Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
