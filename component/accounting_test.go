package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountingComponent_ListMeteringsByUserIDAndTime(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)
	ac.userSvcClient = ac.mocks.userSvcClient

	req := types.ActStatementsReq{
		CurrentUser: "user",
		UserUUID:    "uuid",
	}
	ac.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "user").Return(&types.User{
		UUID:  "uuid",
		Roles: []string{},
	}, nil)
	ac.mocks.accountingClient.EXPECT().ListMeteringsByUserIDAndTime(req).Return(
		"", nil,
	)
	resp, err := ac.ListMeteringsByUserIDAndTime(ctx, req)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}
