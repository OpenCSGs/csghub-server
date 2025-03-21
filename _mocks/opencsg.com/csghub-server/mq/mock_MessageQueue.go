// Code generated by mockery v2.53.0. DO NOT EDIT.

package mq

import (
	context "context"

	jetstream "github.com/nats-io/nats.go/jetstream"
	mock "github.com/stretchr/testify/mock"

	mq "opencsg.com/csghub-server/mq"

	nats "github.com/nats-io/nats.go"
)

// MockMessageQueue is an autogenerated mock type for the MessageQueue type
type MockMessageQueue struct {
	mock.Mock
}

type MockMessageQueue_Expecter struct {
	mock *mock.Mock
}

func (_m *MockMessageQueue) EXPECT() *MockMessageQueue_Expecter {
	return &MockMessageQueue_Expecter{mock: &_m.Mock}
}

// BuildDLQStream provides a mock function with no fields
func (_m *MockMessageQueue) BuildDLQStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for BuildDLQStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_BuildDLQStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BuildDLQStream'
type MockMessageQueue_BuildDLQStream_Call struct {
	*mock.Call
}

// BuildDLQStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) BuildDLQStream() *MockMessageQueue_BuildDLQStream_Call {
	return &MockMessageQueue_BuildDLQStream_Call{Call: _e.mock.On("BuildDLQStream")}
}

func (_c *MockMessageQueue_BuildDLQStream_Call) Run(run func()) *MockMessageQueue_BuildDLQStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_BuildDLQStream_Call) Return(_a0 error) *MockMessageQueue_BuildDLQStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_BuildDLQStream_Call) RunAndReturn(run func() error) *MockMessageQueue_BuildDLQStream_Call {
	_c.Call.Return(run)
	return _c
}

// BuildDeployServiceConsumerWithName provides a mock function with given fields: consumerName
func (_m *MockMessageQueue) BuildDeployServiceConsumerWithName(consumerName string) (jetstream.Consumer, error) {
	ret := _m.Called(consumerName)

	if len(ret) == 0 {
		panic("no return value specified for BuildDeployServiceConsumerWithName")
	}

	var r0 jetstream.Consumer
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (jetstream.Consumer, error)); ok {
		return rf(consumerName)
	}
	if rf, ok := ret.Get(0).(func(string) jetstream.Consumer); ok {
		r0 = rf(consumerName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(jetstream.Consumer)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(consumerName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockMessageQueue_BuildDeployServiceConsumerWithName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BuildDeployServiceConsumerWithName'
type MockMessageQueue_BuildDeployServiceConsumerWithName_Call struct {
	*mock.Call
}

// BuildDeployServiceConsumerWithName is a helper method to define mock.On call
//   - consumerName string
func (_e *MockMessageQueue_Expecter) BuildDeployServiceConsumerWithName(consumerName interface{}) *MockMessageQueue_BuildDeployServiceConsumerWithName_Call {
	return &MockMessageQueue_BuildDeployServiceConsumerWithName_Call{Call: _e.mock.On("BuildDeployServiceConsumerWithName", consumerName)}
}

func (_c *MockMessageQueue_BuildDeployServiceConsumerWithName_Call) Run(run func(consumerName string)) *MockMessageQueue_BuildDeployServiceConsumerWithName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockMessageQueue_BuildDeployServiceConsumerWithName_Call) Return(_a0 jetstream.Consumer, _a1 error) *MockMessageQueue_BuildDeployServiceConsumerWithName_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockMessageQueue_BuildDeployServiceConsumerWithName_Call) RunAndReturn(run func(string) (jetstream.Consumer, error)) *MockMessageQueue_BuildDeployServiceConsumerWithName_Call {
	_c.Call.Return(run)
	return _c
}

// BuildDeployServiceStream provides a mock function with no fields
func (_m *MockMessageQueue) BuildDeployServiceStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for BuildDeployServiceStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_BuildDeployServiceStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BuildDeployServiceStream'
type MockMessageQueue_BuildDeployServiceStream_Call struct {
	*mock.Call
}

// BuildDeployServiceStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) BuildDeployServiceStream() *MockMessageQueue_BuildDeployServiceStream_Call {
	return &MockMessageQueue_BuildDeployServiceStream_Call{Call: _e.mock.On("BuildDeployServiceStream")}
}

