// Code generated by mockery v2.49.1. DO NOT EDIT.

package jetstream

import (
	context "context"

	jetstream "github.com/nats-io/nats.go/jetstream"
	mock "github.com/stretchr/testify/mock"

	nats "github.com/nats-io/nats.go"

	time "time"
)

// MockMsg is an autogenerated mock type for the Msg type
type MockMsg struct {
	mock.Mock
}

type MockMsg_Expecter struct {
	mock *mock.Mock
}

func (_m *MockMsg) EXPECT() *MockMsg_Expecter {
	return &MockMsg_Expecter{mock: &_m.Mock}
}

// Ack provides a mock function with given fields:
func (_m *MockMsg) Ack() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Ack")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_Ack_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Ack'
type MockMsg_Ack_Call struct {
	*mock.Call
}

// Ack is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Ack() *MockMsg_Ack_Call {
	return &MockMsg_Ack_Call{Call: _e.mock.On("Ack")}
}

func (_c *MockMsg_Ack_Call) Run(run func()) *MockMsg_Ack_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Ack_Call) Return(_a0 error) *MockMsg_Ack_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Ack_Call) RunAndReturn(run func() error) *MockMsg_Ack_Call {
	_c.Call.Return(run)
	return _c
}

// Data provides a mock function with given fields:
func (_m *MockMsg) Data() []byte {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Data")
	}

	var r0 []byte
	if rf, ok := ret.Get(0).(func() []byte); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	return r0
}

// MockMsg_Data_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Data'
type MockMsg_Data_Call struct {
	*mock.Call
}

// Data is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Data() *MockMsg_Data_Call {
	return &MockMsg_Data_Call{Call: _e.mock.On("Data")}
}

func (_c *MockMsg_Data_Call) Run(run func()) *MockMsg_Data_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Data_Call) Return(_a0 []byte) *MockMsg_Data_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Data_Call) RunAndReturn(run func() []byte) *MockMsg_Data_Call {
	_c.Call.Return(run)
	return _c
}

// DoubleAck provides a mock function with given fields: _a0
func (_m *MockMsg) DoubleAck(_a0 context.Context) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for DoubleAck")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_DoubleAck_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DoubleAck'
type MockMsg_DoubleAck_Call struct {
	*mock.Call
}

// DoubleAck is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockMsg_Expecter) DoubleAck(_a0 interface{}) *MockMsg_DoubleAck_Call {
	return &MockMsg_DoubleAck_Call{Call: _e.mock.On("DoubleAck", _a0)}
}

func (_c *MockMsg_DoubleAck_Call) Run(run func(_a0 context.Context)) *MockMsg_DoubleAck_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockMsg_DoubleAck_Call) Return(_a0 error) *MockMsg_DoubleAck_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_DoubleAck_Call) RunAndReturn(run func(context.Context) error) *MockMsg_DoubleAck_Call {
	_c.Call.Return(run)
	return _c
}

// Headers provides a mock function with given fields:
func (_m *MockMsg) Headers() nats.Header {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Headers")
	}

	var r0 nats.Header
	if rf, ok := ret.Get(0).(func() nats.Header); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(nats.Header)
		}
	}

	return r0
}

// MockMsg_Headers_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Headers'
type MockMsg_Headers_Call struct {
	*mock.Call
}

// Headers is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Headers() *MockMsg_Headers_Call {
	return &MockMsg_Headers_Call{Call: _e.mock.On("Headers")}
}

func (_c *MockMsg_Headers_Call) Run(run func()) *MockMsg_Headers_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Headers_Call) Return(_a0 nats.Header) *MockMsg_Headers_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Headers_Call) RunAndReturn(run func() nats.Header) *MockMsg_Headers_Call {
	_c.Call.Return(run)
	return _c
}

// InProgress provides a mock function with given fields:
func (_m *MockMsg) InProgress() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for InProgress")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_InProgress_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'InProgress'
type MockMsg_InProgress_Call struct {
	*mock.Call
}

// InProgress is a helper method to define mock.On call
func (_e *MockMsg_Expecter) InProgress() *MockMsg_InProgress_Call {
	return &MockMsg_InProgress_Call{Call: _e.mock.On("InProgress")}
}

func (_c *MockMsg_InProgress_Call) Run(run func()) *MockMsg_InProgress_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_InProgress_Call) Return(_a0 error) *MockMsg_InProgress_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_InProgress_Call) RunAndReturn(run func() error) *MockMsg_InProgress_Call {
	_c.Call.Return(run)
	return _c
}

// Metadata provides a mock function with given fields:
func (_m *MockMsg) Metadata() (*jetstream.MsgMetadata, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Metadata")
	}

	var r0 *jetstream.MsgMetadata
	var r1 error
	if rf, ok := ret.Get(0).(func() (*jetstream.MsgMetadata, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *jetstream.MsgMetadata); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*jetstream.MsgMetadata)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockMsg_Metadata_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Metadata'
type MockMsg_Metadata_Call struct {
	*mock.Call
}

// Metadata is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Metadata() *MockMsg_Metadata_Call {
	return &MockMsg_Metadata_Call{Call: _e.mock.On("Metadata")}
}

