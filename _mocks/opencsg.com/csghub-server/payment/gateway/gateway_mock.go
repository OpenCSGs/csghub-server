package gateway

import (
	"context"
	"io"
	"time"

	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/common/utils/money"
	"opencsg.com/csghub-server/payment/gateway"
)

type MockPaymentGateway struct {
	mock.Mock
}

func (m *MockPaymentGateway) GenerateQRCode(ctx context.Context, outTradeNo string, amount *money.Money, subject string, body string, expireIn time.Duration) (gateway.QRCodePayment, error) {
	args := m.Called(ctx, outTradeNo, amount, subject, body, expireIn)
	return args.Get(0).(gateway.QRCodePayment), args.Error(1)
}

func (m *MockPaymentGateway) WaitForPayment(ctx context.Context, outTradeNo string, after time.Duration, expireIn time.Duration) gateway.PaymentResult {
	args := m.Called(ctx, outTradeNo, after, expireIn)
	return args.Get(0).(gateway.PaymentResult)
}

func (m *MockPaymentGateway) ClosePayment(ctx context.Context, outTradeNo string) error {
	args := m.Called(ctx, outTradeNo)
	return args.Error(0)
}

func (m *MockPaymentGateway) DownloadBill(ctx context.Context, billType gateway.BillType, date time.Time) (*gateway.BillDownloadResult, error) {
	args := m.Called(ctx, billType, date)
	return args.Get(0).(*gateway.BillDownloadResult), args.Error(1)
}

func (m *MockPaymentGateway) ReadTradeBill(ctx context.Context, billDate time.Time, data io.Reader) (*gateway.BillSummary, []gateway.BillDetail, error) {
	args := m.Called(ctx, billDate, data)
	return args.Get(0).(*gateway.BillSummary), args.Get(1).([]gateway.BillDetail), args.Error(2)
}