func (_c *MockMessageQueue_BuildDeployServiceStream_Call) Run(run func()) *MockMessageQueue_BuildDeployServiceStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_BuildDeployServiceStream_Call) Return(_a0 error) *MockMessageQueue_BuildDeployServiceStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_BuildDeployServiceStream_Call) RunAndReturn(run func() error) *MockMessageQueue_BuildDeployServiceStream_Call {
	_c.Call.Return(run)
	return _c
}

// BuildEventStreamAndConsumer provides a mock function with given fields: cfg, streamCfg, consumerCfg
func (_m *MockMessageQueue) BuildEventStreamAndConsumer(cfg mq.EventConfig, streamCfg jetstream.StreamConfig, consumerCfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	ret := _m.Called(cfg, streamCfg, consumerCfg)

	if len(ret) == 0 {
		panic("no return value specified for BuildEventStreamAndConsumer")
	}

	var r0 jetstream.Consumer
	var r1 error
	if rf, ok := ret.Get(0).(func(mq.EventConfig, jetstream.StreamConfig, jetstream.ConsumerConfig) (jetstream.Consumer, error)); ok {
		return rf(cfg, streamCfg, consumerCfg)
	}
	if rf, ok := ret.Get(0).(func(mq.EventConfig, jetstream.StreamConfig, jetstream.ConsumerConfig) jetstream.Consumer); ok {
		r0 = rf(cfg, streamCfg, consumerCfg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(jetstream.Consumer)
		}
	}

	if rf, ok := ret.Get(1).(func(mq.EventConfig, jetstream.StreamConfig, jetstream.ConsumerConfig) error); ok {
		r1 = rf(cfg, streamCfg, consumerCfg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockMessageQueue_BuildEventStreamAndConsumer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BuildEventStreamAndConsumer'
type MockMessageQueue_BuildEventStreamAndConsumer_Call struct {
	*mock.Call
}

// BuildEventStreamAndConsumer is a helper method to define mock.On call
//   - cfg mq.EventConfig
//   - streamCfg jetstream.StreamConfig
//   - consumerCfg jetstream.ConsumerConfig
func (_e *MockMessageQueue_Expecter) BuildEventStreamAndConsumer(cfg interface{}, streamCfg interface{}, consumerCfg interface{}) *MockMessageQueue_BuildEventStreamAndConsumer_Call {
	return &MockMessageQueue_BuildEventStreamAndConsumer_Call{Call: _e.mock.On("BuildEventStreamAndConsumer", cfg, streamCfg, consumerCfg)}
}

func (_c *MockMessageQueue_BuildEventStreamAndConsumer_Call) Run(run func(cfg mq.EventConfig, streamCfg jetstream.StreamConfig, consumerCfg jetstream.ConsumerConfig)) *MockMessageQueue_BuildEventStreamAndConsumer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(mq.EventConfig), args[1].(jetstream.StreamConfig), args[2].(jetstream.ConsumerConfig))
	})
	return _c
}

func (_c *MockMessageQueue_BuildEventStreamAndConsumer_Call) Return(_a0 jetstream.Consumer, _a1 error) *MockMessageQueue_BuildEventStreamAndConsumer_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockMessageQueue_BuildEventStreamAndConsumer_Call) RunAndReturn(run func(mq.EventConfig, jetstream.StreamConfig, jetstream.ConsumerConfig) (jetstream.Consumer, error)) *MockMessageQueue_BuildEventStreamAndConsumer_Call {
	_c.Call.Return(run)
	return _c
}

