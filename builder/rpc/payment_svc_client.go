package rpc

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/money"
	"opencsg.com/csghub-server/common/utils/payment/consts"
)

type PaymentSvcClient interface {
	CreateSimplePayment(ctx context.Context,
		orderNo string,
		subject string,
		body string,
		amount *money.Money,
		extra string,
		channel consts.PaymentChannel) (*PaymentResponse, error)
}

type PaymentSvcHttpClient struct {
	hc *HttpClient
}

func NewPaymentSvcHttpClient(endpoint string, opts ...RequestOption) PaymentSvcClient {
	return &PaymentSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *PaymentSvcHttpClient) CreateSimplePayment(
	ctx context.Context,
	orderNo string,
	subject string,
	body string,
	amount *money.Money,
	extra string,
	channel consts.PaymentChannel) (*PaymentResponse, error) {
	yuanAmount, _ := amount.ToYuanFloat()

	req := &types.CreatePaymentReq{
		OrderNo: orderNo,
		Amount:  yuanAmount,
		Channel: channel,
		Subject: subject,
		Body:    body,
		Extra:   extra,
	}

	url := "/api/v1/payment/simple-payments"
	resp := &PaymentResponse{}
	err := c.hc.Post(ctx, url, req, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke simple payment: %w", err)
	}
	return resp, nil
}
