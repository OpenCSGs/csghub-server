package types

import (
	"errors"
	"time"

	"opencsg.com/csghub-server/common/utils/payment/consts"

	"github.com/google/uuid"
)

var (
	ErrDuplicatedEvent         = errors.New("duplicated fee event uuid")
	ErrDuplicatedMeterByUUID   = errors.New("duplicated metering event by event uuid")
	ErrDuplicatedMeterInMinute = errors.New("duplicated metering event in minute level")
)

const (
	OrderDetailID      = "order_detail_id"
	MeterFromSource    = "from_source"
	PromptTokenNum     = "prompt_token_num"
	CompletionTokenNum = "completion_token_num"
)

type OrderStatus int

var (
	OrderCreated    OrderStatus = 1 // created and unpaid
	OrderPayTimeour OrderStatus = 2 // payment timeout
	OrderPaid       OrderStatus = 3 // paid and valid
	OrderCancelled  OrderStatus = 4 // cancelled by user
	OrderExpired    OrderStatus = 5 // Expired
	OrderClosed     OrderStatus = 6 // closed by admin or system
)

type SkuUnitType string

var (
	UnitMinute SkuUnitType = "minute"
	UnitDay    SkuUnitType = "day"
	UnitWeek   SkuUnitType = "week"
	UnitMonth  SkuUnitType = "month"
	UnitYear   SkuUnitType = "year"
	UnitToken  SkuUnitType = "token"
	UnitRepo   SkuUnitType = "repository"
	UnitByte   SkuUnitType = "byte"
)

type ACCTStatus int

var (
	ACCTSuccess       ACCTStatus = 0 // charge success
	ACCTInvalidFormat ACCTStatus = 1 // invalid event data format
	ACCTChargeFail    ACCTStatus = 2 // fail to charge user fee
	ACCTLackBalance   ACCTStatus = 3 // balance <= 0
	ACCTDuplicated    ACCTStatus = 4 // duplicated charge by event uuid
	ACCTSubscription  ACCTStatus = 5 // subscription change
	ACCTStopDeploy    ACCTStatus = 6 // stop deploy service
)

type SKUType int

var (
	SKUReserve  SKUType = 0 // system reserve
	SKUCSGHub   SKUType = 1 // csghub server
	SKUStarship SKUType = 2 // starship
)

type SKUKind int

var (
	SKUPayAsYouGo      SKUKind = 1 // Time-based billing Pay-as-you-go
	SKUTimeSpan        SKUKind = 2 // monthly or yearly billing
	SKUPackageAddon    SKUKind = 3 // Package addon time-based billing
	SKUPromptToken     SKUKind = 4 // Token-based billing of prompt
	SKUCompletionToken SKUKind = 5 // Token-based billing of completion
)

var (
	ChargeBalance     string = "balance"
	ChargeCashBalance string = "cash_balance"
)

type SceneType int

var (
	SceneReserve         SceneType = 0 // system reserve
	ScenePortalCharge    SceneType = 1 // portal charge fee
	ScenePayOrder        SceneType = 2 // create order to reduce fee
	SceneCashCharge      SceneType = 3 // cash charge from user payment
	ScenePaySubscription SceneType = 4 // pay subscription and reduce fee
	// csghub
	SceneModelInference  SceneType = 10 // model inference endpoint
	SceneSpace           SceneType = 11 // csghub space
	SceneModelFinetune   SceneType = 12 // model finetune
	SceneMultiSync       SceneType = 13 // multi source sync
	SceneEvaluation      SceneType = 14 // model evaluation
	SceneModelServerless SceneType = 15 // model serverless deploy
	// starship
	SceneStarship SceneType = 20 // starship
	SceneGuiAgent SceneType = 22 // gui agent
	// unknow
	SceneUnknow SceneType = 99 // unknow
)

type TokenUsageType string

var (
	ExternalInference                  TokenUsageType = "0"
	CSGHubUserDeployedInference        TokenUsageType = "1"
	CSGHubOtherDeployedInference       TokenUsageType = "2"
	CSGHubServerlessInference          TokenUsageType = "3"
	CSGHubOrganFellowDeployedInference TokenUsageType = "4"
)