// BuildMeterEventStream provides a mock function with no fields
func (_m *MockMessageQueue) BuildMeterEventStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for BuildMeterEventStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_BuildMeterEventStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BuildMeterEventStream'
type MockMessageQueue_BuildMeterEventStream_Call struct {
	*mock.Call
}

// BuildMeterEventStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) BuildMeterEventStream() *MockMessageQueue_BuildMeterEventStream_Call {
	return &MockMessageQueue_BuildMeterEventStream_Call{Call: _e.mock.On("BuildMeterEventStream")}
}

func (_c *MockMessageQueue_BuildMeterEventStream_Call) Run(run func()) *MockMessageQueue_BuildMeterEventStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_BuildMeterEventStream_Call) Return(_a0 error) *MockMessageQueue_BuildMeterEventStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_BuildMeterEventStream_Call) RunAndReturn(run func() error) *MockMessageQueue_BuildMeterEventStream_Call {
	_c.Call.Return(run)
	return _c
}

// CreateOrUpdateStream provides a mock function with given fields: ctx, streamName, streamCfg
func (_m *MockMessageQueue) CreateOrUpdateStream(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig) (jetstream.Stream, error) {
	ret := _m.Called(ctx, streamName, streamCfg)

	if len(ret) == 0 {
		panic("no return value specified for CreateOrUpdateStream")
	}

	var r0 jetstream.Stream
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, jetstream.StreamConfig) (jetstream.Stream, error)); ok {
		return rf(ctx, streamName, streamCfg)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, jetstream.StreamConfig) jetstream.Stream); ok {
		r0 = rf(ctx, streamName, streamCfg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(jetstream.Stream)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, jetstream.StreamConfig) error); ok {
		r1 = rf(ctx, streamName, streamCfg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockMessageQueue_CreateOrUpdateStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateOrUpdateStream'
type MockMessageQueue_CreateOrUpdateStream_Call struct {
	*mock.Call
}

// CreateOrUpdateStream is a helper method to define mock.On call
//   - ctx context.Context
//   - streamName string
//   - streamCfg jetstream.StreamConfig
func (_e *MockMessageQueue_Expecter) CreateOrUpdateStream(ctx interface{}, streamName interface{}, streamCfg interface{}) *MockMessageQueue_CreateOrUpdateStream_Call {
	return &MockMessageQueue_CreateOrUpdateStream_Call{Call: _e.mock.On("CreateOrUpdateStream", ctx, streamName, streamCfg)}
}

func (_c *MockMessageQueue_CreateOrUpdateStream_Call) Run(run func(ctx context.Context, streamName string, streamCfg jetstream.StreamConfig)) *MockMessageQueue_CreateOrUpdateStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(jetstream.StreamConfig))
	})
	return _c
}

func (_c *MockMessageQueue_CreateOrUpdateStream_Call) Return(_a0 jetstream.Stream, _a1 error) *MockMessageQueue_CreateOrUpdateStream_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockMessageQueue_CreateOrUpdateStream_Call) RunAndReturn(run func(context.Context, string, jetstream.StreamConfig) (jetstream.Stream, error)) *MockMessageQueue_CreateOrUpdateStream_Call {
	_c.Call.Return(run)
	return _c
}

