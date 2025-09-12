package types

import (
	"time"
)

type NotificationType string

const (
	NotificationSystem               NotificationType = "system"
	NotificationComment              NotificationType = "comment"
	NotificationOrganization         NotificationType = "organization"
	NotificationAssetManagement      NotificationType = "asset_management"
	NotificationDeploymentManagement NotificationType = "deployment"
)

type ActionType string

func NotificationTypeAll() []string {
	return []string{
		string(NotificationComment),
		string(NotificationSystem),
		string(NotificationOrganization),
		string(NotificationAssetManagement),
		string(NotificationDeploymentManagement),
	}
}

func (t NotificationType) String() string {
	return string(t)
}

func (t NotificationType) IsSystem() bool {
	return t == NotificationSystem
}

var validNotificationTypes = map[NotificationType]bool{
	NotificationSystem:               true,
	NotificationComment:              true,
	NotificationOrganization:         true,
	NotificationAssetManagement:      true,
	NotificationDeploymentManagement: true,
}

func (t NotificationType) IsValid() bool {
	return validNotificationTypes[t]
}

type NotificationMessage struct {
	MsgUUID          string           `json:"msg_uuid"`
	UserUUIDs        []string         `json:"user_uuids"`
	SenderUUID       string           `json:"sender_uuid"`
	NotificationType NotificationType `json:"notification_type"`
	Title            string           `json:"title"`
	Summary          string           `json:"summary"`
	Content          string           `json:"content"`
	ClickActionURL   string           `json:"click_action_url"`
	Priority         int              `json:"priority"`
	CreateAt         time.Time        `json:"create_at"`
	// Template and Payload are used to render the message content in portal,
	Template string         `json:"template"` // template value should be the same as the scenario name
	Payload  map[string]any `json:"payload"`  // used to render the message content in portal
}

type NotificationsResp struct {
	Messages    []Notifications `json:"messages"`
	UnreadCount int64           `json:"unread_count"`
	TotalCount  int64           `json:"total_count"`
}

type NewNotifications struct {
	NextPollTime time.Time       `json:"next_poll_time"`
	Data         []Notifications `json:"data"`
}

type Notifications struct {
	ID               int64          `json:"id"`
	UserUUID         string         `json:"user_uuid"`
	SenderUUID       string         `json:"sender_uuid"`
	NotificationType string         `json:"notification_type"`
	Title            string         `json:"title"`
	Summary          string         `json:"summary"`
	Content          string         `json:"content"`
	Template         string         `json:"template"`
	Payload          map[string]any `json:"payload"`
	IsRead           bool           `json:"is_read"`
	ClickActionURL   string         `json:"click_action_url"`
	CreatedAt        int64          `json:"created_at"`
	UpdatedAt        int64          `json:"updated_at"`
}

type NotificationsRequest struct {
	Page             int    `form:"page"`
	PageSize         int    `form:"page_size"`
	NotificationType string `form:"notification_type"`
	UnReadOnly       bool   `form:"unread_only"`
	Title            string `form:"title"`
}

// BatchNotificationOperationReq represents a common request structure for batch notification operations
// such as mark as read, mark as unread, and delete operations
type BatchNotificationOperationReq struct {
	IDs     []int64 `json:"ids"`
	MarkAll bool    `json:"mark_all"`
}

type UpdateNotificationReq struct {
	SubNotificationType        []string  `json:"sub_notification_type"`
	EmailAddress               string    `json:"email_address"`
	IsEmailNotificationEnabled bool      `json:"is_email_notification_enabled"`
	MessageTTL                 int64     `json:"message_ttl"`
	PhoneNumber                string    `json:"phone_number"`
	IsSMSNotificationEnabled   bool      `json:"is_sms_notification_enabled"`
	DoNotDisturbStart          string    `json:"do_not_disturb_start"`
	DoNotDisturbEnd            string    `json:"do_not_disturb_end"`
	DoNotDisturbStartTime      time.Time `json:"-"`
	DoNotDisturbEndTime        time.Time `json:"-"`
	IsDoNotDisturbEnabled      bool      `json:"is_do_not_disturb_enabled"`
	Timezone                   string    `json:"timezone" binding:"required"`
}

type NotificationSetting struct {
	UserUUID                   string    `json:"user_uuid"`
	SubNotificationType        []string  `json:"sub_notification_type"`
	EmailAddress               string    `json:"email_address"`
	IsEmailNotificationEnabled bool      `json:"is_email_notification_enabled"`
	MessageTTL                 int64     `json:"message_ttl"`
	PhoneNumber                string    `json:"phone_number"`
	IsSMSNotificationEnabled   bool      `json:"is_sms_notification_enabled"`
	DoNotDisturbStart          string    `json:"do_not_disturb_start"`
	DoNotDisturbEnd            string    `json:"do_not_disturb_end"`
	IsDoNotDisturbEnabled      bool      `json:"is_do_not_disturb_enabled"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type MailType string

const (
	MailSystem          MailType = "system"
	MailRechargeSucceed MailType = "recharge_succeed"
	MailWeeklyRecharges MailType = "weekly_recharges"
)

func (t MailType) IsValid() bool {
	switch t {
	case MailSystem, MailRechargeSucceed, MailWeeklyRecharges:
		return true
	default:
		return false
	}
}

type MailMessage struct {
	MsgUUID   string    `json:"msg_uuid"`
	UserUUIDs []string  `json:"user_uuid"`
	Mails     []string  `json:"mails"`
	MailType  MailType  `json:"mail_type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	FileName  string    `json:"file_name"`
	CreateAt  time.Time `json:"create_at"`
	DataJson  string    `json:"data_json"`
}

