package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestPaymentStripe_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPaymentStripeStoreWithDB(db)

	crfid := uuid.New().String()

	pay := database.PaymentStripe{
		ClientReferenceID: crfid,
		UserUUID:          "test-user-uuid",
		AmountTotal:       1000,
		Currency:          "usd",
		SessionID:         "test-session-id",
		SessionCreatedAt:  time.Now(),
	}

	res, err := store.Create(ctx, pay)
	require.Nil(t, err)
	require.Equal(t, res.ClientReferenceID, crfid)

	res1, err := store.Get(ctx, res.ID)
	require.Nil(t, err)
	require.Equal(t, res1.ClientReferenceID, crfid)

	res2, err := store.GetBySessionID(ctx, res1.SessionID)
	require.Nil(t, err)
	require.Equal(t, res2.ClientReferenceID, crfid)

	res2.SessionStatus = "completed"
	res2.PaymentStatus = "paid"
	res3, err := store.Update(ctx, *res2)
	require.Nil(t, err)
	require.Equal(t, res3.SessionStatus, "completed")
	require.Equal(t, res3.PaymentStatus, "paid")

	req := &types.StripeSessionListReq{
		Per:           10,
		Page:          1,
		QueryUserUUID: pay.UserUUID,
		StartDate:     "2000-01-01 00:00:00",
		EndDate:       "2099-12-31 23:59:59",
	}

	res4, err := store.List(ctx, req)
	require.Nil(t, err)
	require.Equal(t, len(res4.Data), 1)
	require.Equal(t, res4.TotalAmount, int64(1000))
}