// FetchMeterEventMessages provides a mock function with given fields: batch
func (_m *MockMessageQueue) FetchMeterEventMessages(batch int) (jetstream.MessageBatch, error) {
	ret := _m.Called(batch)

	if len(ret) == 0 {
		panic("no return value specified for FetchMeterEventMessages")
	}

	var r0 jetstream.MessageBatch
	var r1 error
	if rf, ok := ret.Get(0).(func(int) (jetstream.MessageBatch, error)); ok {
		return rf(batch)
	}
	if rf, ok := ret.Get(0).(func(int) jetstream.MessageBatch); ok {
		r0 = rf(batch)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(jetstream.MessageBatch)
		}
	}

	if rf, ok := ret.Get(1).(func(int) error); ok {
		r1 = rf(batch)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockMessageQueue_FetchMeterEventMessages_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FetchMeterEventMessages'
type MockMessageQueue_FetchMeterEventMessages_Call struct {
	*mock.Call
}

// FetchMeterEventMessages is a helper method to define mock.On call
//   - batch int
func (_e *MockMessageQueue_Expecter) FetchMeterEventMessages(batch interface{}) *MockMessageQueue_FetchMeterEventMessages_Call {
	return &MockMessageQueue_FetchMeterEventMessages_Call{Call: _e.mock.On("FetchMeterEventMessages", batch)}
}

func (_c *MockMessageQueue_FetchMeterEventMessages_Call) Run(run func(batch int)) *MockMessageQueue_FetchMeterEventMessages_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(int))
	})
	return _c
}

func (_c *MockMessageQueue_FetchMeterEventMessages_Call) Return(_a0 jetstream.MessageBatch, _a1 error) *MockMessageQueue_FetchMeterEventMessages_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockMessageQueue_FetchMeterEventMessages_Call) RunAndReturn(run func(int) (jetstream.MessageBatch, error)) *MockMessageQueue_FetchMeterEventMessages_Call {
	_c.Call.Return(run)
	return _c
}

// GetConn provides a mock function with no fields
func (_m *MockMessageQueue) GetConn() *nats.Conn {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetConn")
	}

	var r0 *nats.Conn
	if rf, ok := ret.Get(0).(func() *nats.Conn); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*nats.Conn)
		}
	}

	return r0
}

// MockMessageQueue_GetConn_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetConn'
type MockMessageQueue_GetConn_Call struct {
	*mock.Call
}

// GetConn is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) GetConn() *MockMessageQueue_GetConn_Call {
	return &MockMessageQueue_GetConn_Call{Call: _e.mock.On("GetConn")}
}

func (_c *MockMessageQueue_GetConn_Call) Run(run func()) *MockMessageQueue_GetConn_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_GetConn_Call) Return(_a0 *nats.Conn) *MockMessageQueue_GetConn_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_GetConn_Call) RunAndReturn(run func() *nats.Conn) *MockMessageQueue_GetConn_Call {
	_c.Call.Return(run)
	return _c
}

// GetJetStream provides a mock function with no fields
func (_m *MockMessageQueue) GetJetStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetJetStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_GetJetStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetJetStream'
type MockMessageQueue_GetJetStream_Call struct {
	*mock.Call
}

// GetJetStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) GetJetStream() *MockMessageQueue_GetJetStream_Call {
	return &MockMessageQueue_GetJetStream_Call{Call: _e.mock.On("GetJetStream")}
}

func (_c *MockMessageQueue_GetJetStream_Call) Run(run func()) *MockMessageQueue_GetJetStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_GetJetStream_Call) Return(_a0 error) *MockMessageQueue_GetJetStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_GetJetStream_Call) RunAndReturn(run func() error) *MockMessageQueue_GetJetStream_Call {
	_c.Call.Return(run)
	return _c
}

// PublishData provides a mock function with given fields: subject, data
func (_m *MockMessageQueue) PublishData(subject string, data []byte) error {
	ret := _m.Called(subject, data)

	if len(ret) == 0 {
		panic("no return value specified for PublishData")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []byte) error); ok {
		r0 = rf(subject, data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishData'
type MockMessageQueue_PublishData_Call struct {
	*mock.Call
}

// PublishData is a helper method to define mock.On call
//   - subject string
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishData(subject interface{}, data interface{}) *MockMessageQueue_PublishData_Call {
	return &MockMessageQueue_PublishData_Call{Call: _e.mock.On("PublishData", subject, data)}
}

func (_c *MockMessageQueue_PublishData_Call) Run(run func(subject string, data []byte)) *MockMessageQueue_PublishData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishData_Call) Return(_a0 error) *MockMessageQueue_PublishData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishData_Call) RunAndReturn(run func(string, []byte) error) *MockMessageQueue_PublishData_Call {
	_c.Call.Return(run)
	return _c
}

