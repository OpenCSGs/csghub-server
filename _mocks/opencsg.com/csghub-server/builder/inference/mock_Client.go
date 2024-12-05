// Code generated by mockery v2.49.1. DO NOT EDIT.

package inference

import (
	mock "github.com/stretchr/testify/mock"
	inference "opencsg.com/csghub-server/builder/inference"
)

// MockClient is an autogenerated mock type for the Client type
type MockClient struct {
	mock.Mock
}

type MockClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClient) EXPECT() *MockClient_Expecter {
	return &MockClient_Expecter{mock: &_m.Mock}
}

// GetModelInfo provides a mock function with given fields: id
func (_m *MockClient) GetModelInfo(id inference.ModelID) (inference.ModelInfo, error) {
	ret := _m.Called(id)

	if len(ret) == 0 {
		panic("no return value specified for GetModelInfo")
	}

	var r0 inference.ModelInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(inference.ModelID) (inference.ModelInfo, error)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(inference.ModelID) inference.ModelInfo); ok {
		r0 = rf(id)
	} else {
		r0 = ret.Get(0).(inference.ModelInfo)
	}

	if rf, ok := ret.Get(1).(func(inference.ModelID) error); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClient_GetModelInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetModelInfo'
type MockClient_GetModelInfo_Call struct {
	*mock.Call
}

// GetModelInfo is a helper method to define mock.On call
//   - id inference.ModelID
func (_e *MockClient_Expecter) GetModelInfo(id interface{}) *MockClient_GetModelInfo_Call {
	return &MockClient_GetModelInfo_Call{Call: _e.mock.On("GetModelInfo", id)}
}

func (_c *MockClient_GetModelInfo_Call) Run(run func(id inference.ModelID)) *MockClient_GetModelInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(inference.ModelID))
	})
	return _c
}

func (_c *MockClient_GetModelInfo_Call) Return(_a0 inference.ModelInfo, _a1 error) *MockClient_GetModelInfo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockClient_GetModelInfo_Call) RunAndReturn(run func(inference.ModelID) (inference.ModelInfo, error)) *MockClient_GetModelInfo_Call {
	_c.Call.Return(run)
	return _c
}

// Predict provides a mock function with given fields: id, req
func (_m *MockClient) Predict(id inference.ModelID, req *inference.PredictRequest) (*inference.PredictResponse, error) {
	ret := _m.Called(id, req)

	if len(ret) == 0 {
		panic("no return value specified for Predict")
	}

	var r0 *inference.PredictResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(inference.ModelID, *inference.PredictRequest) (*inference.PredictResponse, error)); ok {
		return rf(id, req)
	}
	if rf, ok := ret.Get(0).(func(inference.ModelID, *inference.PredictRequest) *inference.PredictResponse); ok {
		r0 = rf(id, req)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*inference.PredictResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(inference.ModelID, *inference.PredictRequest) error); ok {
		r1 = rf(id, req)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClient_Predict_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Predict'
type MockClient_Predict_Call struct {
	*mock.Call
}

// Predict is a helper method to define mock.On call
//   - id inference.ModelID
//   - req *inference.PredictRequest
func (_e *MockClient_Expecter) Predict(id interface{}, req interface{}) *MockClient_Predict_Call {
	return &MockClient_Predict_Call{Call: _e.mock.On("Predict", id, req)}
}

func (_c *MockClient_Predict_Call) Run(run func(id inference.ModelID, req *inference.PredictRequest)) *MockClient_Predict_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(inference.ModelID), args[1].(*inference.PredictRequest))
	})
	return _c
}

func (_c *MockClient_Predict_Call) Return(_a0 *inference.PredictResponse, _a1 error) *MockClient_Predict_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockClient_Predict_Call) RunAndReturn(run func(inference.ModelID, *inference.PredictRequest) (*inference.PredictResponse, error)) *MockClient_Predict_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockClient creates a new instance of MockClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClient {
	mock := &MockClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