type ChargeValueType int

var (
	TimeDurationMinType ChargeValueType = 0
	TokenNumberType     ChargeValueType = 1
	QuotaNumberType     ChargeValueType = 2
)

type AcctEventReq struct {
	EventUUID        uuid.UUID       `json:"event_uuid"`
	UserUUID         string          `json:"user_uuid"`
	Value            float64         `json:"value"`
	Scene            SceneType       `json:"scene"`
	OpUID            string          `json:"op_uid"`
	CustomerID       string          `json:"customer_id"`
	EventDate        time.Time       `json:"event_date"`
	Price            float64         `json:"price"`
	PriceUnit        string          `json:"price_unit"`
	Consumption      float64         `json:"consumption"`
	ValueType        ChargeValueType `json:"value_type"`
	ResourceID       string          `json:"resource_id"`
	ResourceName     string          `json:"resource_name"`
	SkuID            int64           `json:"sku_id"`
	RecordedAt       time.Time       `json:"recorded_at"`
	SkuUnit          int64           `json:"sku_unit"`
	SkuUnitType      SkuUnitType     `json:"sku_unit_type"`
	SkuPriceCurrency string          `json:"sku_price_currency"`
	Quota            float64         `json:"quota"`
	SubBillID        int64           `json:"sub_bill_id"`
	Discount         float64         `json:"discount"`
	RegularValue     float64         `json:"regular_value"`
}

// generate charge event from client
type AcctEvent struct {
	Uuid         uuid.UUID       `json:"uuid"`       // event uuid
	UserUUID     string          `json:"user_uuid"`  // user uuid
	Value        int64           `json:"value"`      // time duration in minutes or token number
	ValueType    ChargeValueType `json:"value_type"` // 0: credit, 1: token
	Scene        int             `json:"scene"`
	OpUID        string          `json:"op_uid"`        // operator uuid
	ResourceID   string          `json:"resource_id"`   // resource id
	ResourceName string          `json:"resource_name"` // resource name
	CustomerID   string          `json:"customer_id"`   // customer_id will be shown in bill
	CreatedAt    time.Time       `json:"created_at"`    // time of event happen
	Extra        string          `json:"extra"`
}

// notify response to client
type AcctNotify struct {
	Uuid       uuid.UUID  `json:"uuid"`
	UserUUID   string     `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ReasonCode ACCTStatus `json:"reason_code"` // ACCTStatus
	ReasonMsg  string     `json:"reason_msg"`
}

type SubscriptionNotify struct {
	PreResourceID string `json:"pre_resource_id"`
	ResourceID    string `json:"resource_id"`
	PeriodStart   int64  `json:"period_start"`
	PeriodEnd     int64  `json:"period_end"`
}

type AcctSubscriptionNotify struct {
	AcctNotify
	Subscription *SubscriptionNotify `json:"subscription"`
}

type ActStatementsReq struct {
	CurrentUser  string `json:"current_user"`
	UserUUID     string `json:"user_uuid"`
	Scene        int    `json:"scene"`
	InstanceName string `json:"instance_name"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Per          int    `json:"per"`
	Page         int    `json:"page"`
	UserName     string `json:"user_name"`
}

