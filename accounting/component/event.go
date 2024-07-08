package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type AccountingEventComponent struct {
	ae *database.AccountEventStore
}

func NewAccountingEvent() *AccountingEventComponent {
	aec := &AccountingEventComponent{
		ae: database.NewAccountEventStore(),
	}
	return aec
}

func (a *AccountingEventComponent) AddNewAccountingEvent(ctx context.Context, event *types.ACCT_EVENT) error {
	_, err := a.ae.GetByEventID(ctx, event.Uuid)
	if errors.Is(err, sql.ErrNoRows) {
		body := make(map[string]string)
		elem := reflect.ValueOf(event).Elem()
		relType := elem.Type()
		for i := 0; i < relType.NumField(); i++ {
			name := relType.Field(i).Name
			body[name] = fmt.Sprintf("%v", elem.Field(i).Interface())
		}
		input := database.AccountEvent{
			EventUUID: event.Uuid,
			EventBody: body,
		}
		return a.ae.Create(ctx, input)
	}

	return err
}
