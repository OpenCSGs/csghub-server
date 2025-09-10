//go:build saas

package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"opencsg.com/csghub-server/builder/deploy/common"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (d *deployer) registerStopDeployConsuming() {
	for {
		subParam := bldmq.SubscribeParams{
			Group: bldmq.AccountingNotifyGroup,
			Topics: []string{
				bldmq.NotifyStopDeploySubject,
			},
			AutoACK:  true,
			Callback: d.stopUserDeployCallback,
		}

		err := d.eventPub.MQ.Subscribe(subParam)
		if err == nil {
			slog.Info("register stop user deploy consumer successfully")
			break
		}

		slog.Error("failed to register stop user deploy consumer and retry after 5 seconds", slog.Any("error", err))
		time.Sleep(5 * time.Second)
	}

}

func (d *deployer) stopUserDeployCallback(raw []byte, meta bldmq.MessageMeta) error {
	if meta.Topic != bldmq.NotifyStopDeploySubject {
		slog.Warn("received wrong notification of topic in stop deploy callback", slog.Any("topic", meta.Topic))
		return nil
	}

	var err error
	for range 3 {
		err = d.stopUserRunningDeploys(raw)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to stop user deploy with retry 3 times msg: %s, error: %w", string(raw), err)
	}

	return nil
}

func (d *deployer) stopUserRunningDeploys(raw []byte) error {
	notifyEvt := types.AcctNotify{}
	err := json.Unmarshal(raw, &notifyEvt)
	if err != nil {
		return fmt.Errorf("failed to unmarshal stop user deploy notify event: %w", err)
	}

	if notifyEvt.ReasonCode != types.ACCTStopDeploy {
		slog.Warn("received wrong reason code of stop user deploy notify event", slog.Any("notifyEvt", notifyEvt))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := d.userStore.FindByUUID(ctx, notifyEvt.UserUUID)
	if err != nil {
		slog.Warn("failed to find user by uuid to stop deploy", slog.Any("user UUID", notifyEvt.UserUUID))
		return nil
	}

	deploys, err := d.deployTaskStore.GetRunningDeployByUserID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get running deploys by user id %d to stop, %w", user.ID, err)
	}

	for _, deploy := range deploys {
		d.stopUserDeploy(ctx, deploy, notifyEvt)
	}
	return nil
}

func (d *deployer) stopUserDeploy(ctx context.Context, deploy database.Deploy, notifyEvt types.AcctNotify) {
	if deploy.OrderDetailID > 0 {
		ur, err := d.userResStore.FindUserResourcesByOrderDetailId(ctx, deploy.UserUUID, deploy.OrderDetailID)
		if err != nil {
			slog.Warn("get reserved deploy for stop notification", slog.Any("deploy ID", deploy.ID), slog.Any("OrderDetailID", deploy.OrderDetailID), slog.Any("error", err))
			return
		}
		if ur.StartTime.Before(time.Now()) && ur.EndTime.After(time.Now()) {
			// should not stop for valid deploy of payment by monthly or yearly
			return
		}
	}

	price, err := d.queryPrice(deploy)
	if err != nil {
		slog.Error("query price for stop notification", slog.Any("error", err), slog.Any("deploy ID", deploy.ID), slog.Any("notifyEvt", notifyEvt))
		return
	}

	if price.SkuPrice <= 0 {
		// free deploy, no need to stop
		slog.Debug("free deploy, no need to stop", slog.Any("deploy ID", deploy.ID), slog.Any("notifyEvt", notifyEvt))
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromGitPath(deploy.GitPath)
	if err != nil {
		slog.Warn("get namespace/name for stop notification", slog.Any("error", err), slog.Any("GitPath", deploy.GitPath), slog.Any("notifyEvt", notifyEvt))
		return
	}

	// stop deploy service
	deployRepo := types.DeployRepo{
		DeployID:      deploy.ID,
		SpaceID:       deploy.SpaceID,
		ModelID:       deploy.ModelID,
		Namespace:     namespace,
		Name:          name,
		SvcName:       deploy.SvcName,
		ClusterID:     deploy.ClusterID,
		OrderDetailID: deploy.OrderDetailID,
		UserUUID:      notifyEvt.UserUUID,
	}
	slog.Debug("do stop action for stop deploy notification", slog.Any("deployRepo", deployRepo))

	err = d.Stop(ctx, deployRepo)
	if err != nil {
		// fail to stop deploy instance, maybe service is gone
		slog.Warn("failed to stop deploy instance for stop notification", slog.Any("error", err), slog.Any("deployRepo", deployRepo), slog.Any("notifyEvt", notifyEvt))
	}

	time.Sleep(5 * time.Second)

	exist, err := d.Exist(ctx, deployRepo)
	if err != nil {
		slog.Warn("check if deploy instance exists for stop notification", slog.Any("error", err), slog.Any("deployRepo", deployRepo), slog.Any("notifyEvt", notifyEvt))
	}

	if exist {
		// fail to delete service
		slog.Warn("failed to stop deploy instance for stop notification", slog.Any("error", err), slog.Any("deployRepo", deployRepo), slog.Any("notifyEvt", notifyEvt))
		return
	}

	// update database deploy for stopped
	repoType := types.ModelRepo
	if deploy.SpaceID > 0 {
		repoType = types.SpaceRepo
	}

	err = d.deployTaskStore.StopDeploy(ctx, repoType, deploy.RepoID, deploy.UserID, deploy.ID)
	if err != nil {
		slog.Warn("failed to update deploy status for stop notification", slog.Any("error", err), slog.Any("deployRepo", deployRepo), slog.Any("notifyEvt", notifyEvt))
	} else {
		slog.Info("stop deploy instance successfully", slog.Any("deployRepo", deployRepo), slog.Any("notifyEvt", notifyEvt))
	}

}

func (d *deployer) queryPrice(deploy database.Deploy) (*database.AccountPrice, error) {
	req := types.AcctPriceListReq{
		SkuType:    types.SKUCSGHub,
		SkuKind:    strconv.Itoa(int(types.SKUPayAsYouGo)),
		ResourceID: deploy.SKU,
		Per:        1,
		Page:       1,
	}

	resp, err := d.acctClient.QueryPricesBySKUType("", req)
	if err != nil {
		return nil, fmt.Errorf("failed to query price in notification error: %w", err)
	}
	tempJSON, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("error to marshal json for query price, error: %w", err)
	}

	var priceData database.PriceResp
	if err := json.Unmarshal(tempJSON, &priceData); err != nil {
		return nil, fmt.Errorf("error to unmarshal json for query price, error: %w", err)
	}

	if len(priceData.Prices) <= 0 {
		return nil, fmt.Errorf("empty price list for deploy %d sku %s", deploy.ID, deploy.SKU)
	}

	return &priceData.Prices[0], nil
}

func (d *deployer) startAcctOrderConsuming() {
	for {
		consumer, err := d.eventPub.CreateOrderExpiredConsumer()
		if err != nil {
			slog.Error("fail to create continuous polling order expired consumer", slog.Any("error", err))
		} else {
			_, err = consumer.Consume(d.acctOrderExpiredConsumerCallback)
			if err != nil {
				slog.Error("fail to begin consuming order expired message", slog.Any("error", err))
			} else {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (d *deployer) acctOrderExpiredConsumerCallback(msg jetstream.Msg) {
	slog.Debug("Received an order expired message", slog.Any("msg", string(msg.Data())))
	err := msg.Ack()
	if err != nil {
		slog.Warn("fail to ack after processing order expired message", slog.Any("msg", string(msg.Data())))
	}
}