// PublishDeployServiceData provides a mock function with given fields: data
func (_m *MockMessageQueue) PublishDeployServiceData(data []byte) error {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for PublishDeployServiceData")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte) error); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishDeployServiceData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishDeployServiceData'
type MockMessageQueue_PublishDeployServiceData_Call struct {
	*mock.Call
}

// PublishDeployServiceData is a helper method to define mock.On call
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishDeployServiceData(data interface{}) *MockMessageQueue_PublishDeployServiceData_Call {
	return &MockMessageQueue_PublishDeployServiceData_Call{Call: _e.mock.On("PublishDeployServiceData", data)}
}

func (_c *MockMessageQueue_PublishDeployServiceData_Call) Run(run func(data []byte)) *MockMessageQueue_PublishDeployServiceData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishDeployServiceData_Call) Return(_a0 error) *MockMessageQueue_PublishDeployServiceData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishDeployServiceData_Call) RunAndReturn(run func([]byte) error) *MockMessageQueue_PublishDeployServiceData_Call {
	_c.Call.Return(run)
	return _c
}

// PublishFeeCreditData provides a mock function with given fields: data
func (_m *MockMessageQueue) PublishFeeCreditData(data []byte) error {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for PublishFeeCreditData")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte) error); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishFeeCreditData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishFeeCreditData'
type MockMessageQueue_PublishFeeCreditData_Call struct {
	*mock.Call
}

// PublishFeeCreditData is a helper method to define mock.On call
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishFeeCreditData(data interface{}) *MockMessageQueue_PublishFeeCreditData_Call {
	return &MockMessageQueue_PublishFeeCreditData_Call{Call: _e.mock.On("PublishFeeCreditData", data)}
}

func (_c *MockMessageQueue_PublishFeeCreditData_Call) Run(run func(data []byte)) *MockMessageQueue_PublishFeeCreditData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishFeeCreditData_Call) Return(_a0 error) *MockMessageQueue_PublishFeeCreditData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishFeeCreditData_Call) RunAndReturn(run func([]byte) error) *MockMessageQueue_PublishFeeCreditData_Call {
	_c.Call.Return(run)
	return _c
}

// PublishFeeQuotaData provides a mock function with given fields: data
func (_m *MockMessageQueue) PublishFeeQuotaData(data []byte) error {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for PublishFeeQuotaData")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte) error); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishFeeQuotaData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishFeeQuotaData'
type MockMessageQueue_PublishFeeQuotaData_Call struct {
	*mock.Call
}

// PublishFeeQuotaData is a helper method to define mock.On call
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishFeeQuotaData(data interface{}) *MockMessageQueue_PublishFeeQuotaData_Call {
	return &MockMessageQueue_PublishFeeQuotaData_Call{Call: _e.mock.On("PublishFeeQuotaData", data)}
}

func (_c *MockMessageQueue_PublishFeeQuotaData_Call) Run(run func(data []byte)) *MockMessageQueue_PublishFeeQuotaData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishFeeQuotaData_Call) Return(_a0 error) *MockMessageQueue_PublishFeeQuotaData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishFeeQuotaData_Call) RunAndReturn(run func([]byte) error) *MockMessageQueue_PublishFeeQuotaData_Call {
	_c.Call.Return(run)
	return _c
}

