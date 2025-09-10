package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type OrganizationVerifyComponent interface {
	Create(ctx context.Context, req *types.OrgVerifyReq) (*database.OrganizationVerify, error)
	Update(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*database.OrganizationVerify, error)
	Get(ctx context.Context, path string) (*database.OrganizationVerify, error)
	sendNotification(ctx context.Context, status types.VerifyStatus, userUUID string) error
}

type OrganizationVerifyComponentImpl struct {
	orgVerifyStore        database.OrganizationVerifyStore
	orgStore              database.OrgStore
	notificationSvcClient rpc.NotificationSvcClient
	config                *config.Config
}

func NewOrganizationVerifyComponent(config *config.Config) (OrganizationVerifyComponent, error) {
	c := &OrganizationVerifyComponentImpl{}
	c.orgVerifyStore = database.NewOrganizationVerifyStore()
	c.orgStore = database.NewOrgStore()
	c.config = config
	c.notificationSvcClient = rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return c, nil
}

func (o *OrganizationVerifyComponentImpl) Create(ctx context.Context, req *types.OrgVerifyReq) (*database.OrganizationVerify, error) {
	orgVerify, err := o.orgVerifyStore.CreateOrganizationVerify(ctx, &database.OrganizationVerify{
		Name:               req.Name,
		CompanyName:        req.CompanyName,
		UnifiedCreditCode:  req.UnifiedCreditCode,
		Username:           req.Username,
		ContactName:        req.ContactName,
		ContactEmail:       req.ContactEmail,
		BusinessLicenseImg: req.BusinessLicenseImg,
		Status:             types.VerifyStatusPending,
		Reason:             "",
		UserUUID:           req.UserUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization verify, error: %w", err)
	}

	err = o.orgStore.UpdateVerifyStatus(ctx, orgVerify.Name, types.VerifyStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization verify status, error: %w", err)
	}

	return orgVerify, nil
}

func (o *OrganizationVerifyComponentImpl) Update(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*database.OrganizationVerify, error) {
	orgVerify, err := o.orgVerifyStore.UpdateOrganizationVerify(ctx, id, status, reason)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization verify, error: %w", err)
	}

	err = o.orgStore.UpdateVerifyStatus(ctx, orgVerify.Name, status)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization verify status, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = o.sendNotification(notificationCtx, status, orgVerify.UserUUID)
		if err != nil {
			slog.Error("failed to send organization verify notification", slog.Any("error", err))
		}
	}()

	return orgVerify, nil
}

func (o *OrganizationVerifyComponentImpl) Get(ctx context.Context, path string) (*database.OrganizationVerify, error) {
	orgVerify, err := o.orgVerifyStore.GetOrganizationVerify(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to find organization verify, error: %w", err)
	}
	return orgVerify, nil
}

func (o *OrganizationVerifyComponentImpl) sendNotification(ctx context.Context, status types.VerifyStatus, userUUID string) error {
	if userUUID == "" {
		return fmt.Errorf("userUUID is empty")
	}

	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        []string{userUUID},
		NotificationType: types.NotificationSystem,
		CreateAt:         time.Now(),
		Template:         string(types.MessageScenarioOrgVerify),
		Payload: map[string]any{
			"verify_status": status,
		},
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}
	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioOrgVerify,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := o.config.Notification.NotificationRetryCount
	for i := range retryCount {
		if sendErr = o.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.Warn("failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}

	return nil
}