func (_c *MockMsg_Metadata_Call) Run(run func()) *MockMsg_Metadata_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Metadata_Call) Return(_a0 *jetstream.MsgMetadata, _a1 error) *MockMsg_Metadata_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockMsg_Metadata_Call) RunAndReturn(run func() (*jetstream.MsgMetadata, error)) *MockMsg_Metadata_Call {
	_c.Call.Return(run)
	return _c
}

// Nak provides a mock function with given fields:
func (_m *MockMsg) Nak() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Nak")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_Nak_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Nak'
type MockMsg_Nak_Call struct {
	*mock.Call
}

// Nak is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Nak() *MockMsg_Nak_Call {
	return &MockMsg_Nak_Call{Call: _e.mock.On("Nak")}
}

func (_c *MockMsg_Nak_Call) Run(run func()) *MockMsg_Nak_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Nak_Call) Return(_a0 error) *MockMsg_Nak_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Nak_Call) RunAndReturn(run func() error) *MockMsg_Nak_Call {
	_c.Call.Return(run)
	return _c
}

// NakWithDelay provides a mock function with given fields: delay
func (_m *MockMsg) NakWithDelay(delay time.Duration) error {
	ret := _m.Called(delay)

	if len(ret) == 0 {
		panic("no return value specified for NakWithDelay")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(time.Duration) error); ok {
		r0 = rf(delay)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_NakWithDelay_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NakWithDelay'
type MockMsg_NakWithDelay_Call struct {
	*mock.Call
}

// NakWithDelay is a helper method to define mock.On call
//   - delay time.Duration
func (_e *MockMsg_Expecter) NakWithDelay(delay interface{}) *MockMsg_NakWithDelay_Call {
	return &MockMsg_NakWithDelay_Call{Call: _e.mock.On("NakWithDelay", delay)}
}

func (_c *MockMsg_NakWithDelay_Call) Run(run func(delay time.Duration)) *MockMsg_NakWithDelay_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(time.Duration))
	})
	return _c
}

func (_c *MockMsg_NakWithDelay_Call) Return(_a0 error) *MockMsg_NakWithDelay_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_NakWithDelay_Call) RunAndReturn(run func(time.Duration) error) *MockMsg_NakWithDelay_Call {
	_c.Call.Return(run)
	return _c
}

// Reply provides a mock function with given fields:
func (_m *MockMsg) Reply() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Reply")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockMsg_Reply_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Reply'
type MockMsg_Reply_Call struct {
	*mock.Call
}

// Reply is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Reply() *MockMsg_Reply_Call {
	return &MockMsg_Reply_Call{Call: _e.mock.On("Reply")}
}

func (_c *MockMsg_Reply_Call) Run(run func()) *MockMsg_Reply_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Reply_Call) Return(_a0 string) *MockMsg_Reply_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Reply_Call) RunAndReturn(run func() string) *MockMsg_Reply_Call {
	_c.Call.Return(run)
	return _c
}

// Subject provides a mock function with given fields:
func (_m *MockMsg) Subject() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Subject")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockMsg_Subject_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Subject'
type MockMsg_Subject_Call struct {
	*mock.Call
}

// Subject is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Subject() *MockMsg_Subject_Call {
	return &MockMsg_Subject_Call{Call: _e.mock.On("Subject")}
}

func (_c *MockMsg_Subject_Call) Run(run func()) *MockMsg_Subject_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Subject_Call) Return(_a0 string) *MockMsg_Subject_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Subject_Call) RunAndReturn(run func() string) *MockMsg_Subject_Call {
	_c.Call.Return(run)
	return _c
}

// Term provides a mock function with given fields:
func (_m *MockMsg) Term() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Term")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_Term_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Term'
type MockMsg_Term_Call struct {
	*mock.Call
}

// Term is a helper method to define mock.On call
func (_e *MockMsg_Expecter) Term() *MockMsg_Term_Call {
	return &MockMsg_Term_Call{Call: _e.mock.On("Term")}
}

func (_c *MockMsg_Term_Call) Run(run func()) *MockMsg_Term_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMsg_Term_Call) Return(_a0 error) *MockMsg_Term_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_Term_Call) RunAndReturn(run func() error) *MockMsg_Term_Call {
	_c.Call.Return(run)
	return _c
}

// TermWithReason provides a mock function with given fields: reason
func (_m *MockMsg) TermWithReason(reason string) error {
	ret := _m.Called(reason)

	if len(ret) == 0 {
		panic("no return value specified for TermWithReason")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(reason)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMsg_TermWithReason_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'TermWithReason'
type MockMsg_TermWithReason_Call struct {
	*mock.Call
}

// TermWithReason is a helper method to define mock.On call
//   - reason string
func (_e *MockMsg_Expecter) TermWithReason(reason interface{}) *MockMsg_TermWithReason_Call {
	return &MockMsg_TermWithReason_Call{Call: _e.mock.On("TermWithReason", reason)}
}

func (_c *MockMsg_TermWithReason_Call) Run(run func(reason string)) *MockMsg_TermWithReason_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockMsg_TermWithReason_Call) Return(_a0 error) *MockMsg_TermWithReason_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMsg_TermWithReason_Call) RunAndReturn(run func(string) error) *MockMsg_TermWithReason_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockMsg creates a new instance of MockMsg. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockMsg(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMsg {
	mock := &MockMsg{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}