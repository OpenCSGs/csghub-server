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

type UserVerifyComponent interface {
	Create(ctx context.Context, req *types.UserVerifyReq) (*database.UserVerify, error)
	Update(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*database.UserVerify, error)
	Get(ctx context.Context, UUID string) (*database.UserVerify, error)
	sendNotification(ctx context.Context, status types.VerifyStatus, userUUID string) error
}

type UserVerifyComponentImpl struct {
	userVerifyStore       database.UserVerifyStore
	userStore             database.UserStore
	notificationSvcClient rpc.NotificationSvcClient
	config                *config.Config
}

func NewUserVerifyComponent(config *config.Config) (UserVerifyComponent, error) {
	c := &UserVerifyComponentImpl{}
	c.userVerifyStore = database.NewUserVerifyStore()
	c.userStore = database.NewUserStore()
	c.config = config
	c.notificationSvcClient = rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return c, nil
}

func (uv *UserVerifyComponentImpl) Create(ctx context.Context, req *types.UserVerifyReq) (*database.UserVerify, error) {
	userVerify, err := uv.userVerifyStore.CreateUserVerify(ctx, &database.UserVerify{
		UUID:        req.UUID,
		RealName:    req.RealName,
		Username:    req.Username,
		IDCardFront: req.IDCardFront,
		IDCardBack:  req.IDCardBack,
		Status:      types.VerifyStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user verify, error: %w", err)
	}
	err = uv.userStore.UpdateVerifyStatus(ctx, userVerify.UUID, types.VerifyStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to update user verify status, error: %w", err)
	}

	return userVerify, nil
}

func (uv *UserVerifyComponentImpl) Update(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*database.UserVerify, error) {
	userVerify, err := uv.userVerifyStore.UpdateUserVerify(ctx, id, status, reason)
	if err != nil {
		return nil, fmt.Errorf("failed to update user verify, error: %w", err)
	}
	err = uv.userStore.UpdateVerifyStatus(ctx, userVerify.UUID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to update user verify status, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = uv.sendNotification(notificationCtx, status, userVerify.UUID)
		if err != nil {
			slog.ErrorContext(notificationCtx, "failed to send user verify notification", slog.Any("error", err))
		}
	}()
	return userVerify, nil
}

func (uv *UserVerifyComponentImpl) Get(ctx context.Context, uuid string) (*database.UserVerify, error) {
	userVerify, err := uv.userVerifyStore.GetUserVerify(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to find user verify, error: %w", err)
	}
	return userVerify, nil
}

func (uv *UserVerifyComponentImpl) sendNotification(ctx context.Context, status types.VerifyStatus, userUUID string) error {
	if userUUID == "" {
		return fmt.Errorf("userUUID is empty")
	}

	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        []string{userUUID},
		NotificationType: types.NotificationSystem,
		CreateAt:         time.Now(),
		Template:         string(types.MessageScenarioUserVerify),
		Payload: map[string]any{
			"verify_status": status,
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}

	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioUserVerify,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := uv.config.Notification.NotificationRetryCount
	for i := range retryCount {
		if sendErr = uv.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.WarnContext(ctx, "failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}

	return nil
}
