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

type accountingEventComponentImpl struct {
	ae database.AccountEventStore
}

type AccountingEventComponent interface {
	AddNewAccountingEvent(ctx context.Context, event *types.MeteringEvent, isDuplicated bool) error
}

func NewAccountingEventComponent() AccountingEventComponent {
	aec := &accountingEventComponentImpl{
		ae: database.NewAccountEventStore(),
	}
	return aec
}

func (a *accountingEventComponentImpl) AddNewAccountingEvent(ctx context.Context, event *types.MeteringEvent, isDuplicated bool) error {
	_, err := a.ae.GetByEventID(ctx, event.Uuid)
 if err != nil {
     log.Printf("Error: %v", err)
 }
	if errors.Is(err, sql.ErrNoRows) {
		body := make(map[string]string)
		elem := reflect.ValueOf(event).Elem()
		relType := elem.Type()
		for i := 0; i < relType.NumField(); i++ {
			name := relType.Field(i).Name
			body[name] = fmt.Sprintf("%v", elem.Field(i).Interface())
		}
		input := database.AccountEvent{
			EventUUID:  event.Uuid,
			EventBody:  body,
			Duplicated: isDuplicated,
		}
		return a.ae.Create(ctx, input)
	}

	return err
}
