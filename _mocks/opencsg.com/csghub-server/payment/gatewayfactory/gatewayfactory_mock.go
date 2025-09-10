package gatewayfactory

import (
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/common/utils/payment/consts"
	"opencsg.com/csghub-server/payment/gateway"
)

type MockPaymentGatewayFactory struct {
	mock.Mock
}

func (m *MockPaymentGatewayFactory) GetPaymentGateway(paymentChannel consts.PaymentChannel) (gateway.PaymentGateway, error) {
	args := m.Called(paymentChannel)
	return args.Get(0).(gateway.PaymentGateway), args.Error(1)
}
