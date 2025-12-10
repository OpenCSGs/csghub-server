package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type LfsXnetProcessedMessage struct {
	RepoID int64  `json:"repo_id"`
	Oid    string `json:"oid"`
}

type LfsComponent interface {
	DispatchLfsXnetProcessed() error
}

type lfsComponentImpl struct {
	cfg                *config.Config
	mq                 bldmq.MessageQueue
	lfsMetaObjectStore database.LfsMetaObjectStore
}

func NewLfsComponent(config *config.Config, mqFactory bldmq.MessageQueueFactory) (LfsComponent, error) {
	mq, err := mqFactory.GetInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get mq instance error: %w", err)
	}

	return &lfsComponentImpl{
		cfg:                config,
		mq:                 mq,
		lfsMetaObjectStore: database.NewLfsMetaObjectStore(),
	}, nil
}

func (l *lfsComponentImpl) DispatchLfsXnetProcessed() error {
	err := l.mq.Subscribe(bldmq.SubscribeParams{
		Group:    bldmq.LfsXnetProcessedGroup,
		Topics:   []string{bldmq.LfsXnetProcessedSubject},
		AutoACK:  true,
		Callback: l.handleLfsXnetProcessedMsg,
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe lfs xnet processed event error: %w", err)
	}

	return nil
}

func (l *lfsComponentImpl) handleLfsXnetProcessedMsg(raw []byte, meta bldmq.MessageMeta) error {
	strData := string(raw)
	slog.Debug("mq.lfs.xnet.received", slog.Any("msg.subject", meta.Topic), slog.Any("msg.data", strData))

	var msg LfsXnetProcessedMessage
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal lfs xnet processed message error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = l.lfsMetaObjectStore.UpdateXnetUsed(ctx, msg.RepoID, msg.Oid, true)
	if err != nil {
		return fmt.Errorf("failed to update xnet_used for repo_id=%d, oid=%s error: %w", msg.RepoID, msg.Oid, err)
	}

	slog.Info("lfs xnet processed successfully", slog.Int64("repo_id", msg.RepoID), slog.String("oid", msg.Oid))
	return nil
}
