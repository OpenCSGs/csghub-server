package component

import (
	"context"
	"database/sql"
	"fmt"
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

func (a *AccountingStatementComponent) AddNewStatement(ctx context.Context, event *types.ACC_EVENT, eventExtra *types.ACC_EVENT_EXTRA, changeValue float64) error {
	statement := database.AccountStatement{
		EventUUID:  event.Uuid,
		UserID:     event.UserID,
		Value:      event.Value,
		Scene:      a.getValidSceneType(event.Scene),
		OpUID:      event.OpUID,
		CustomerID: eventExtra.CustomerID,
		EventDate:  event.CreatedAt,
		Price:      eventExtra.CustomerPrice,
		PriceUnit:  eventExtra.PriceUnit,
	}
	if event.Scene == int(database.SceneStarship) {
		// starship token count
		statement.Consumption = event.Value
	} else if event.Scene == int(database.SceneModelInference) || event.Scene == int(database.SceneSpace) || event.Scene == int(database.SceneModelFinetune) {
		// time duration of csghub resource
		statement.Consumption = eventExtra.CustomerDuration
	} else {
		statement.Consumption = 0
	}
	return a.asms.Create(ctx, statement, changeValue)
}

func (a *AccountingStatementComponent) ListStatementByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) ([]types.ACCT_STATEMENTS_RES, int, error) {
	statements, total, err := a.asms.ListByUserIDAndTime(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	var resStatements []types.ACCT_STATEMENTS_RES
	for _, st := range statements {
		resStatements = append(resStatements, types.ACCT_STATEMENTS_RES{
			ID:          st.ID,
			EventUUID:   st.EventUUID,
			UserID:      st.UserID,
			Value:       st.Value,
			Scene:       int(st.Scene),
			OpUID:       st.OpUID,
			CreatedAt:   st.CreatedAt,
			CustomerID:  st.CustomerID,
			EventDate:   st.EventDate,
			Price:       st.Price,
			PriceUnit:   st.PriceUnit,
			Consumption: st.Consumption,
		})
	}

	return resStatements, total, err
}

func (a *AccountingStatementComponent) FindStatementByEventID(ctx context.Context, event *types.ACC_EVENT) (*database.AccountStatement, error) {
	statement, err := a.asms.GetByEventID(ctx, event.Uuid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &statement, err
}

func (a *AccountingStatementComponent) getValidSceneType(scene int) database.SceneType {
	switch scene {
	case 0:
		return database.SceneReserve
	case 1:
		return database.ScenePortalCharge
	case 10:
		return database.SceneModelInference
	case 11:
		return database.SceneSpace
	case 12:
		return database.SceneModelFinetune
	case 20:
		return database.SceneStarship
	default:
		return database.SceneUnknow
	}
}

func (a *AccountingStatementComponent) RechargeAccountingUser(ctx context.Context, userID string, req types.RECHARGE_REQ) error {
	event := types.ACC_EVENT{
		Uuid:      uuid.New(),
		UserID:    userID,
		Value:     req.Value,
		ValueType: 0,
		Scene:     int(database.ScenePortalCharge),
		OpUID:     req.OpUID,
		CreatedAt: time.Now(),
		Extra:     "",
	}
	eventExtra := types.ACC_EVENT_EXTRA{}
	err := a.AddNewStatement(ctx, &event, &eventExtra, event.Value)
	if err != nil {
		return fmt.Errorf("fail to add statement and rechange balance, %v, %w", event, err)
	}
	return err
}