type AcctBillsReq struct {
	UserUUID  string `json:"user_id"`
	Scene     int    `json:"scene"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Per       int    `json:"per"`
	Page      int    `json:"page"`
}

type AcctStatementsRes struct {
	ID               int64       `json:"id"`
	EventUUID        uuid.UUID   `json:"event_uuid"`
	UserUUID         string      `json:"user_id"`
	Value            float64     `json:"value"`
	Scene            int         `json:"scene"`
	OpUID            string      `json:"op_uid"`
	CreatedAt        time.Time   `json:"created_at"`
	CustomerID       string      `json:"instance_name"`
	EventDate        time.Time   `json:"event_date"`
	Price            float64     `json:"price"`
	PriceUnit        string      `json:"price_unit"`
	Consumption      float64     `json:"consumption"`
	SkuUnit          int64       `json:"sku_unit"`
	SkuUnitType      SkuUnitType `json:"sku_unit_type"`
	SkuPriceCurrency string      `json:"sku_price_currency"`
	UserName         string      `json:"user_name"`
	SkuID            int64       `json:"sku_id"`
	SkuType          int         `json:"sku_type"`
	SkuKind          int         `json:"sku_kind"`
	SkuDesc          string      `json:"sku_desc"`
}

// AcctStatementsResFiltered is a filtered version of AcctStatementsRes that excludes certain fields
type AcctStatementsResFiltered struct {
	ID           int64     `json:"id"`
	UserUUID     string    `json:"user_id"`
	Value        float64   `json:"value"`
	Scene        int       `json:"scene"`
	InstanceName string    `json:"instance_name"`
	CreatedAt    time.Time `json:"created_at"`
	Consumption  float64   `json:"consumption"`
	UserName     string    `json:"user_name"`
	SkuID        int64     `json:"sku_id"`
	SkuType      int       `json:"sku_type"`
	SkuKind      int       `json:"sku_kind"`
	SkuDesc      string    `json:"sku_desc"`
}

type RechargeReq struct {
	Value float64   `json:"value" binding:"min=1"`
	OpUID string    `json:"op_uid"`
	Scene SceneType `json:"scene"`
}

type AcctQuotaReq struct {
	RepoCountLimit int64 `json:"repo_count_limit"`
	SpeedLimit     int64 `json:"speed_limit"`
	TrafficLimit   int64 `json:"traffic_limit"`
}

type AcctQuotaStatementReq struct {
	RepoPath string `json:"repo_path"`
	RepoType string `json:"repo_type"`
}

type AcctSummary struct {
	Total            int     `json:"total"`
	TotalValue       float64 `json:"total_value"`
	TotalConsumption float64 `json:"total_consumption"`
}

type ITEM struct {
	Consumption  float64   `json:"consumption"`
	InstanceName string    `json:"instance_name"`
	Value        float64   `json:"value"`
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"`
	RepoPath     string    `json:"repo_path"`
	DeployID     int64     `json:"deploy_id"`
	DeployName   string    `json:"deploy_name"`
	DeployUser   string    `json:"deploy_user"`
}

type BILLS struct {
	Data []ITEM `json:"data"`
	AcctSummary
}

type AcctStatementsResult struct {
	Data []AcctStatementsRes `json:"data"`
	AcctSummary
}

type MeteringEvent struct {
	Uuid         uuid.UUID       `json:"uuid"`       // event uuid
	UserUUID     string          `json:"user_uuid"`  // user uuid
	Value        int64           `json:"value"`      // time duration in minutes or token number
	ValueType    ChargeValueType `json:"value_type"` // 0: duration, 1: token
	Scene        int             `json:"scene"`
	OpUID        string          `json:"op_uid"`        // operator uuid
	ResourceID   string          `json:"resource_id"`   // resource id
	ResourceName string          `json:"resource_name"` // resource name
	CustomerID   string          `json:"customer_id"`   // customer_id will be shown in bill
	CreatedAt    time.Time       `json:"created_at"`    // time of event happen
	Extra        string          `json:"extra"`
}

type AcctPriceCreateReq struct {
	SkuType          SKUType     `json:"sku_type"`
	SkuPrice         int64       `json:"sku_price"`
	SkuUnit          int64       `json:"sku_unit"`
	SkuDesc          string      `json:"sku_desc"`
	ResourceID       string      `json:"resource_id"`
	SkuUnitType      SkuUnitType `json:"sku_unit_type"`
	SkuPriceCurrency string      `json:"sku_price_currency"`
	SkuKind          SKUKind     `json:"sku_kind"`
	Quota            string      `json:"quota"`
	SkuPriceID       int64       `json:"sku_price_id"`
	Discount         float64     `json:"discount" binding:"omitempty,min=0,max=1"`
	UseLimitPrice    int64       `json:"use_limit_price"`
}

