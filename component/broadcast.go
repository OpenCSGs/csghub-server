package component

import (
	"context"
	"encoding/json"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type BroadcastComponent interface {
	NewBroadcast(ctx context.Context, broadcast types.Broadcast) error
	GetBroadcast(ctx context.Context, id int64) (*types.Broadcast, error)
	UpdateBroadcast(ctx context.Context, broadcast types.Broadcast) (*types.Broadcast, error)
	AllBroadcasts(ctx context.Context) ([]types.Broadcast, error)
	ActiveBroadcast(ctx context.Context) (*types.Broadcast, error)
}

type broadcastComponentImpl struct {
	broadcastStore database.BroadcastStore
}

func NewBroadcastComponent() BroadcastComponent {
	return &broadcastComponentImpl{
		broadcastStore: database.NewBroadcastStore(),
	}
}

func (ec *broadcastComponentImpl) NewBroadcast(ctx context.Context, broadcast types.Broadcast) error {
	dbbroadcast := database.Broadcast{
		Content: broadcast.Content,
		BcType:  broadcast.BcType,
		Theme:   broadcast.Theme,
		Status:  broadcast.Status,
	}

	return ec.broadcastStore.Save(ctx, dbbroadcast)
}

func (ec *broadcastComponentImpl) GetBroadcast(ctx context.Context, id int64) (*types.Broadcast, error) {
	broadcast, err := ec.broadcastStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	var newBroadcast types.Broadcast
	temporaryVariable, _ := json.Marshal(broadcast)
	err = json.Unmarshal(temporaryVariable, &newBroadcast)
	if err != nil {
		return nil, err
	}

	return &newBroadcast, nil
}

func (ec *broadcastComponentImpl) UpdateBroadcast(ctx context.Context, broadcastInput types.Broadcast) (*types.Broadcast, error) {
	broadcast, err := ec.broadcastStore.Get(ctx, broadcastInput.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot find broadcast to update, %w", err)
	}
	broadcast.Content = broadcastInput.Content
	broadcast.BcType = broadcastInput.BcType
	broadcast.Theme = broadcastInput.Theme
	broadcast.Status = broadcastInput.Status
	_, err = ec.broadcastStore.Update(ctx, *broadcast)

	if err != nil {
		return nil, fmt.Errorf("failed to update broadcast, %w", err)
	}

	return &broadcastInput, nil
}

func (ec *broadcastComponentImpl) AllBroadcasts(ctx context.Context) ([]types.Broadcast, error) {
	var broadcasts []types.Broadcast

	dbBroadcasts, err := ec.broadcastStore.FindAll(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find broadcasts, %w", err)
	}

	temporaryVariable, _ := json.Marshal(dbBroadcasts)
	err = json.Unmarshal(temporaryVariable, &broadcasts)
	if err != nil {
		return nil, err
	}
	return broadcasts, nil
}

func (ec *broadcastComponentImpl) ActiveBroadcast(ctx context.Context) (*types.Broadcast, error) {
	var broadcasts []types.Broadcast

	dbBroadcasts, err := ec.broadcastStore.FindAll(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find broadcasts, %w", err)
	}

	var activeBroadcasts []database.Broadcast

	for _, item := range dbBroadcasts {
		if item.Status == "active" {
			activeBroadcasts = append(activeBroadcasts, item)
		}
	}

	temporaryVariable, _ := json.Marshal(activeBroadcasts)
	err = json.Unmarshal(temporaryVariable, &broadcasts)
	if err != nil {
		return nil, err
	}

	if len(broadcasts) > 0 {
		return &broadcasts[0], nil
	} else {
		return nil, nil
	}
}