type LarkMessagePriority string

const (
	LarkMessagePriorityHigh   LarkMessagePriority = "high"
	LarkMessagePriorityNormal LarkMessagePriority = "normal"
)

type LarkMessageType string

const (
	LarkMessageTypeText        LarkMessageType = "text"
	LarkMessageTypePost        LarkMessageType = "post"
	LarkMessageTypeInteractive LarkMessageType = "interactive"
)

type LarkMessageReceiveIDType string

const (
	LarkMessageReceiveIDTypeOpenID LarkMessageReceiveIDType = "open_id"
	LarkMessageReceiveIDTypeChatID LarkMessageReceiveIDType = "chat_id"
)

type LarkMessage struct {
	Priority      LarkMessagePriority      `json:"priority"`
	MsgType       LarkMessageType          `json:"msg_type"`
	ReceiveIDType LarkMessageReceiveIDType `json:"receive_id_type"`
	ReceiveID     string                   `json:"receive_id"`
	Content       string                   `json:"content"`

	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Size      int       `json:"size"`
}

<<<<<<< HEAD
=======
type MessageScenario string

const (
	MessageScenarioRepoSync             MessageScenario = "repo-sync"
	MessageScenarioInternalNotification MessageScenario = "internal-notification"
	MessageScenarioEmailVerifyCode      MessageScenario = "email-verify-code"
	MessageScenarioSMSVerifyCode        MessageScenario = "sms-verify-code"
	MessageScenarioAssetManagement      MessageScenario = "asset-management"
	MessageScenarioUserVerify           MessageScenario = "user-verify"
	MessageScenarioOrgVerify            MessageScenario = "org-verify"
	MessageScenarioOrgMember            MessageScenario = "org-member"
	MessageScenarioDiscussion           MessageScenario = "discussion"
	MessageScenarioRecharge             MessageScenario = "recharge"
	MessageScenarioLowBalance           MessageScenario = "low-balance"
	MessageScenarioRechargeSuccess      MessageScenario = "recharge-success"
	MessageScenarioWeeklyRecharges      MessageScenario = "weekly-recharges"
	MessageScenarioDeployment           MessageScenario = "deployment"
	MessageScenarioNegativeBalance      MessageScenario = "negative-balance"
)

>>>>>>> 0003cf1a (update user's phone number using sms verify code in backend)
type MessageChannel string

const (
	MessageChannelLark            MessageChannel = "lark"
	MessageChannelEmail           MessageChannel = "email"
	MessageChannelSMS             MessageChannel = "sms"
	MessageChannelInternalMessage MessageChannel = "internal-message"
)

func (t MessageChannel) String() string {
	return string(t)
}

type MessagePriority string

const (
	MessagePriorityHigh   MessagePriority = "high"
	MessagePriorityNormal MessagePriority = "normal"
)

type MessageRequest struct {
	Scenario   MessageScenario `json:"scenario" binding:"required"`
	Parameters string          `json:"parameters" binding:"required"`
	Priority   MessagePriority `json:"priority" binding:"required,oneof=high normal"`
}

type ScenarioMessage struct {
	MsgUUID    string          `json:"msg_uuid"`
	Scenario   MessageScenario `json:"scenario"`
	Parameters string          `json:"parameters"`
	Priority   MessagePriority `json:"priority"`
	CreatedAt  time.Time       `json:"created_at"`
}

type EmailContentType string

const (
	ContentTypeTextPlain EmailContentType = "text/plain"
	ContentTypeTextHTML  EmailContentType = "text/html"
)

type EmailAttachment struct {
	Name    string `json:"name"`
	Content []byte `json:"content"`
	Path    string `json:"path"`
}

type EmailReq struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	CC          []string          `json:"cc"`
	BCC         []string          `json:"bcc"`
	Subject     string            `json:"subject"`
	Body        string            `json:"body"`
	ContentType EmailContentType  `json:"content_type"` // "text/plain" or "text/html"
	Attachments []EmailAttachment `json:"attachments"`
	Source      EmailSource       `json:"source"` // source of fetching email address, eg: notification setting table, user table, etc.
}

type EmailSource string

const (
	EmailSourceNotificationSetting EmailSource = "notification_setting" // from notification setting table
	EmailSourceUser                EmailSource = "user"                 // from user table
)

type OrgMemberOperation string

const (
	OrgMemberOperationAdd    OrgMemberOperation = "add"
	OrgMemberOperationRemove OrgMemberOperation = "remove"
	OrgMemberOperationUpdate OrgMemberOperation = "update"
)

type OrgMemberReq struct {
	UserName  string             `json:"user_name"`
	NewRole   string             `json:"new_role"`
	OrgName   string             `json:"org_name"`
	Operation OrgMemberOperation `json:"operation"`
	UserUUIDs []string           `json:"user_uuids"`
}

type EmailVerifyCodeNotificationReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
	TTL   int64  `json:"ttl"` // in minutes
}

type TemplateOutput struct {
	Title   string
	Content string
}

type EmailRechargeSuccessNotification struct {
	Email      string `json:"email"`
	UserUUID   string `json:"user_uuid"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	PayChannel string `json:"pay_channel"`
}

type EmailWeeklyRechargesNotification struct {
	Email     string `json:"email"`
	FileData  string `json:"file_data"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type SMSReq struct {
	PhoneNumbers  []string `json:"phone_numbers"`
	SignName      string   `json:"sign_name"`
	TemplateCode  string   `json:"template_code"`
	TemplateParam string   `json:"template_param"`
}