type AcctPriceResp struct {
	Id               int64   `json:"id"`
	SkuType          SKUType `json:"sku_type"`
	SkuPrice         int64   `json:"sku_price"`
	SkuUnit          int64   `json:"sku_unit"`
	SkuDesc          string  `json:"sku_desc"`
	ResourceID       string  `json:"resource_id"`
	SkuUnitType      string  `json:"sku_unit_type"`
	SkuPriceCurrency string  `json:"sku_price_currency"`
	SkuKind          SKUKind `json:"sku_kind"`
	Quota            string  `json:"quota"`
	SkuPriceID       int64   `json:"sku_price_id"`
}

type AcctPriceQueryReq struct {
	SkuType     SKUType   `json:"sku_type"`
	ResourceID  string    `json:"resource_id"`
	PriceTime   time.Time `json:"price_time"`
	SkuKind     SKUKind   `json:"sku_kind"`
	SkuUnitType string    `json:"sku_unit_type"`
}

type AcctOrderDetailReq struct {
	ResourceID  string  `json:"resource_id" binding:"required"`
	SkuType     SKUType `json:"sku_type" binding:"required"`
	SkuUnitType string  `json:"sku_unit_type" binding:"required"`
	OrderCount  int     `json:"order_count" binding:"required,min=1"`
	BeginTime   string  `json:"begin_time" binding:"optional_date_format"`
}

type AcctOrderCreateReq struct {
	OrderUUID    uuid.UUID            `json:"order_uuid" binding:"required"`
	UserUUID     string               `json:"user_uuid" binding:"required"`
	OrderDetails []AcctOrderDetailReq `json:"order_details" binding:"required,dive"`
}

type AcctOrderExpiredEvent struct {
	OrderUUID  string    `json:"order_uuid"`
	UserUUID   string    `json:"user_uuid"`
	DetailID   int64     `json:"detail_id"`
	ResourceID string    `json:"resource_id"`
	BeginTime  time.Time `json:"begin_time"`
	EndTime    time.Time `json:"end_time"`
	CreatedAt  time.Time `json:"created_at"`
}

// used for listing prices with pagination
//
// in accounting price DB
type AcctPriceListDBReq struct {
	SkuType    SKUType  `json:"sku_type"`
	SkuKind    string   `json:"sku_kind"`
	ResourceID []string `json:"resource_id"`
	Per        int      `json:"per"`
	Page       int      `json:"page"`
}

// used for listing prices with pagination and filter
//
// in accounting service and starhub server
type AcctPriceListReq struct {
	SkuType    SKUType             `json:"sku_type"`
	SkuKind    string              `json:"sku_kind"`
	ResourceID []string            `json:"resource_id"`
	Filter     AcctPriceListFilter `json:"filter"`
	Per        int                 `json:"per"`
	Page       int                 `json:"page"`
}

type AcctPriceListFilter struct {
	HardwareType string `json:"hardware_type"`
}

type AcctRechargeReq struct {
	ChannelCode    consts.PaymentChannel `json:"channelCode"`
	RechargeAmount float64               `json:"rechargeAmount"` //unit yuan
}

type AcctRechargeResp struct {
	Content         string                `json:"content"`
	RechargeUUID    string                `json:"rechargeUUID"`
	RechargeOrderNo string                `json:"orderNo"`
	Channel         consts.PaymentChannel `json:"channel"`
	CreateTime      time.Time             `json:"createTime"` //2024-11-18 15:50:47
}

type RechargeStatusResp struct {
	RechargeUUID      string `json:"rechargeUUID"`
	RechargeSucceeded bool   `json:"rechargeSucceeded"`
}

type RechargeResp struct {
	RechargeUUID   string    `json:"uuid"`
	OrderNo        string    `json:"order_no"`
	UserUUID       string    `json:"user_uuid"`
	FromUserUUID   string    `json:"from_user_uuid"`
	Amount         int64     `json:"amount"`
	Currency       string    `json:"currency"`
	PaymentUUID    string    `json:"payment_uuid"`
	PaymentChannel string    `json:"recharge_payment_type"`
	Succeeded      bool      `json:"succeeded"`
	Closed         bool      `json:"closed"`
	TimeSucceeded  time.Time `json:"time_succeeded"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Description    string    `json:"description"`
	UserName       string    `json:"user_name"`
}

const RechargeExtraType = "recharge"

type ComposedBalance struct {
	Cash  float64 `json:"cash"`
	Bonus float64 `json:"bonus"`
}

type UserBalanceResp struct {
	UserUUID       string          `json:"userUUID"`
	Balance        float64         `json:"balance"`
	Composition    ComposedBalance `json:"composition"`
	LowBalanceWarn float64         `json:"low_balance_warn"`
	LastWarnAt     time.Time       `json:"low_balance_warnat"`
	NegativeWarnAt time.Time       `json:"negative_balance_warnat"`
}

type AcctRechargeListReq struct {
	UserUUID    string `json:"user_uuid"`
	Scene       int    `json:"scene"`
	ActivityID  int64  `json:"activity_id"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Per         int    `json:"per"`
	Page        int    `json:"page"`
	CurrentUser string `json:"-"`
}

