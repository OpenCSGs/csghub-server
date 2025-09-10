package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountingComponent_ListMeteringsByUserIDAndTime(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.ActStatementsReq{
		CurrentUser: "user",
		UserUUID:    "uuid",
	}
	ac.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		UUID: "uuid",
	}, nil)
	ac.mocks.accountingClient.EXPECT().ListMeteringsByUserIDAndTime(req).Return(
		"", nil,
	)
	resp, err := ac.ListMeteringsByUserIDAndTime(ctx, req)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}