// PublishFeeTokenData provides a mock function with given fields: data
func (_m *MockMessageQueue) PublishFeeTokenData(data []byte) error {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for PublishFeeTokenData")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte) error); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishFeeTokenData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishFeeTokenData'
type MockMessageQueue_PublishFeeTokenData_Call struct {
	*mock.Call
}

// PublishFeeTokenData is a helper method to define mock.On call
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishFeeTokenData(data interface{}) *MockMessageQueue_PublishFeeTokenData_Call {
	return &MockMessageQueue_PublishFeeTokenData_Call{Call: _e.mock.On("PublishFeeTokenData", data)}
}

func (_c *MockMessageQueue_PublishFeeTokenData_Call) Run(run func(data []byte)) *MockMessageQueue_PublishFeeTokenData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishFeeTokenData_Call) Return(_a0 error) *MockMessageQueue_PublishFeeTokenData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishFeeTokenData_Call) RunAndReturn(run func([]byte) error) *MockMessageQueue_PublishFeeTokenData_Call {
	_c.Call.Return(run)
	return _c
}

// PublishMeterDataToDLQ provides a mock function with given fields: data
func (_m *MockMessageQueue) PublishMeterDataToDLQ(data []byte) error {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for PublishMeterDataToDLQ")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte) error); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishMeterDataToDLQ_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishMeterDataToDLQ'
type MockMessageQueue_PublishMeterDataToDLQ_Call struct {
	*mock.Call
}

// PublishMeterDataToDLQ is a helper method to define mock.On call
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishMeterDataToDLQ(data interface{}) *MockMessageQueue_PublishMeterDataToDLQ_Call {
	return &MockMessageQueue_PublishMeterDataToDLQ_Call{Call: _e.mock.On("PublishMeterDataToDLQ", data)}
}

func (_c *MockMessageQueue_PublishMeterDataToDLQ_Call) Run(run func(data []byte)) *MockMessageQueue_PublishMeterDataToDLQ_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishMeterDataToDLQ_Call) Return(_a0 error) *MockMessageQueue_PublishMeterDataToDLQ_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishMeterDataToDLQ_Call) RunAndReturn(run func([]byte) error) *MockMessageQueue_PublishMeterDataToDLQ_Call {
	_c.Call.Return(run)
	return _c
}

// PublishMeterDurationData provides a mock function with given fields: data
func (_m *MockMessageQueue) PublishMeterDurationData(data []byte) error {
	ret := _m.Called(data)

	if len(ret) == 0 {
		panic("no return value specified for PublishMeterDurationData")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte) error); ok {
		r0 = rf(data)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_PublishMeterDurationData_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PublishMeterDurationData'
type MockMessageQueue_PublishMeterDurationData_Call struct {
	*mock.Call
}

// PublishMeterDurationData is a helper method to define mock.On call
//   - data []byte
func (_e *MockMessageQueue_Expecter) PublishMeterDurationData(data interface{}) *MockMessageQueue_PublishMeterDurationData_Call {
	return &MockMessageQueue_PublishMeterDurationData_Call{Call: _e.mock.On("PublishMeterDurationData", data)}
}

func (_c *MockMessageQueue_PublishMeterDurationData_Call) Run(run func(data []byte)) *MockMessageQueue_PublishMeterDurationData_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockMessageQueue_PublishMeterDurationData_Call) Return(_a0 error) *MockMessageQueue_PublishMeterDurationData_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_PublishMeterDurationData_Call) RunAndReturn(run func([]byte) error) *MockMessageQueue_PublishMeterDurationData_Call {
	_c.Call.Return(run)
	return _c
}

// VerifyDLQStream provides a mock function with no fields
func (_m *MockMessageQueue) VerifyDLQStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for VerifyDLQStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_VerifyDLQStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'VerifyDLQStream'
type MockMessageQueue_VerifyDLQStream_Call struct {
	*mock.Call
}

// VerifyDLQStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) VerifyDLQStream() *MockMessageQueue_VerifyDLQStream_Call {
	return &MockMessageQueue_VerifyDLQStream_Call{Call: _e.mock.On("VerifyDLQStream")}
}

