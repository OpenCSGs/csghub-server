package consts

type PaymentChannel string

type TradeStatus string

const (
	ChannelAlipay           PaymentChannel = "alipay"
	ChannelAlipayWap        PaymentChannel = "alipay_wap"
	ChannelAlipayQr         PaymentChannel = "alipay_qr"
	ChannelAlipayScan       PaymentChannel = "alipay_scan"
	ChannelAlipayLite       PaymentChannel = "alipay_lite"
	ChannelAlipayPcDirect   PaymentChannel = "alipay_pc_direct"
	ChannelWx               PaymentChannel = "wx"
	ChannelWxPub            PaymentChannel = "wx_pub"
	ChannelWxPubQr          PaymentChannel = "wx_pub_qr"
	ChannelWxPubScan        PaymentChannel = "wx_pub_scan"
	ChannelWxWap            PaymentChannel = "wx_wap"
	ChannelWxLite           PaymentChannel = "wx_lite"
	ChannelQpay             PaymentChannel = "qpay"
	ChannelQpayPub          PaymentChannel = "qpay_pub"
	ChannelUpacp            PaymentChannel = "upacp"
	ChannelUpacpPc          PaymentChannel = "upacp_pc"
	ChannelUpacpQr          PaymentChannel = "upacp_qr"
	ChannelUpacpScan        PaymentChannel = "upacp_scan"
	ChannelUpacpWap         PaymentChannel = "upacp_wap"
	ChannelUpacpB2b         PaymentChannel = "upacp_b2b"
	ChannelCpB2b            PaymentChannel = "cp_b2b"
	ChannelApplepayUpacp    PaymentChannel = "applepay_upacp"
	ChannelCmbWallet        PaymentChannel = "cmb_wallet"
	ChannelCmbPcQr          PaymentChannel = "cmb_pc_qr"
	ChannelBfbWap           PaymentChannel = "bfb_wap"
	ChannelJdpayWap         PaymentChannel = "jdpay_wap"
	ChannelYeepayWap        PaymentChannel = "yeepay_wap"
	ChannelIsvQr            PaymentChannel = "isv_qr"
	ChannelIsvScan          PaymentChannel = "isv_scan"
	ChannelIsvWap           PaymentChannel = "isv_wap"
	ChannelIsvLite          PaymentChannel = "isv_lite"
	ChannelCcbPay           PaymentChannel = "ccb_pay"
	ChannelCcbQr            PaymentChannel = "ccb_qr"
	ChannelCmpay            PaymentChannel = "cmpay"
	ChannelCoolcredit       PaymentChannel = "coolcredit"
	ChannelCbAlipay         PaymentChannel = "cb_alipay"
	ChannelCbAlipayWap      PaymentChannel = "cb_alipay_wap"
	ChannelCbAlipayQr       PaymentChannel = "cb_alipay_qr"
	ChannelCbAlipayScan     PaymentChannel = "cb_alipay_scan"
	ChannelCbAlipayPcDirect PaymentChannel = "cb_alipay_pc_direct"
	ChannelCbWx             PaymentChannel = "cb_wx"
	ChannelCbWxPub          PaymentChannel = "cb_wx_pub"
	ChannelCbWxPubQr        PaymentChannel = "cb_wx_pub_qr"
	ChannelCbWxPubScan      PaymentChannel = "cb_wx_pub_scan"
	ChannelPaypal           PaymentChannel = "paypal"
	ChannelBalance          PaymentChannel = "balance"
	ChannelYeepayWxPubQr    PaymentChannel = "yeepay_wx_pub_qr"
	ChannelYeepayWxPub      PaymentChannel = "yeepay_wx_pub"
	ChannelYeepayWxPubOfl   PaymentChannel = "yeepay_wx_pub_ofl"
	ChannelYeepayWxLite     PaymentChannel = "yeepay_wx_lite"
	ChannelYeepayWxLiteOfl  PaymentChannel = "yeepay_wx_lite_ofl"
	ChannelYeepayWxPubScan  PaymentChannel = "yeepay_wx_pub_scan"
	ChannelYeepayAlipayQr   PaymentChannel = "yeepay_alipay_qr"
	ChannelYeepayAlipayLite PaymentChannel = "yeepay_alipay_lite"
	ChannelYeepayAlipayPub  PaymentChannel = "yeepay_alipay_pub"
	ChannelYeepayAlipayScan PaymentChannel = "yeepay_alipay_scan"
	ChannelYeepayUpacpQr    PaymentChannel = "yeepay_upacp_qr"
	ChannelYeepayUpacpScan  PaymentChannel = "yeepay_upacp_scan"

	TradeStatusSuccess string = "SUCCESS"

	// TradeStatusRefund Transaction order status: Refunded
	TradeStatusRefund string = "REFUND"

	// TradeStatusNotPay Transaction order status: Unpaid
	TradeStatusNotPay string = "NOTPAY"

	// TradeStatusClosed Transaction order status: Closed
	TradeStatusClosed string = "CLOSED"

	// TradeStatusRevoked Transaction order status: Revoked
	TradeStatusRevoked string = "REVOKED"

	// TradeStatusUserPaying Transaction order status: User Paying
	TradeStatusUserPaying string = "USERPAYING"

	// TradeStatusPayError Transaction order status: Payment Failed
	TradeStatusPayError string = "PAYERROR"

	// Alipay-specific trade statuses
	TradeStatusWaitBuyerPay  TradeStatus = "WAIT_BUYER_PAY"
	TradeStatusTradeClosed   TradeStatus = "TRADE_CLOSED"
	TradeStatusTradeSuccess  TradeStatus = "TRADE_SUCCESS"
	TradeStatusTradeFinished TradeStatus = "TRADE_FINISHED"
)

type PaymentGatewayType string

const (
	AliPayGateway PaymentGatewayType = "alipay"
	WXPayGateway  PaymentGatewayType = "wxpay"
)

type PaymentStatus int

const (
	PaymentSuccess PaymentStatus = iota
	PaymentPending
	PaymentClosed
	PaymentFailed
	PaymentTimeout
)
