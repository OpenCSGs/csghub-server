package mq

const (
	NotifySubChangeSubject  string = "accounting.notify.subscription"
	NotifyStopDeploySubject string = "accounting.notify.stopdeploy"

	FeeRequestSubject       string = "accounting.fee.>"
	FeeSendSubject          string = "accounting.fee.credit"
	TokenSendSubject        string = "accounting.fee.token"
	QuotaSendSubject        string = "accounting.fee.quota"
	SubscriptionSendSubject string = "accounting.fee.subscription"

	MeterRequestSubject      string = "accounting.metering.>"
	MeterDurationSendSubject string = "accounting.metering.duration"
	MeterTokenSendSubject    string = "accounting.metering.token"
	MeterQuotaSendSubject    string = "accounting.metering.quota"

	OrderExpiredSubject    string = "accounting.order.expired"
	RechargeSucceedSubject string = "accounting.recharge.succeed"

	DLQFeeSubject      string = "accounting.dlq.fee"
	DLQMeterSubject    string = "accounting.dlq.meter"
	DLQRechargeSubject string = "accounting.dlq.recharge"

	HighPriorityMsgSubject   string = "notification.message.high"
	NormalPriorityMsgSubject string = "notification.message.normal"
)

type MQGroup struct {
	StreamName   string
	ConsumerName string
}

var (
	MeteringEventGroup = MQGroup{
		StreamName:   "meteringEventStream",            // metering event stream name
		ConsumerName: "metertingServerDurableConsumer", // metering event consumer name
	}
	AccountingEventGroup = MQGroup{
		StreamName:   "accountingEventStream",           // fee request stream name
		ConsumerName: "accountingServerDurableConsumer", // fee request consumer name
	}
	AccountingNotifyGroup = MQGroup{
		StreamName:   "accountingNotifyStream",          // notify stream name
		ConsumerName: "accountingNotifyDurableConsumer", // notify consumer name
	}
	AccountingDlqGroup = MQGroup{
		StreamName:   "accountingDlqStream",          // dlq
		ConsumerName: "accountingDlqDurableConsumer", // dlq consumer name
	}
	AccountingOrderGroup = MQGroup{
		StreamName:   "accountingOrderStream",          // order stream name
		ConsumerName: "accountingOrderDurableConsumer", // order consumer name
	}
	RechargeGroup = MQGroup{
		StreamName:   "rechargeStream",          // recharge stream name
		ConsumerName: "rechargeDurableConsumer", // recharge consumer name
	}
	DeployServiceUpdateGroup = MQGroup{
		StreamName:   "deployServiceUpdateStream", // deploy service update stream name
		ConsumerName: "deployServiceUpdateConsumer",
	}
	SiteInternalMailGroup = MQGroup{
		StreamName:   "siteInternalMailStream",
		ConsumerName: "siteInternalMailConsumer", // site internal mail consumer name
	}
	HighPriorityMsgGroup = MQGroup{
		StreamName:   "highPriorityMsgStream",
		ConsumerName: "highPriorityMsgConsumer",
	}
	NormalPriorityMsgGroup = MQGroup{
		StreamName:   "normalPriorityMsgStream",
		ConsumerName: "normalPriorityMsgConsumer", // normal priority message consumer name
	}
	WebhookEventGroup = MQGroup{
		StreamName:   "webhookEventStream", // webhook event stream name
		ConsumerName: "webhookEventConsumer",
	}
	AgentSessionHistoryMsgGroup = MQGroup{
		StreamName:   "agentSessionHistoryMsgStream",
		ConsumerName: "agentSessionHistoryMsgConsumer",
	}
)

type MessageMeta struct {
	Topic string
}

type MessageCallback func(raw []byte, meta MessageMeta) error

type SubscribeParams struct {
	Group                  MQGroup
	Topics                 []string
	AutoACK                bool // auto acknowledge message after callback success
	IsRedeliverForCBFailed bool // whether or not redeliver message for callback return error
	Callback               MessageCallback
}
