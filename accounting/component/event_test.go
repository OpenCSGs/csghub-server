package component

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func NewTestAccountingEventComponent(ae database.AccountEventStore) AccountingEventComponent {
	aec := &accountingEventComponentImpl{
		ae: ae,
	}
	return aec
}

func TestEventComponent_AddNewAccountingEvent(t *testing.T) {
	ctx := context.TODO()

	now := time.Now()
	uid := uuid.New()
	event := &types.MeteringEvent{
		Uuid:       uid,
		UserUUID:   "test-user-uuid",
		Scene:      int(types.SceneModelInference),
		Value:      -10,
		ValueType:  0,
		CustomerID: "test-customer-id",
		CreatedAt:  now,
	}

	input := database.AccountEvent{
		EventUUID: uid,
		EventBody: map[string]string{
			"CreatedAt":    fmt.Sprintf("%v", now),
			"CustomerID":   "test-customer-id",
			"Extra":        "",
			"OpUID":        "",
			"ResourceID":   "",
			"ResourceName": "",
			"Scene":        "10",
			"UserUUID":     "test-user-uuid",
			"Uuid":         uid.String(),
			"Value":        "-10",
			"ValueType":    "0",
		},
		Duplicated: false,
	}

	mockEvtStore := mockdb.NewMockAccountEventStore(t)
	mockEvtStore.EXPECT().GetByEventID(ctx, event.Uuid).Return(nil, sql.ErrNoRows)
	mockEvtStore.EXPECT().Create(ctx, input).Return(nil)

	mockEvt := NewTestAccountingEventComponent(mockEvtStore)

	err := mockEvt.AddNewAccountingEvent(ctx, event, false)

	require.Nil(t, err)
}
