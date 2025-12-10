package database

/*
import (
	"context"
	"fmt"
	"github.com/go-pay/xlog"
	"opencsg.com/csghub-server/payment/consts"
	"testing"
	"time"
)

func TestPayment(t *testing.T) {
	// TODO init DB
	ctx := context.Background()
	paymentStore := NewPaymentStore()

	// 创建支付记录
	newPayment := &Payment{
		PaymentUUID:   "unique-payment-id",
		TransactionNo: "transaction-no",
		OrderNo:       "order-no",
		Channel:       consts.ChannelAlipayQr,
		Amount:        200.0,
		Currency:      "CNY",
		Subject:       "Product Title",
		Body:          "Product Description",
		ClientIp:      "127.0.0.1",
		TimeExpire:    time.Now().Add(30 * time.Minute),
	}

	err := paymentStore.CreatePayment(ctx, newPayment)
	if err != nil {
		xlog.Errorf("Failed to create payment: %v", err)
	}

	// 获取支付记录
	payment, err := paymentStore.GetPaymentByID(ctx, "unique-payment-id")
	if err != nil {
		xlog.Errorf("Failed to get payment: %v", err)
	}
	fmt.Printf("Payment: %+v\n", payment)

	// 更新支付记录
	payment.Paid = true
	payment.TimePaid = time.Now()
	err = paymentStore.UpdatePayment(ctx, payment)
	if err != nil {
		xlog.Errorf("Failed to update payment: %v", err)
	}

	// 列出支付记录
	filter := &PaymentFilter{
		Paid:  new(bool), // 默认为 false，表示未支付
		Limit: 10,
	}
	payments, err := paymentStore.ListPayments(ctx, filter)
	if err != nil {
		xlog.Errorf("Failed to list payments: %v", err)
	}
	for _, p := range payments {
		fmt.Printf("Payment: %+v\n", p)
	}
}
*/
