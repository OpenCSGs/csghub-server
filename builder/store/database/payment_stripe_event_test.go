package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestPaymentStripeEventStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPaymentStripeEventStoreWithDB(db)

	event := database.PaymentStripeEvent{
		EventID:   "event_1234567890",
		EventType: "charge.succeeded",
		EventBody: `{"id": "event_1234567890", "type": "charge.succeeded"}`,
	}

	res, err := store.Create(ctx, event)
	require.Nil(t, err)
	require.Equal(t, event.EventID, res.EventID)

	res, err = store.Get(ctx, event.EventID)
	require.Nil(t, err)
	require.Equal(t, event.EventID, res.EventID)
	require.Equal(t, event.EventType, res.EventType)
	require.Equal(t, event.EventBody, res.EventBody)

}
