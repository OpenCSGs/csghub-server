package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type AccountingStatementComponent struct {
	asms *database.AccountStatementStore
}

func NewAccountingStatement() *AccountingStatementComponent {
	asc := &AccountingStatementComponent{
		asms: database.NewAccountStatementStore(),
	}
	return asc
}

func (a *AccountingStatementComponent) AddNewStatement(ctx context.Context, event *types.ACCT_EVENT_REQ) error {
	statement := database.AccountStatement{
		EventUUID:        event.EventUUID,
		UserUUID:         event.UserUUID,
		Value:            event.Value,
		Scene:            event.Scene,
		OpUID:            event.OpUID,
		CustomerID:       event.CustomerID,
		EventDate:        event.RecordedAt,
		Price:            event.Price,
		PriceUnit:        event.PriceUnit,
		Consumption:      event.Consumption,
		ValueType:        event.ValueType,
		ResourceID:       event.ResourceID,
		ResourceName:     event.ResourceName,
		SkuID:            event.SkuID,
		RecordedAt:       event.RecordedAt,
		SkuUnit:          event.SkuUnit,
		SkuUnitType:      event.SkuUnitType,
		SkuPriceCurrency: event.SkuPriceCurrency,
	}
	err := a.asms.Create(ctx, statement)
	if err != nil {
		return fmt.Errorf("fail to save accounting statement, %w", err)
	}
	return nil
}

func (a *AccountingStatementComponent) ListStatementByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (types.ACCT_STATEMENTS_RESULT, error) {
	statements, err := a.asms.ListByUserIDAndTime(ctx, req)
	if err != nil {
		return types.ACCT_STATEMENTS_RESULT{}, fmt.Errorf("fail to list accounting statement by user and time, %w", err)
	}

	var resStatements []types.ACCT_STATEMENTS_RES
	for _, st := range statements.Data {
		resStatements = append(resStatements, types.ACCT_STATEMENTS_RES{
			ID:               st.ID,
			EventUUID:        st.EventUUID,
			UserUUID:         st.UserUUID,
			Value:            st.Value,
			Scene:            int(st.Scene),
			OpUID:            st.OpUID,
			CreatedAt:        st.CreatedAt,
			CustomerID:       st.CustomerID,
			EventDate:        st.EventDate,
			Price:            st.Price,
			PriceUnit:        st.PriceUnit,
			Consumption:      st.Consumption,
			SkuUnit:          st.SkuUnit,
			SkuUnitType:      st.SkuUnitType,
			SkuPriceCurrency: st.SkuPriceCurrency,
		})
	}

	return types.ACCT_STATEMENTS_RESULT{Data: resStatements, ACCT_SUMMARY: statements.ACCT_SUMMARY}, err
}

func (a *AccountingStatementComponent) FindStatementByEventID(ctx context.Context, event *types.ACCT_EVENT) (*database.AccountStatement, error) {
	statement, err := a.asms.GetByEventID(ctx, event.Uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail to find statement by event uuid, %w", err)
	}
	return &statement, nil
}

func (a *AccountingStatementComponent) RechargeAccountingUser(ctx context.Context, userUUID string, req types.RECHARGE_REQ) error {
	event := types.ACCT_EVENT_REQ{
		EventUUID:    uuid.New(),
		UserUUID:     userUUID,
		Value:        req.Value,
		Scene:        types.ScenePortalCharge,
		OpUID:        strconv.Itoa(req.OpUID),
		CustomerID:   "",
		EventDate:    time.Now(),
		Price:        0,
		PriceUnit:    "",
		Consumption:  0,
		ValueType:    0,
		ResourceID:   "",
		ResourceName: "",
		SkuID:        0,
		RecordedAt:   time.Now(),
	}
	err := a.AddNewStatement(ctx, &event)
	if err != nil {
		return fmt.Errorf("fail to add statement and rechange balance, %v, %w", event, err)
	}
	return nil
}
