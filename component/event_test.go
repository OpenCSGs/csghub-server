package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestEventComponent_NewEvent(t *testing.T) {
	ctx := context.TODO()
	ec := initializeTestEventComponent(ctx, t)

	ec.mocks.stores.EventMock().EXPECT().BatchSave(ctx, []database.Event{
		{EventID: "e1"},
	}).Return(nil)

	err := ec.NewEvents(ctx, []types.Event{{ID: "e1"}})
	require.Nil(t, err)
}