func (_c *MockMessageQueue_VerifyDLQStream_Call) Run(run func()) *MockMessageQueue_VerifyDLQStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_VerifyDLQStream_Call) Return(_a0 error) *MockMessageQueue_VerifyDLQStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_VerifyDLQStream_Call) RunAndReturn(run func() error) *MockMessageQueue_VerifyDLQStream_Call {
	_c.Call.Return(run)
	return _c
}

// VerifyDeployServiceStream provides a mock function with no fields
func (_m *MockMessageQueue) VerifyDeployServiceStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for VerifyDeployServiceStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_VerifyDeployServiceStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'VerifyDeployServiceStream'
type MockMessageQueue_VerifyDeployServiceStream_Call struct {
	*mock.Call
}

// VerifyDeployServiceStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) VerifyDeployServiceStream() *MockMessageQueue_VerifyDeployServiceStream_Call {
	return &MockMessageQueue_VerifyDeployServiceStream_Call{Call: _e.mock.On("VerifyDeployServiceStream")}
}

func (_c *MockMessageQueue_VerifyDeployServiceStream_Call) Run(run func()) *MockMessageQueue_VerifyDeployServiceStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_VerifyDeployServiceStream_Call) Return(_a0 error) *MockMessageQueue_VerifyDeployServiceStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_VerifyDeployServiceStream_Call) RunAndReturn(run func() error) *MockMessageQueue_VerifyDeployServiceStream_Call {
	_c.Call.Return(run)
	return _c
}

// VerifyMeteringStream provides a mock function with no fields
func (_m *MockMessageQueue) VerifyMeteringStream() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for VerifyMeteringStream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_VerifyMeteringStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'VerifyMeteringStream'
type MockMessageQueue_VerifyMeteringStream_Call struct {
	*mock.Call
}

// VerifyMeteringStream is a helper method to define mock.On call
func (_e *MockMessageQueue_Expecter) VerifyMeteringStream() *MockMessageQueue_VerifyMeteringStream_Call {
	return &MockMessageQueue_VerifyMeteringStream_Call{Call: _e.mock.On("VerifyMeteringStream")}
}

func (_c *MockMessageQueue_VerifyMeteringStream_Call) Run(run func()) *MockMessageQueue_VerifyMeteringStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMessageQueue_VerifyMeteringStream_Call) Return(_a0 error) *MockMessageQueue_VerifyMeteringStream_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_VerifyMeteringStream_Call) RunAndReturn(run func() error) *MockMessageQueue_VerifyMeteringStream_Call {
	_c.Call.Return(run)
	return _c
}

// VerifyStreamByName provides a mock function with given fields: streamName
func (_m *MockMessageQueue) VerifyStreamByName(streamName string) error {
	ret := _m.Called(streamName)

	if len(ret) == 0 {
		panic("no return value specified for VerifyStreamByName")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(streamName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockMessageQueue_VerifyStreamByName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'VerifyStreamByName'
type MockMessageQueue_VerifyStreamByName_Call struct {
	*mock.Call
}

// VerifyStreamByName is a helper method to define mock.On call
//   - streamName string
func (_e *MockMessageQueue_Expecter) VerifyStreamByName(streamName interface{}) *MockMessageQueue_VerifyStreamByName_Call {
	return &MockMessageQueue_VerifyStreamByName_Call{Call: _e.mock.On("VerifyStreamByName", streamName)}
}

func (_c *MockMessageQueue_VerifyStreamByName_Call) Run(run func(streamName string)) *MockMessageQueue_VerifyStreamByName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockMessageQueue_VerifyStreamByName_Call) Return(_a0 error) *MockMessageQueue_VerifyStreamByName_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMessageQueue_VerifyStreamByName_Call) RunAndReturn(run func(string) error) *MockMessageQueue_VerifyStreamByName_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockMessageQueue creates a new instance of MockMessageQueue. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockMessageQueue(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMessageQueue {
	mock := &MockMessageQueue{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