type AcctRecharge struct {
	ID         int64     `json:"id"`
	EventUUID  string    `json:"event_uuid"`
	UserUUID   string    `json:"user_id"`
	Value      float64   `json:"value"`
	Scene      int       `json:"scene"`
	OpUID      string    `json:"op_uid"`
	CreatedAt  time.Time `json:"created_at"`
	EventDate  time.Time `json:"event_date"`
	ActivityID int64     `json:"activity_id"`
	OpDesc     string    `json:"op_desc"`
}

type AcctRechargeListResp struct {
	Data       []AcctRecharge `json:"data"`
	Total      int            `json:"total"`
	TotalValue float64        `json:"total_value"`
}

type RechargesIndexReq struct {
	UserName    string `json:"user_name"`
	UserUUID    string `json:"user_uuid"`
	OrderNo     string `json:"order_no"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Status      string `json:"recharge_status"`
	PaymentType string `json:"recharge_payment_type"`
	Per         int    `json:"per"`
	Page        int    `json:"page"`
}

type RechargeIndexResp struct {
	RechargeResp
	Amount float64 `json:"amount"`
}

type RechargesIndexResp struct {
	Data  []*RechargeIndexResp `json:"data"`
	Total int                  `json:"total"`
	Sum   int64                `json:"sum"` // Total recharge amount
}

type SetLowBalanceWarnReq struct {
	LowBalanceWarn float64 `json:"low_balance_warn"`
	UserUUID       string  `json:"user_uuid"`
}

type AccInvoiceTitleReq struct {
	UserUUID string `json:"-"`
	UserName string `json:"-"`

	Title        string `json:"title" binding:"required"`      // Invoice title name
	TitleType    string `json:"title_type" binding:"required"` // Invoice title type
	InvoiceType  string `json:"invoice_type" binding:"required"`
	TaxID        string `json:"tax_id" binding:"required"`        // Taxpayer identification number
	Address      string `json:"address"`                          // Registered address
	BankName     string `json:"bank_name"`                        // Bank name
	BankAccount  string `json:"bank_account"`                     // Bank account number
	ContactPhone string `json:"contact_phone" binding:"required"` // Contact phone number
	Email        string `json:"email" binding:"required,email"`   // Email address
	IsDefault    bool   `json:"is_default"`
}

type AccInvoiceTitleResp struct {
	ID           int64     `json:"id"`
	UserUUID     string    `json:"user_uuid"` // User
	UserName     string    `json:"user_name"`
	Title        string    `json:"title"`         // Invoice title name
	TitleType    string    `json:"title_type"`    // Invoice title type
	TaxID        string    `json:"tax_id"`        // Taxpayer identification number
	Address      string    `json:"address"`       // Registered address
	BankName     string    `json:"bank_name"`     // Bank name
	BankAccount  string    `json:"bank_account"`  // Bank account number
	ContactPhone string    `json:"contact_phone"` // Contact phone number
	Email        string    `json:"email"`         // Email address
	IsDefault    bool      `json:"is_default"`    // Whether it is the default title
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AccInvoiceTitleListReq struct {
	UserUUID string `json:"-"`

	Page     int    `json:"page" binding:"min=1"`              // Current page number
	PageSize int    `json:"page_size" binding:"min=1,max=100"` // Number of items per page
	Search   string `json:"search"`                            // Search field
}

type AccInvoiceTitleListResp struct {
	Data  []AccInvoiceTitleResp `json:"data"`
	Total int                   `json:"total"`
}

type AccInvoiceListReq struct {
	UserUUID string `json:"-"`

	Page     int    `json:"page" binding:"min=1"`              // Current page number
	PageSize int    `json:"page_size" binding:"min=1,max=100"` // Number of items per page
	Search   string `json:"search"`                            // Search field

	Status string `json:"status"` // Invoice status eq processing,issued,failed
}

type AccInvoiceListResp struct {
	Data  []AccInvoiceResp `json:"data"`
	Total int              `json:"total"`
}

type AccInvoiceResp struct {
	ID             int       `json:"id"`
	UserUUID       string    `json:"user_uuid"`
	UserName       string    `json:"user_name"`       // User name
	TitleType      string    `json:"title_type"`      // Invoice title type
	InvoiceType    string    `json:"invoice_type"`    // Invoice type
	BillCycle      string    `json:"bill_cycle"`      // Billing cycle
	InvoiceTitle   string    `json:"invoice_title"`   // Invoice title
	ApplyTime      time.Time `json:"apply_time"`      // Invoice application time
	InvoiceAmount  float64   `json:"invoice_amount"`  // Invoice amount
	Status         string    `json:"status"`          // Invoice status eq processing,issued,failed
	Reason         string    `json:"reason"`          // Reason
	InvoiceDate    time.Time `json:"invoice_date"`    // Invoice issuance date
	InvoiceURL     string    `json:"invoice_url"`     // Invoice URL
	TaxpayerID     string    `json:"taxpayer_id"`     // Taxpayer identification number
	BankName       string    `json:"bank_name"`       // Bank name
	BankAccount    string    `json:"bank_account"`    // Bank account number
	RegisteredAddr string    `json:"registered_addr"` // Registered address
	ContactPhone   string    `json:"contact_phone"`   // Contact phone number
	Email          string    `json:"email"`           // Email address
	CreatedAt      time.Time `json:"created_at"`      // Creation time
	UpdatedAt      time.Time `json:"updated_at"`      // Update time
}

type AccInvoiceDashboardResp struct {
	CurrentMonthNonInvoicable float64 `json:"current_month_non_invoicable"`
	InvoicedAmount            float64 `json:"invoiced_amount"`
	UninvoicedAmount          float64 `json:"uninvoiced_amount"`
}

type AccInvoiceDashboardReq struct {
	UserUUID string `json:"-"`

	StartMonth string `json:"start_month" binding:"required"`
	EndMonth   string `json:"end_month" binding:"required"`
}

type AccInvoiceCreateReq struct {
	UserUUID      string  `json:"-"`
	UserName      string  `json:"-"`
	TitleID       int64   `json:"title_id" binding:"required"`
	BillCycle     string  `json:"bill_cycle" binding:"required"`
	InvoiceAmount float64 `json:"invoice_amount" binding:"required"`
}

type AccInvoicableReq struct {
	UserUUID   string `json:"-"`
	Page       int    `json:"page" binding:"min=1"`              // Current page number
	PageSize   int    `json:"page_size" binding:"min=1,max=100"` // Number of items per page
	StartMonth string `json:"start_month"`
	EndMonth   string `json:"end_month"`
}
type AccInvoicableResp struct {
	Data  []AccInvoicable `json:"data"`
	Total int             `json:"total"`
}

type AccInvoicable struct {
	BillCycle string  `json:"bill_cycle"`
	Amount    float64 `json:"amount"`
}

type AdminUpdateInvoiceReq struct {
	ID         int64  `json:"-"`
	Status     string `json:"status" binding:"required"` //Invoice status eq processing,issued,failed
	Reason     string `json:"reason"`
	InvoiceURL string `json:"invoice_url"`
}

type RechargeStats struct {
	Count int   `bun:"count"`
	Sum   int64 `bun:"sum"`
}

type AccountPresentStatus int

const (
	AccountPresentStatusInit     AccountPresentStatus = 0
	AccountPresentStatusUsed     AccountPresentStatus = 1
	AccountPresentStatusCanceled AccountPresentStatus = 2
)
