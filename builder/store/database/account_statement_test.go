package database_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountStatementStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	as := database.NewAccountStatementStoreWithDB(db)
	au := database.NewAccountUserStoreWithDB(db)

	err := au.Create(ctx, database.AccountUser{
		UserUUID:    "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Balance:     100,
		CashBalance: 200,
	})
	require.Nil(t, err)

	uid := uuid.New()
	err = as.Create(ctx, database.AccountStatement{
		EventUUID:        uid,
		UserUUID:         "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:            100,
		Scene:            types.SceneCashCharge,
		OpUID:            "1003",
		CustomerID:       "u-opencsg-test-streamlit-6",
		EventDate:        time.Now(),
		Price:            60,
		PriceUnit:        string(types.UnitMinute),
		Consumption:      10,
		ValueType:        0,
		ResourceID:       "629395367163770880",
		ResourceName:     "Streamlit",
		SkuID:            11,
		RecordedAt:       time.Now(),
		SkuUnit:          60,
		SkuUnitType:      types.UnitMinute,
		SkuPriceCurrency: "",
		EventValue:       100,
		IsCancel:         false,
	}, types.AcctStatementExtra{})

	require.Nil(t, err)

	user, err := au.FindUserByID(ctx, "bd05a582-a185-42d7-bf19-ad8108c4523b")
	require.Nil(t, err)
	require.Equal(t, float64(300), user.CashBalance)
}

func TestAccountStatementStore_CreateSimple(t *testing.T) {
	cases := []struct {
		scene           types.SceneType
		charge          bool
		balanceType     string
		balanceValue    float64
		userBalance     float64
		userCashBalance float64
		bill            bool
	}{
		{types.ScenePortalCharge, true, types.ChargeBalance, 70, 70, 30, false},
		{types.SceneCashCharge, true, types.ChargeCashBalance, 50, 50, 50, false},
		{types.SceneMultiSync, false, types.ChargeCashBalance, 10, 50, 10, false},
		{types.SceneSpace, false, types.ChargeCashBalance, 10, 50, 10, true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%v %v", c.scene, c.charge), func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			au := &database.AccountUser{
				UserUUID:    "foo",
				Balance:     50,
				CashBalance: 30,
			}
			_, err := db.Core.NewInsert().Model(au).Exec(ctx)
			require.Nil(t, err)

			// SceneSpace triggers voucher deduction which requires a valid ResourceID and SpaceResource with XPU model
			if c.scene == types.SceneSpace {
				sr := &database.SpaceResource{
					Name:      "test-resource",
					Resources: `{"gpu": {"num": "1", "type": "A10"}}`,
					ClusterID: "test-cluster",
				}
				_, err = db.Core.NewInsert().Model(sr).Exec(ctx, sr)
				require.Nil(t, err)
			}

			store := database.NewAccountStatementStoreWithDB(db)

			var v float64 = 20
			if !c.charge {
				v = -20
			}
			input := database.AccountStatement{
				EventUUID: uuid.New(),
				UserUUID:  "foo",
				Scene:     c.scene,
				Value:     v,
			}
			if c.scene == types.SceneSpace {
				input.ResourceID = "1"
			}
			err = store.Create(ctx, input, types.AcctStatementExtra{})
			require.Nil(t, err)

			as := &database.AccountStatement{}
			err = db.Core.NewSelect().Model(as).Where("user_uuid=?", "foo").Scan(ctx)
			require.Nil(t, err)
			require.Equal(t, c.balanceType, as.BalanceType)
			require.Equal(t, c.balanceValue, as.BalanceValue)

			acctUser := &database.AccountUser{}
			err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "foo").Scan(ctx)
			require.Nil(t, err)
			require.Equal(t, c.userBalance, acctUser.Balance)
			require.Equal(t, c.userCashBalance, acctUser.CashBalance)

			if c.bill {
				bill := &database.AccountBill{}
				err = db.Core.NewSelect().Model(bill).Where("user_uuid=?", "foo").Scan(ctx)
				require.Nil(t, err)
				require.Equal(t, c.scene, bill.Scene)
				require.Equal(t, v, bill.Value)
			}

		})
	}
}

func TestAccountStatementStore_Create_BillUpsertReasoningToken(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	au := &database.AccountUser{
		UserUUID:    "foo-bill-reasoning",
		Balance:     100,
		CashBalance: 100,
	}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.Nil(t, err)

	// SceneSpace triggers voucher deduction which requires a valid ResourceID and SpaceResource with XPU model
	sr := &database.SpaceResource{
		Name:      "test-resource",
		Resources: `{"gpu": {"num": "1", "type": "A10"}}`,
		ClusterID: "test-cluster",
	}
	_, err = db.Core.NewInsert().Model(sr).Exec(ctx, sr)
	require.Nil(t, err)

	store := database.NewAccountStatementStoreWithDB(db)
	eventDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	for _, reasoning := range []float64{30, 20} {
		err = store.Create(ctx, database.AccountStatement{
			EventUUID:       uuid.New(),
			UserUUID:        "foo-bill-reasoning",
			Scene:           types.SceneSpace,
			Value:           -10,
			EventDate:       eventDate,
			Consumption:     100,
			PromptToken:     100,
			CompletionToken: 50,
			ReasoningToken:  reasoning,
			ResourceID:      strconv.FormatInt(sr.ID, 10),
			CustomerID:      "svc-reasoning",
		}, types.AcctStatementExtra{})
		require.Nil(t, err)
	}

	bill := &database.AccountBill{}
	err = db.Core.NewSelect().Model(bill).Where("user_uuid=?", "foo-bill-reasoning").Scan(ctx)
	require.Nil(t, err)
	// SceneSpace resolves no token plan, so the deduct path bills the whole
	// token detail on the single cash row: both statements upsert into it and
	// their detail adds up instead of being dropped.
	require.Equal(t, float64(200), bill.PromptToken)
	require.Equal(t, float64(100), bill.CompletionToken)
	require.Equal(t, float64(50), bill.ReasoningToken)

	// the statement rows keep the full usage detail
	var statements []database.AccountStatement
	err = db.Core.NewSelect().Model(&statements).Where("user_uuid = ?", "foo-bill-reasoning").Scan(ctx)
	require.NoError(t, err)
	require.Len(t, statements, 2)
	var prompt, completion, reasoning float64
	for _, stmt := range statements {
		prompt += stmt.PromptToken
		completion += stmt.CompletionToken
		reasoning += stmt.ReasoningToken
	}
	require.Equal(t, float64(200), prompt)
	require.Equal(t, float64(100), completion)
	require.Equal(t, float64(50), reasoning)
}

func TestAccountStatementStore_DeductAccountFee(t *testing.T) {

	cases := []struct {
		name                string
		remain              float64
		userBalance         float64
		userCashBalance     float64
		expectedUserBalance float64
		expectedCashBalance float64
	}{
		{"remain zero", 0, -20, -10, -20, -10},
		{"user balance enough", -10, -10, 10, -10, 0},
		{"user balance and cash balance enough", -10, 5, 5, 0, 0},
		{"not enough", -10, 4, 4, 0, -2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				au := database.AccountUser{
					UserUUID:    "foo",
					Balance:     c.userBalance,
					CashBalance: c.userCashBalance,
				}
				_, err := tx.NewInsert().Model(&au).Exec(ctx)
				require.Nil(t, err)

				_, err = database.DeductAccountFee(ctx, tx, database.AccountStatement{
					Value:    c.remain,
					UserUUID: "foo",
				}, false)
				require.Nil(t, err)

				acctUser := &database.AccountUser{}
				err = tx.NewSelect().Model(acctUser).Where("user_uuid = ?", "foo").Scan(ctx)
				require.Nil(t, err)
				require.Equal(t, c.expectedUserBalance, acctUser.Balance)
				require.Equal(t, c.expectedCashBalance, acctUser.CashBalance)
				return nil
			})
			require.Nil(t, err)

		})
	}

}

func TestAccountStatementStore_DeductAccountFeeParallel(t *testing.T) {

	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.TODO()

	au := database.AccountUser{
		UserUUID:    "foo",
		Balance:     50,
		CashBalance: 50,
	}
	_, err := db.Core.NewInsert().Model(&au).Exec(ctx)
	require.Nil(t, err)

	var wg sync.WaitGroup

	// total 110
	for _, cost := range []float64{2, 4, 6, 8, 10, 12, 14, 16, 18, 20} {
		wg.Add(1)
		go func(cost float64) {
			defer wg.Done()
			tx, err := db.Core.BeginTx(ctx, nil)
			require.Nil(t, err)

			_, err = database.DeductAccountFee(ctx, tx, database.AccountStatement{
				Value:     -cost,
				UserUUID:  "foo",
				EventUUID: uuid.New(),
			}, false)
			require.Nil(t, err)
			err = tx.Commit()
			require.Nil(t, err)
		}(cost)
	}
	wg.Wait()

	acctUser := &database.AccountUser{}
	err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "foo").Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, float64(0), acctUser.Balance)
	require.Equal(t, float64(-10), acctUser.CashBalance)

}

// seedTokenPlanPeriod creates the billing period row that a statement's
// TokenPlanPeriodID points at. Deduct accumulates the covered cost onto that row
// and fails when the update matches nothing, so every period a test references
// must exist even when the test never reads the period itself.
func seedTokenPlanPeriod(ctx context.Context, t *testing.T, db *database.DB, periodID int64) {
	t.Helper()

	period := database.AccountTokenPlanInstancePeriod{
		ID:            periodID,
		InstanceID:    periodID + 1_000_000,
		PlanVersionID: 1,
		PeriodNo:      1,
		StartAt:       time.Now().Add(-time.Hour).Unix(),
		EndAt:         time.Now().Add(time.Hour).Unix(),
		Amount:        0,
	}
	_, err := db.Core.NewInsert().Model(&period).Exec(ctx, &period)
	require.NoError(t, err)
}

func TestAccountStatementStore_Create_TokenPlanQuotaCharge(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	// Every billing period this test references must exist: deduct
	// accumulates the covered cost onto the period row and fails when the
	// update matches no row.
	seedTokenPlanPeriod(ctx, t, db, 101)
	seedTokenPlanPeriod(ctx, t, db, 102)
	seedTokenPlanPeriod(ctx, t, db, 103)
	seedTokenPlanPeriod(ctx, t, db, 104)
	seedTokenPlanPeriod(ctx, t, db, 105)
	seedTokenPlanPeriod(ctx, t, db, 106)
	seedTokenPlanPeriod(ctx, t, db, 107)
	seedTokenPlanPeriod(ctx, t, db, 108)
	seedTokenPlanPeriod(ctx, t, db, 109)
	seedTokenPlanPeriod(ctx, t, db, 110)

	store := database.NewAccountStatementStoreWithDB(db)

	const tokenPlanStatementCost = 25

	cases := []struct {
		name        string
		scene       types.SceneType
		valueType   types.ChargeValueType
		periodID    int64
		modelID     string // statement CustomerID: the gateway model id
		consumption float64
		fee         float64
		seed        *database.AccountTokenPlanPeriodModelQuota
		wantValue   float64
		wantTokens  int64
		wantReqs    int64
		wantUsedAt  bool
		// plan-absorbed fee and consumption expected on the period-attributed
		// bill row: non-zero only for a partial coverage, which splits into a
		// cash row (period_id 0) plus a plan row (period_id = periodID)
		wantPlanValue       float64
		wantPlanConsumption float64
		wantPeriodCost      float64
	}{
		{
			name: "token quota fully covers consumption", periodID: 101,
			consumption: 500, fee: -10,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 101, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 1000},
			wantValue:  0,
			wantTokens: 500, wantReqs: 1, wantUsedAt: true,
			wantPeriodCost: tokenPlanStatementCost,
		},
		{
			name: "partial coverage charges the uncovered share", periodID: 102,
			consumption: 500, fee: -10,
			// free remaining 200 of 500 consumed: charge 300/500 = 60% of the
			// fee and record only the covered 200 tokens
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 102, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 1000, UsedTokens: 800},
			wantValue:  -6,
			wantTokens: 1000, wantReqs: 1, wantUsedAt: true,
			// the plan absorbs 200 of 500 tokens: -4 of the fee bills on the
			// period row, the uncovered -6/300 on the default-view cash row
			wantPlanValue: -4, wantPlanConsumption: 200,
			wantPeriodCost: tokenPlanStatementCost * 0.4,
		},
		{
			// an exhausted quota covers nothing and records nothing, not
			// even the request count; only LatestUsedAt moves
			name: "exhausted quota charges full pay-as-you-go", periodID: 103,
			consumption: 500, fee: -10,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 103, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 1000, UsedTokens: 1000},
			wantValue:  -10,
			wantTokens: 1000, wantReqs: 0, wantUsedAt: true,
		},
		{
			name: "non-positive token allowance means unlimited free usage", periodID: 104,
			consumption: 500, fee: -10,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 104, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 0},
			wantValue:  0,
			wantTokens: 500, wantReqs: 1, wantUsedAt: true,
			wantPeriodCost: tokenPlanStatementCost,
		},
		{
			// unlimited count dimension: the fee is covered and the metered
			// consumption is still recorded as token usage
			name: "non-positive request-count allowance means unlimited free usage", periodID: 110,
			scene: types.SceneMultiModalServerless, valueType: types.CountNumberType,
			consumption: 4, fee: -8,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 110, ModelID: "model-a", TaskType: string(types.Text2Video), QuotaRequestCount: 0},
			wantValue:  0,
			wantTokens: 4, wantReqs: 1, wantUsedAt: true,
			wantPeriodCost: tokenPlanStatementCost,
		},
		{
			name: "count-metered multimodal consumes one request unit", periodID: 105,
			scene: types.SceneMultiModalServerless, valueType: types.CountNumberType,
			consumption: 4, fee: -8,
			// the request-count dimension is binary: 2 free requests remain, so
			// the whole fee is covered and the statement consumes one request
			// unit; coverage ignores the token quota, but the metered
			// consumption (4) is still recorded as token usage
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 105, ModelID: "model-a", TaskType: string(types.Text2Video), QuotaTokens: 1000, QuotaRequestCount: 10, UsedRequestCount: 8},
			wantValue:  0,
			wantTokens: 4, wantReqs: 9, wantUsedAt: true,
			wantPeriodCost: tokenPlanStatementCost,
		},
		{
			// exhausted count quota: full fee, no usage recorded at all
			name: "exhausted request-count allowance charges full pay-as-you-go", periodID: 108,
			scene: types.SceneMultiModalServerless, valueType: types.CountNumberType,
			consumption: 4, fee: -8,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 108, ModelID: "model-a", TaskType: string(types.Text2Video), QuotaRequestCount: 10, UsedRequestCount: 10},
			wantValue:  -8,
			wantTokens: 0, wantReqs: 10, wantUsedAt: true,
		},
		{
			// the instance defined no model quota for this period at all: the
			// plan is unrestricted and every call inside the period is covered
			name: "period without any model quota covers all calls", periodID: 109,
			consumption: 500, fee: -10,
			wantValue:  0,
			wantTokens: 0, wantReqs: 0, wantUsedAt: false,
			wantPeriodCost: tokenPlanStatementCost,
		},
		{
			name: "model outside the plan quota stays pay-as-you-go", periodID: 106,
			modelID: "model-other", consumption: 500, fee: -10,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 106, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 1000},
			wantValue:  -10,
			wantTokens: 0, wantReqs: 0, wantUsedAt: false,
		},
		{
			name: "statement without a resolved period stays pay-as-you-go", periodID: 0,
			consumption: 500, fee: -10,
			seed:       &database.AccountTokenPlanPeriodModelQuota{PeriodID: 107, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 1000},
			wantValue:  -10,
			wantTokens: 0, wantReqs: 0, wantUsedAt: false,
		},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			scene := c.scene
			if scene == 0 {
				scene = types.SceneModelServerless
			}
			valueType := c.valueType
			if valueType == 0 {
				valueType = types.TokenNumberType
			}
			modelID := c.modelID
			if modelID == "" {
				modelID = "model-a"
			}
			ns := fmt.Sprintf("foo-tokenplan-%d-%s", i, uuid.NewString())

			au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
			_, err := db.Core.NewInsert().Model(au).Exec(ctx)
			require.NoError(t, err)

			if c.seed != nil {
				_, err = db.Core.NewInsert().Model(c.seed).Exec(ctx)
				require.NoError(t, err)
			}

			eventUUID := uuid.New()
			err = store.Create(ctx, database.AccountStatement{
				EventUUID:         eventUUID,
				UserUUID:          ns,
				Scene:             scene,
				Value:             c.fee,
				EventDate:         time.Now(),
				Consumption:       c.consumption,
				ValueType:         valueType,
				CustomerID:        modelID,
				TokenPlanPeriodID: c.periodID,
				TokenID:           7,
				CostAmount:        tokenPlanStatementCost,
				RecordedAt:        time.Now(),
			}, types.AcctStatementExtra{})
			require.NoError(t, err)

			// the statement carries the plan-adjusted fee and the full usage
			var stored database.AccountStatement
			err = db.Core.NewSelect().Model(&stored).Where("event_uuid = ?", eventUUID).Scan(ctx)
			require.NoError(t, err)
			require.InDelta(t, c.wantValue, stored.Value, 1e-9)
			require.InDelta(t, c.consumption, stored.Consumption, 1e-9)

			var acctUser database.AccountUser
			err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
			require.NoError(t, err)
			require.InDelta(t, 100+c.wantValue, acctUser.CashBalance, 1e-9)
			require.Equal(t, float64(0), acctUser.Balance)

			// the charged fee always lands on a default-view cash row
			// (period_id 0) carrying the uncovered consumption share; a
			// partial coverage additionally bills the plan-absorbed fee and
			// the covered consumption share on a row under its period. A
			// fully covered statement has no cash fee, so its plan-absorbed
			// fee bills alone on the period-attributed plan row
			var bills []database.AccountBill
			err = db.Core.NewSelect().Model(&bills).Where("user_uuid = ?", ns).Scan(ctx)
			require.NoError(t, err)
			if c.wantPlanValue != 0 {
				require.Len(t, bills, 2)
				var cash, plan *database.AccountBill
				for i := range bills {
					if bills[i].PeriodID == 0 {
						cash = &bills[i]
					} else {
						plan = &bills[i]
					}
				}
				require.NotNil(t, cash)
				require.NotNil(t, plan)
				require.InDelta(t, c.wantValue, cash.Value, 1e-9)
				require.InDelta(t, c.consumption-c.wantPlanConsumption, cash.Consumption, 1e-9)
				require.InDelta(t, c.wantPlanValue, plan.Value, 1e-9)
				require.InDelta(t, c.wantPlanConsumption, plan.Consumption, 1e-9)
				require.Equal(t, c.periodID, plan.PeriodID)
			} else if c.wantValue == 0 {
				// fully covered: no cash fee, so the plan-absorbed fee bills
				// alone under the resolved period
				require.Len(t, bills, 1)
				require.InDelta(t, c.fee, bills[0].Value, 1e-9)
				require.InDelta(t, c.consumption, bills[0].Consumption, 1e-9)
				require.Equal(t, c.periodID, bills[0].PeriodID)
			} else {
				require.Len(t, bills, 1)
				require.InDelta(t, c.wantValue, bills[0].Value, 1e-9)
				require.InDelta(t, c.consumption, bills[0].Consumption, 1e-9)
				require.Zero(t, bills[0].PeriodID)
			}

			if c.seed != nil {
				var quota database.AccountTokenPlanPeriodModelQuota
				err = db.Core.NewSelect().Model(&quota).Where("id = ?", c.seed.ID).Scan(ctx)
				require.NoError(t, err)
				require.Equal(t, c.wantTokens, quota.UsedTokens)
				require.Equal(t, c.wantReqs, quota.UsedRequestCount)
				if c.wantUsedAt {
					require.Positive(t, quota.LatestUsedAt)
				} else {
					require.Zero(t, quota.LatestUsedAt)
				}
			}

			if c.periodID > 0 {
				var period database.AccountTokenPlanInstancePeriod
				err = db.Core.NewSelect().Model(&period).Where("id = ?", c.periodID).Scan(ctx)
				require.NoError(t, err)
				require.InDelta(t, c.wantPeriodCost, period.CostAmount, 1e-9)
			}
		})
	}

	t.Run("zero covered cost does not update the period row", func(t *testing.T) {
		oldUpdatedAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		cases := []struct {
			name     string
			periodID int64
			modelID  string
		}{
			{name: "exhausted quota", periodID: 112, modelID: "model-a"},
			{name: "model outside quota table", periodID: 113, modelID: "model-other"},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				seedTokenPlanPeriod(ctx, t, db, c.periodID)
				_, err := db.Core.NewUpdate().Model((*database.AccountTokenPlanInstancePeriod)(nil)).
					Set("updated_at = ?", oldUpdatedAt).
					Where("id = ?", c.periodID).
					Exec(ctx)
				require.NoError(t, err)

				_, err = db.Core.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
					PeriodID: c.periodID, ModelID: "model-a",
					TaskType:    string(types.TextGeneration),
					QuotaTokens: 1000, UsedTokens: 1000,
				}).Exec(ctx)
				require.NoError(t, err)

				ns := "foo-tokenplan-zero-cost-" + uuid.NewString()
				_, err = db.Core.NewInsert().Model(&database.AccountUser{
					UserUUID: ns, Balance: 0, CashBalance: 100,
				}).Exec(ctx)
				require.NoError(t, err)

				err = store.Create(ctx, database.AccountStatement{
					EventUUID:         uuid.New(),
					UserUUID:          ns,
					Scene:             types.SceneModelServerless,
					Value:             -10,
					EventDate:         time.Now(),
					Consumption:       500,
					ValueType:         types.TokenNumberType,
					CustomerID:        c.modelID,
					TokenPlanPeriodID: c.periodID,
					TokenID:           7,
					CostAmount:        25,
					RecordedAt:        time.Now(),
				}, types.AcctStatementExtra{})
				require.NoError(t, err)

				var period database.AccountTokenPlanInstancePeriod
				err = db.Core.NewSelect().Model(&period).Where("id = ?", c.periodID).Scan(ctx)
				require.NoError(t, err)
				require.Zero(t, period.CostAmount)
				require.True(t, period.UpdatedAt.Equal(oldUpdatedAt))
			})
		}
	})

	t.Run("cost accumulates across statements on the same period", func(t *testing.T) {
		const periodID int64 = 111
		seedTokenPlanPeriod(ctx, t, db, periodID)

		ns := "foo-tokenplan-accumulate-" + uuid.NewString()
		_, err := db.Core.NewInsert().Model(&database.AccountUser{
			UserUUID: ns, Balance: 0, CashBalance: 100,
		}).Exec(ctx)
		require.NoError(t, err)

		_, err = db.Core.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
			PeriodID: periodID, ModelID: "model-a",
			TaskType: string(types.TextGeneration), QuotaTokens: 1000,
		}).Exec(ctx)
		require.NoError(t, err)

		for _, cost := range []float64{10, 15} {
			err = store.Create(ctx, database.AccountStatement{
				EventUUID:         uuid.New(),
				UserUUID:          ns,
				Scene:             types.SceneModelServerless,
				Value:             -10,
				EventDate:         time.Now(),
				Consumption:       100,
				ValueType:         types.TokenNumberType,
				CustomerID:        "model-a",
				TokenPlanPeriodID: periodID,
				TokenID:           7,
				CostAmount:        cost,
				RecordedAt:        time.Now(),
			}, types.AcctStatementExtra{})
			require.NoError(t, err)
		}

		var period database.AccountTokenPlanInstancePeriod
		err = db.Core.NewSelect().Model(&period).Where("id = ?", periodID).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 25.0, period.CostAmount, 1e-9)
	})
}

// TestAccountStatementStore_Create_TokenPlanMatchesResourceIDModel verifies
// that the quota key comes from the gateway resource id's model segment rather
// than the statement's display CustomerID, which is the public gateway model ID.
func TestAccountStatementStore_Create_TokenPlanMatchesResourceIDModel(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewAccountStatementStoreWithDB(db)
	ns := "foo-tokenplan-resource-" + uuid.NewString()
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: ns, Balance: 0, CashBalance: 100,
	}).Exec(ctx)
	require.NoError(t, err)
	seedTokenPlanPeriod(ctx, t, db, 120)

	quota := &database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 120, ModelID: "model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000,
	}
	_, err = db.Core.NewInsert().Model(quota).Exec(ctx)
	require.NoError(t, err)

	eventUUID := uuid.New()
	err = store.Create(ctx, database.AccountStatement{
		EventUUID:         eventUUID,
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		CustomerID:        "model-a:2f",
		ResourceID:        "csghub://serverless/model-a",
		TokenPlanPeriodID: 120,
		TokenID:           7,
		RecordedAt:        time.Now(),
	}, types.AcctStatementExtra{})
	require.NoError(t, err)

	var storedQuota database.AccountTokenPlanPeriodModelQuota
	err = db.Core.NewSelect().Model(&storedQuota).Where("id = ?", quota.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(500), storedQuota.UsedTokens)
	require.Equal(t, int64(1), storedQuota.UsedRequestCount)

	// The per-api-key mirror uses the same resource-derived model key.
	var keyQuota database.AccountTokenPlanPeriodKeyQuota
	err = db.Core.NewSelect().Model(&keyQuota).
		Where("period_id = ?", 120).
		Where("token_id = ?", 7).
		Where("model_id = ?", "model-a").
		Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(500), keyQuota.UsedTokens)
	require.Equal(t, int64(1), keyQuota.UsedRequestCount)

	// The display CustomerID and the resolved period stay on the statement.
	var stored database.AccountStatement
	err = db.Core.NewSelect().Model(&stored).Where("event_uuid = ?", eventUUID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, "model-a:2f", stored.CustomerID)
	require.Equal(t, int64(120), stored.TokenPlanPeriodID)
	require.InDelta(t, 0, stored.Value, 1e-9)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, 100, acctUser.CashBalance, 1e-9)
}

// TestAccountStatementStore_Create_TokenPlanIncompleteResourceIDFallsBackToCustomerID
// verifies that an incomplete csghub resource id does not resolve a model key:
// csghub:///ns/model has no type segment, so quota matching must fall back to
// CustomerID instead of consuming the quota keyed by the repo path carried in
// the malformed id.
func TestAccountStatementStore_Create_TokenPlanIncompleteResourceIDFallsBackToCustomerID(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewAccountStatementStoreWithDB(db)
	ns := "foo-tokenplan-incomplete-resource-" + uuid.NewString()
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: ns, Balance: 0, CashBalance: 100,
	}).Exec(ctx)
	require.NoError(t, err)
	seedTokenPlanPeriod(ctx, t, db, 121)

	// The repo-path quota would be matched only if the empty type segment were
	// parsed away from csghub:///ns/model. The CustomerID quota is the fallback
	// key and must be the one consumed.
	repoPathQuota := &database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 121, ModelID: "ns/model", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000,
	}
	_, err = db.Core.NewInsert().Model(repoPathQuota).Exec(ctx)
	require.NoError(t, err)
	customerIDQuota := &database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 121, ModelID: "svc-model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000,
	}
	_, err = db.Core.NewInsert().Model(customerIDQuota).Exec(ctx)
	require.NoError(t, err)

	eventUUID := uuid.New()
	err = store.Create(ctx, database.AccountStatement{
		EventUUID:         eventUUID,
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		CustomerID:        "svc-model-a",
		ResourceID:        "csghub:///ns/model",
		TokenPlanPeriodID: 121,
		TokenID:           7,
		RecordedAt:        time.Now(),
	}, types.AcctStatementExtra{})
	require.NoError(t, err)

	// The malformed resource id yielded no model key: the repo-path quota
	// stays untouched and the fallback CustomerID quota covers the call.
	var storedRepoPathQuota database.AccountTokenPlanPeriodModelQuota
	err = db.Core.NewSelect().Model(&storedRepoPathQuota).Where("id = ?", repoPathQuota.ID).Scan(ctx)
	require.NoError(t, err)
	require.Zero(t, storedRepoPathQuota.UsedTokens)
	require.Zero(t, storedRepoPathQuota.UsedRequestCount)

	var storedCustomerIDQuota database.AccountTokenPlanPeriodModelQuota
	err = db.Core.NewSelect().Model(&storedCustomerIDQuota).Where("id = ?", customerIDQuota.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(500), storedCustomerIDQuota.UsedTokens)
	require.Equal(t, int64(1), storedCustomerIDQuota.UsedRequestCount)

	// The per-key mirror uses the same fallback key, never the malformed one.
	var keyQuota database.AccountTokenPlanPeriodKeyQuota
	err = db.Core.NewSelect().Model(&keyQuota).
		Where("period_id = ?", 121).
		Where("token_id = ?", 7).
		Where("model_id = ?", "svc-model-a").
		Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(500), keyQuota.UsedTokens)
	require.Equal(t, int64(1), keyQuota.UsedRequestCount)

	malformedKeyCount, err := db.Core.NewSelect().Model(&database.AccountTokenPlanPeriodKeyQuota{}).
		Where("period_id = ?", 121).
		Where("token_id = ?", 7).
		Where("model_id = ?", "ns/model").
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, malformedKeyCount)

	// The fallback quota covers the whole fee.
	var stored database.AccountStatement
	err = db.Core.NewSelect().Model(&stored).Where("event_uuid = ?", eventUUID).Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, 0, stored.Value, 1e-9)
	require.Equal(t, "svc-model-a", stored.CustomerID)
	require.Equal(t, int64(121), stored.TokenPlanPeriodID)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, 100, acctUser.CashBalance, 1e-9)
}

// TestAccountStatementStore_Create_TokenPlanPayAsYouGoBillStaysInDefaultView
// is the query-side regression for the bill period-attribution review
// findings:
//   - a statement that resolved a token plan period but got no deduction
//     (case 3: model outside the period quota; case 8: quota exhausted)
//     charges the full pay-as-you-go fee and must bill on period_id 0, so
//     the default bills view (period_id = 0) and its totals keep showing
//     the real cash spend;
//   - a partially covered statement (case 6) splits into a default-view
//     cash row carrying the uncovered fee, consumption and token detail
//     share, and a period-attributed plan row carrying the plan-absorbed
//     fee plus the covered consumption and token detail share; the two rows
//     add back to the statement's full detail.
//
// Statement rows keep the resolved period for attribution in all cases.
func TestAccountStatementStore_Create_TokenPlanPayAsYouGoBillStaysInDefaultView(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	// Every billing period this test references must exist: deduct
	// accumulates the covered cost onto the period row and fails when the
	// update matches no row.
	seedTokenPlanPeriod(ctx, t, db, 301)
	seedTokenPlanPeriod(ctx, t, db, 302)

	ns := "foo-tokenbill-" + uuid.NewString()
	au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.NoError(t, err)

	// period 301: the model-a quota exists but is exhausted (case 8)
	_, err = db.Core.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 301, ModelID: "model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000, UsedTokens: 1000,
	}).Exec(ctx)
	require.NoError(t, err)
	// period 302: the model-a quota has 200 of 1000 tokens free (case 6),
	// and no quota row exists for model-other (case 3)
	_, err = db.Core.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 302, ModelID: "model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000, UsedTokens: 800,
	}).Exec(ctx)
	require.NoError(t, err)

	store := database.NewAccountStatementStoreWithDB(db)
	deduct := func(periodID int64, modelID string, fee float64) {
		require.NoError(t, store.Create(ctx, database.AccountStatement{
			EventUUID:         uuid.New(),
			UserUUID:          ns,
			Scene:             types.SceneModelServerless,
			Value:             fee,
			EventDate:         time.Now(),
			Consumption:       500,
			ValueType:         types.TokenNumberType,
			CustomerID:        modelID,
			TokenPlanPeriodID: periodID,
			TokenID:           7,
			RecordedAt:        time.Now(),
		}, types.AcctStatementExtra{}))
	}

	deduct(301, "model-a", -10)     // case 8: exhausted quota, full cash fee
	deduct(302, "model-other", -20) // case 3: model outside the quota, full cash fee
	// case 6 control: the plan covers 200 of 500 tokens -> a plan row of -4
	// on period 302 and a cash row of -6 on the default view; the token
	// detail splits 3/5 (uncovered) to the cash row and 2/5 to the plan row
	require.NoError(t, store.Create(ctx, database.AccountStatement{
		EventUUID:         uuid.New(),
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		CustomerID:        "model-a",
		TokenPlanPeriodID: 302,
		TokenID:           7,
		PromptToken:       300,
		CompletionToken:   200,
		RecordedAt:        time.Now(),
	}, types.AcctStatementExtra{}))

	now := time.Now()
	start := now.Add(-24 * time.Hour).Format(time.RFC3339)
	end := now.Add(24 * time.Hour).Format(time.RFC3339)

	billStore := database.NewAccountBillStoreWithDB(db)
	list := func(periodID int64) database.AccountBillRes {
		res, err := billStore.ListByUserIDAndDate(ctx, types.AcctBillsReq{
			TargetUUID: ns,
			Scene:      types.SceneModelServerless,
			StartDate:  start,
			EndDate:    end,
			PeriodID:   periodID,
			Per:        20,
			Page:       1,
		})
		require.NoError(t, err)
		return res
	}

	// default view (period_id = 0): every real cash spend is visible — the
	// case-8 fee and the case-6 uncovered share aggregate into one model-a
	// row (same bill dimensions and period), and the case-6 plan row is
	// excluded
	def := list(0)
	require.Len(t, def.Data, 2)
	require.InDelta(t, -36, def.TotalValue, 1e-9)
	require.InDelta(t, -36, def.TotalCashValue, 1e-9)
	byModel := map[string]float64{}
	for _, item := range def.Data {
		byModel[item.InstanceName] = item.Value
	}
	require.InDelta(t, -16, byModel["model-a"], 1e-9)
	require.InDelta(t, -20, byModel["model-other"], 1e-9)

	// the exhausted period 301 has no bill attributed to it
	require.Empty(t, list(301).Data)

	// period 302 carries only the plan-absorbed share
	p302 := list(302)
	require.Len(t, p302.Data, 1)
	require.Equal(t, "model-a", p302.Data[0].InstanceName)
	require.InDelta(t, -4, p302.Data[0].Value, 1e-9)
	require.InDelta(t, 200, p302.Data[0].Consumption, 1e-9)

	// period_id = -1 disables the filter: the rows sum back to the full
	// pre-split fees
	all := list(-1)
	require.Len(t, all.Data, 2)
	require.InDelta(t, -40, all.TotalValue, 1e-9)

	// the bills detail query applies the same sentinel semantics
	detail := func(periodID int64) database.AccountBillDetailRes {
		res, err := billStore.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: ns,
			Scene:      int(types.SceneModelServerless),
			StartDate:  start,
			EndDate:    end,
			PeriodID:   periodID,
			// -1 disables the detail query's token filter: the statements are
			// keyed (token_id 7), while a zero value would filter token_id = 0
			TokenID: -1,
			Per:     20,
			Page:    1,
		})
		require.NoError(t, err)
		return res
	}
	d0 := detail(0)
	require.Equal(t, 2, d0.Total)
	for _, row := range d0.Data {
		if row.CustomerID == "model-a" {
			// the case-8 fee and the case-6 cash share aggregated into one row
			require.InDelta(t, -16, row.Value, 1e-9)
			require.InDelta(t, 800, row.Consumption, 1e-9)
			// the case-6 cash row carries the uncovered 3/5 of the detail
			require.InDelta(t, 180, row.PromptToken, 1e-9)
			require.InDelta(t, 120, row.CompletionToken, 1e-9)
			require.InDelta(t, 2, row.Count, 1e-9)
		}
	}
	require.Equal(t, 0, detail(301).Total)
	d302 := detail(302)
	require.Equal(t, 1, d302.Total)
	require.InDelta(t, -4, d302.Data[0].Value, 1e-9)
	require.InDelta(t, 200, d302.Data[0].Consumption, 1e-9)
	// the plan row bills the covered 2/5 of the token detail
	require.InDelta(t, 120, d302.Data[0].PromptToken, 1e-9)
	require.InDelta(t, 80, d302.Data[0].CompletionToken, 1e-9)
	require.Equal(t, 3, detail(-1).Total)

	// the statement keeps the resolved period for attribution even when
	// the bill does not carry it
	var stmt database.AccountStatement
	err = db.Core.NewSelect().Model(&stmt).
		Where("user_uuid = ?", ns).
		Where("token_plan_period_id = ?", 301).
		Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, -10, stmt.Value, 1e-9)
}

// TestAccountStatementStore_Create_TokenPlanSplitsTokenDetail verifies that a
// token plan coverage splits the statement's usage detail — the token columns
// and the duration — between the two bill rows instead of duplicating it: the
// cash row carries the uncovered fraction, the plan row the covered one, and the
// two shares add back to the statement's full detail. A statement the plan never
// covered keeps its whole detail on the single cash row: an exhausted quota, a
// model outside the period's quotas, and a statement that resolved no period at
// all bill as pure pay-as-you-go.
func TestAccountStatementStore_Create_TokenPlanSplitsTokenDetail(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	// Every billing period this test references must exist: deduct
	// accumulates the covered cost onto the period row and fails when the
	// update matches no row.
	seedTokenPlanPeriod(ctx, t, db, 510)
	seedTokenPlanPeriod(ctx, t, db, 511)
	seedTokenPlanPeriod(ctx, t, db, 512)
	seedTokenPlanPeriod(ctx, t, db, 513)
	seedTokenPlanPeriod(ctx, t, db, 514)

	store := database.NewAccountStatementStoreWithDB(db)

	seed := func(t *testing.T, ns string, quota *database.AccountTokenPlanPeriodModelQuota) {
		t.Helper()
		_, err := db.Core.NewInsert().
			Model(&database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}).
			Exec(ctx)
		require.NoError(t, err)
		_, err = db.Core.NewInsert().Model(quota).Exec(ctx)
		require.NoError(t, err)
	}

	deduct := func(t *testing.T, ns string, periodID int64, modelID string) {
		t.Helper()
		err := store.Create(ctx, database.AccountStatement{
			EventUUID:         uuid.New(),
			UserUUID:          ns,
			Scene:             types.SceneModelServerless,
			Value:             -10,
			EventDate:         time.Now(),
			Consumption:       500,
			ValueType:         types.TokenNumberType,
			CustomerID:        modelID,
			TokenPlanPeriodID: periodID,
			TokenID:           7,
			PromptToken:       300,
			PromptCachedToken: 40,
			CompletionToken:   200,
			ReasoningToken:    60,
			Duration:          12,
			RecordedAt:        time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)
	}

	// the statement's full usage detail — every token column plus the duration
	// — as deduct wrote it: asserted on the single cash row of every statement
	// the plan did not cover
	requireFullDetail := func(t *testing.T, row *database.AccountBill) {
		t.Helper()
		require.InDelta(t, 300, row.PromptToken, 1e-9)
		require.InDelta(t, 40, row.PromptCachedToken, 1e-9)
		require.InDelta(t, 200, row.CompletionToken, 1e-9)
		require.InDelta(t, 60, row.ReasoningToken, 1e-9)
		require.InDelta(t, 12, row.Duration, 1e-9)
	}

	bills := func(t *testing.T, ns string) (cash, plan *database.AccountBill) {
		t.Helper()
		var rows []database.AccountBill
		err := db.Core.NewSelect().Model(&rows).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		for i := range rows {
			if rows[i].PeriodID == 0 {
				cash = &rows[i]
			} else {
				plan = &rows[i]
			}
		}
		return cash, plan
	}

	t.Run("partial coverage splits every usage detail column", func(t *testing.T) {
		ns := "foo-token-split-partial-" + uuid.NewString()
		// free remaining 200 of 500 consumed: the plan absorbs 2/5 of the fee,
		// the consumption, every token detail column and 2/5 of the duration
		seed(t, ns, &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 510, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000, UsedTokens: 800,
		})
		deduct(t, ns, 510, "model-a")

		cash, plan := bills(t, ns)
		require.NotNil(t, cash)
		require.NotNil(t, plan)

		// the uncovered 3/5 stays on the cash row, the covered 2/5 is billed
		// on the period-attributed plan row
		require.InDelta(t, 180, cash.PromptToken, 1e-9)
		require.InDelta(t, 24, cash.PromptCachedToken, 1e-9)
		require.InDelta(t, 120, cash.CompletionToken, 1e-9)
		require.InDelta(t, 36, cash.ReasoningToken, 1e-9)
		require.InDelta(t, 7.2, cash.Duration, 1e-9)

		require.InDelta(t, 120, plan.PromptToken, 1e-9)
		require.InDelta(t, 16, plan.PromptCachedToken, 1e-9)
		require.InDelta(t, 80, plan.CompletionToken, 1e-9)
		require.InDelta(t, 24, plan.ReasoningToken, 1e-9)
		require.InDelta(t, 4.8, plan.Duration, 1e-9)

		// split, never duplicated: the two rows add back to the full detail
		require.InDelta(t, 300, cash.PromptToken+plan.PromptToken, 1e-9)
		require.InDelta(t, 40, cash.PromptCachedToken+plan.PromptCachedToken, 1e-9)
		require.InDelta(t, 200, cash.CompletionToken+plan.CompletionToken, 1e-9)
		require.InDelta(t, 60, cash.ReasoningToken+plan.ReasoningToken, 1e-9)
		require.InDelta(t, 12, cash.Duration+plan.Duration, 1e-9)

		// the statement itself keeps the full detail for attribution
		var stored database.AccountStatement
		err := db.Core.NewSelect().Model(&stored).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 300, stored.PromptToken, 1e-9)
		require.InDelta(t, 200, stored.CompletionToken, 1e-9)
	})

	t.Run("no plan row keeps the whole detail on the cash row", func(t *testing.T) {
		ns := "foo-token-split-none-" + uuid.NewString()
		// exhausted quota: full pay-as-you-go, so there is no covered share
		// and no plan row to bill it on
		seed(t, ns, &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 511, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000, UsedTokens: 1000,
		})
		deduct(t, ns, 511, "model-a")

		cash, plan := bills(t, ns)
		require.NotNil(t, cash)
		require.Nil(t, plan)
		requireFullDetail(t, cash)
	})

	t.Run("model outside the plan quota keeps the whole detail on the cash row", func(t *testing.T) {
		ns := "foo-token-split-outside-" + uuid.NewString()
		// the period defines a quota for model-a only, so model-other is not
		// part of the plan: the statement charges the full pay-as-you-go fee,
		// the plan absorbs nothing and there is no plan row to bill a covered
		// share on — the cash row must carry all of the detail, not none of it
		seed(t, ns, &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 513, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000,
		})
		deduct(t, ns, 513, "model-other")

		cash, plan := bills(t, ns)
		require.NotNil(t, cash)
		require.Nil(t, plan)
		require.InDelta(t, -10, cash.Value, 1e-9)
		require.InDelta(t, 500, cash.Consumption, 1e-9)
		require.Zero(t, cash.PeriodID)
		requireFullDetail(t, cash)

		// the plan's own quota is untouched: nothing was covered by it
		var quota database.AccountTokenPlanPeriodModelQuota
		err := db.Core.NewSelect().Model(&quota).Where("period_id = ?", 513).Scan(ctx)
		require.NoError(t, err)
		require.Zero(t, quota.UsedTokens)
		require.Zero(t, quota.UsedRequestCount)
	})

	t.Run("no token plan keeps the whole detail on the cash row", func(t *testing.T) {
		ns := "foo-token-split-noplan-" + uuid.NewString()
		// a plan quota exists, but the statement resolved no period: it is pure
		// pay-as-you-go, so the single cash row keeps the fee, the consumption
		// and the entire token detail
		seed(t, ns, &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 514, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000,
		})
		deduct(t, ns, 0, "model-a")

		cash, plan := bills(t, ns)
		require.NotNil(t, cash)
		require.Nil(t, plan)
		require.InDelta(t, -10, cash.Value, 1e-9)
		require.InDelta(t, 500, cash.Consumption, 1e-9)
		require.Zero(t, cash.PeriodID)
		requireFullDetail(t, cash)

		// the statement keeps the full detail too, and no period quota moved
		var stored database.AccountStatement
		err := db.Core.NewSelect().Model(&stored).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, -10, stored.Value, 1e-9)
		require.InDelta(t, 300, stored.PromptToken, 1e-9)
		require.InDelta(t, 200, stored.CompletionToken, 1e-9)

		var quota database.AccountTokenPlanPeriodModelQuota
		err = db.Core.NewSelect().Model(&quota).Where("period_id = ?", 514).Scan(ctx)
		require.NoError(t, err)
		require.Zero(t, quota.UsedTokens)
		require.Zero(t, quota.UsedRequestCount)
	})

	t.Run("unlimited allowance bills the full detail on the plan row", func(t *testing.T) {
		ns := "foo-token-split-unlimited-" + uuid.NewString()
		// valid plan without a model quota defined: the allowance is unlimited
		// and the plan absorbs the whole fee, consumption and token detail
		_, err := db.Core.NewInsert().
			Model(&database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}).
			Exec(ctx)
		require.NoError(t, err)
		deduct(t, ns, 512, "model-a")

		cash, plan := bills(t, ns)
		require.Nil(t, cash)
		require.NotNil(t, plan)
		require.InDelta(t, -10, plan.Value, 1e-9)
		require.InDelta(t, 500, plan.Consumption, 1e-9)
		require.InDelta(t, 300, plan.PromptToken, 1e-9)
		require.InDelta(t, 40, plan.PromptCachedToken, 1e-9)
		require.InDelta(t, 200, plan.CompletionToken, 1e-9)
		require.InDelta(t, 60, plan.ReasoningToken, 1e-9)
		// no cash row exists for a fully covered statement, so the plan row
		// must carry the whole duration rather than dropping it
		require.InDelta(t, 12, plan.Duration, 1e-9)
	})
}

func TestAccountStatementStore_Create_TokenPlanDuplicateEventConsumesQuotaOnce(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	// Every billing period this test references must exist: deduct
	// accumulates the covered cost onto the period row and fails when the
	// update matches no row.
	seedTokenPlanPeriod(ctx, t, db, 200)
	seedTokenPlanPeriod(ctx, t, db, 400)
	seedTokenPlanPeriod(ctx, t, db, 700)

	ns := "foo-tokendup-" + uuid.NewString()
	au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.NoError(t, err)

	// exhausted quota: the statement charges the full fee, so a redelivered
	// event would double-charge if the dedup check did not run before any
	// mutation
	quota := &database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 200, ModelID: "model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000, UsedTokens: 1000,
	}
	_, err = db.Core.NewInsert().Model(quota).Exec(ctx)
	require.NoError(t, err)
	keyQuota := &database.AccountTokenPlanPeriodKeyQuota{
		PeriodID: 200, TokenID: 7, ModelID: "model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 2000, UsedTokens: 1000,
	}
	_, err = db.Core.NewInsert().Model(keyQuota).Exec(ctx)
	require.NoError(t, err)

	store := database.NewAccountStatementStoreWithDB(db)
	statement := database.AccountStatement{
		EventUUID:         uuid.New(),
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		CustomerID:        "model-a",
		TokenPlanPeriodID: 200,
		TokenID:           7,
		RecordedAt:        time.Now(),
	}
	require.NoError(t, store.Create(ctx, statement, types.AcctStatementExtra{}))
	require.NoError(t, store.Create(ctx, statement, types.AcctStatementExtra{}))

	var statements []database.AccountStatement
	err = db.Core.NewSelect().Model(&statements).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.Len(t, statements, 1)

	// exhausted quota covers nothing and records nothing: both usage
	// dimensions stay at their seeded values, exactly once
	var stored database.AccountTokenPlanPeriodModelQuota
	err = db.Core.NewSelect().Model(&stored).Where("id = ?", quota.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1000), stored.UsedTokens)
	require.Equal(t, int64(0), stored.UsedRequestCount)

	// the mirrored key quota likewise records no usage for the exhausted
	// statement
	var storedKey database.AccountTokenPlanPeriodKeyQuota
	err = db.Core.NewSelect().Model(&storedKey).Where("id = ?", keyQuota.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1000), storedKey.UsedTokens)
	require.Equal(t, int64(0), storedKey.UsedRequestCount)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, 90, acctUser.CashBalance, 1e-9)
}

// A concurrent redelivery of the same event: the second delivery is already in
// flight (it will block on the quota row lock) while the first one commits.
// The dedup check must run after the row lock is acquired, otherwise the
// second delivery consumes the quota again before DeductAccountFee detects the
// duplicate statement and skips — committing the quota increment without a
// statement.
//
// This needs InitTransactionTestDB: the txdb-backed InitTestDB runs every
// statement in one shared session, where a transaction never blocks on its
// own locks and the race cannot be exercised.
func TestAccountStatementStore_Create_TokenPlanConcurrentDuplicateEventConsumesQuotaOnce(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTransactionTestDB()
	defer db.Close()

	ns := "foo-tokendup-concurrent-" + uuid.NewString()
	au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.NoError(t, err)

	quota := &database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 400, ModelID: "model-a", TaskType: string(types.TextGeneration),
		QuotaTokens: 1000, UsedTokens: 0,
	}
	_, err = db.Core.NewInsert().Model(quota).Exec(ctx)
	require.NoError(t, err)

	statement := database.AccountStatement{
		EventUUID:         uuid.New(),
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		CustomerID:        "model-a",
		TokenID:           9,
		TokenPlanPeriodID: 400,
		RecordedAt:        time.Now(),
	}

	// T1 simulates the first delivery in flight: it holds the quota row lock
	// while its statement insert is still uncommitted.
	tx1, err := db.Core.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback() }()

	var locked database.AccountTokenPlanPeriodModelQuota
	lockQuery := tx1.NewSelect().Model(&locked).Where("id = ?", quota.ID).For("UPDATE")
	t.Logf("T1 lock query: %s", lockQuery.String())
	err = lockQuery.Scan(ctx)
	require.NoError(t, err)
	t.Logf("first delivery holds lock on quota row id=%d (seeded id=%d)", locked.ID, quota.ID)

	var isoLevel string
	err = db.Core.NewRaw("SHOW TRANSACTION ISOLATION LEVEL").Scan(ctx, &isoLevel)
	require.NoError(t, err)
	t.Logf("transaction isolation level: %s", isoLevel)

	// The redelivered event starts processing and blocks on the quota row lock.
	store := database.NewAccountStatementStoreWithDB(db)
	createErr := make(chan error, 1)
	go func() {
		err := store.Create(ctx, statement, types.AcctStatementExtra{})
		t.Logf("redelivery Create returned at %v with err=%v", time.Now(), err)
		createErr <- err
	}()
	// Give the redelivery time to pass its gate and reach the row lock. Even if
	// it arrives only after the commit below, the test stays valid: the dedup
	// check then sees the committed statement and skips.
	time.Sleep(200 * time.Millisecond)

	type pgLockRow struct {
		Pid     int64  `bun:"pid"`
		Mode    string `bun:"mode"`
		Granted bool   `bun:"granted"`
	}
	var pgLocks []pgLockRow
	err = db.Core.NewRaw(`SELECT pid, mode, granted FROM pg_locks WHERE relation = 'account_token_plan_period_model_quota'::regclass`).Scan(ctx, &pgLocks)
	require.NoError(t, err)
	for _, l := range pgLocks {
		t.Logf("pg_locks on quota table: pid=%d mode=%s granted=%v", l.Pid, l.Mode, l.Granted)
	}
	var blockedQueries []string
	err = db.Core.NewRaw(`SELECT query FROM pg_stat_activity WHERE wait_event_type = 'Lock'`).Scan(ctx, &blockedQueries)
	require.NoError(t, err)
	for _, q := range blockedQueries {
		t.Logf("blocked query: %s", q)
	}

	time.Sleep(300 * time.Millisecond)

	// The first delivery finishes: quota consumed, statement inserted, fee
	// deducted — then it commits and releases the lock.
	_, err = tx1.NewUpdate().Model(&database.AccountTokenPlanPeriodModelQuota{}).
		Set("used_tokens = used_tokens + 500, used_request_count = used_request_count + 1, latest_used_at = ?", time.Now().Unix()).
		Where("id = ?", quota.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = tx1.NewInsert().Model(&database.AccountStatement{
		EventUUID:         statement.EventUUID,
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		BalanceType:       types.ChargeBalance,
		CustomerID:        "model-a",
		TokenID:           9,
		TokenPlanPeriodID: 400,
		RecordedAt:        time.Now(),
	}).Exec(ctx)
	require.NoError(t, err)
	_, err = tx1.NewUpdate().Model(&database.AccountUser{}).
		Set("cash_balance = cash_balance - 10").
		Where("user_uuid = ?", ns).Exec(ctx)
	require.NoError(t, err)
	commitAt := time.Now()
	require.NoError(t, tx1.Commit())
	t.Logf("first delivery committed at %v", commitAt)

	// The redelivery unblocks, must see the committed statement and skip
	// without touching the quota a second time.
	require.NoError(t, <-createErr)

	var stored database.AccountTokenPlanPeriodModelQuota
	err = db.Core.NewSelect().Model(&stored).Where("id = ?", quota.ID).Scan(ctx)
	require.NoError(t, err)
	var statements []database.AccountStatement
	err = db.Core.NewSelect().Model(&statements).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	t.Logf("final state: statements=%d usedTokens=%d usedReqs=%d cash=%v",
		len(statements), stored.UsedTokens, stored.UsedRequestCount, acctUser.CashBalance)

	require.Equal(t, int64(500), stored.UsedTokens)
	require.Equal(t, int64(1), stored.UsedRequestCount)
	require.Len(t, statements, 1)
	require.InDelta(t, 90, acctUser.CashBalance, 1e-9)
}

// A concurrent redelivery against a period that defines no model quota row at
// all (an unrestricted plan): validateTokenPlan has no quota row to lock, so
// its duplicated-event check runs unserialized and the redelivery passes it
// while the first delivery is still in flight. The per-api-key usage mirror
// must therefore run only after DeductAccountFee's dedup — which executes
// under the account_users row lock — has confirmed the statement insert;
// otherwise the redelivery skips the statement yet still commits a second key
// usage increment.
//
// This needs InitTransactionTestDB for the same reason as the quota-row
// variant above.
func TestAccountStatementStore_Create_TokenPlanUnrestrictedConcurrentDuplicateRecordsKeyUsageOnce(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTransactionTestDB()
	defer db.Close()

	ns := "foo-tokenunres-concurrent-" + uuid.NewString()
	au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.NoError(t, err)

	// period 700 defines no model quota rows at all: every call inside it is
	// fully covered and only the per-key usage is recorded
	statement := database.AccountStatement{
		EventUUID:         uuid.New(),
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             -10,
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		CustomerID:        "model-a",
		TokenPlanPeriodID: 700,
		TokenID:           9,
		RecordedAt:        time.Now(),
	}

	// T1 simulates the first delivery in flight: it holds the account_users
	// row lock and has recorded the statement plus the key usage, all still
	// uncommitted.
	tx1, err := db.Core.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback() }()

	var lockedUser database.AccountUser
	err = tx1.NewSelect().Model(&lockedUser).Where("user_uuid = ?", ns).For("UPDATE").Scan(ctx)
	require.NoError(t, err)

	_, err = tx1.NewInsert().Model(&database.AccountStatement{
		EventUUID:         statement.EventUUID,
		UserUUID:          ns,
		Scene:             types.SceneModelServerless,
		Value:             0, // fully covered by the unrestricted plan
		EventDate:         time.Now(),
		Consumption:       500,
		ValueType:         types.TokenNumberType,
		BalanceType:       types.ChargeBalance,
		CustomerID:        "model-a",
		TokenPlanPeriodID: 700,
		TokenID:           9,
		RecordedAt:        time.Now(),
	}).Exec(ctx)
	require.NoError(t, err)

	keyQuota := &database.AccountTokenPlanPeriodKeyQuota{
		PeriodID: 700, TokenID: 9, ModelID: "model-a",
		UsedTokens: 500, UsedRequestCount: 1, LatestUsedAt: time.Now().Unix(),
	}
	_, err = tx1.NewInsert().Model(keyQuota).
		On("CONFLICT (period_id, token_id, model_id) DO UPDATE").
		Set("used_tokens = account_token_plan_period_key_quota.used_tokens + ?", 500).
		Set("used_request_count = account_token_plan_period_key_quota.used_request_count + ?", 1).
		Exec(ctx)
	require.NoError(t, err)

	// The redelivered event starts processing: the uncommitted statement is
	// invisible to it, so it passes the unserialized dedup check. With the
	// mirror deferred it then writes nothing until it blocks on the user row
	// lock inside DeductAccountFee. Even if it arrives only after the commit
	// below, the test stays valid: the dedup check then sees the committed
	// statement and skips.
	store := database.NewAccountStatementStoreWithDB(db)
	createErr := make(chan error, 1)
	go func() {
		createErr <- store.Create(ctx, statement, types.AcctStatementExtra{})
	}()
	time.Sleep(300 * time.Millisecond)

	// The first delivery finishes: statement and key usage commit together.
	require.NoError(t, tx1.Commit())

	// The redelivery unblocks, sees the committed statement and must skip
	// without recording the key usage a second time.
	require.NoError(t, <-createErr)

	var statements []database.AccountStatement
	err = db.Core.NewSelect().Model(&statements).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.Len(t, statements, 1)

	var storedKey database.AccountTokenPlanPeriodKeyQuota
	err = db.Core.NewSelect().Model(&storedKey).
		Where("period_id = ?", 700).Where("token_id = ?", 9).Where("model_id = ?", "model-a").
		Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(500), storedKey.UsedTokens)
	require.Equal(t, int64(1), storedKey.UsedRequestCount)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, 100, acctUser.CashBalance, 1e-9)
}

// A statement carrying a resolved api key id must mirror its consumption onto
// the optional per-api-key quota row of the same period and model, while the
// charge split stays driven by the period-level model quota.
func TestAccountStatementStore_Create_TokenPlanKeyQuotaMirrorsUsage(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTestDB()
	defer db.Close()

	// Every billing period this test references must exist: deduct
	// accumulates the covered cost onto the period row and fails when the
	// update matches no row.
	seedTokenPlanPeriod(ctx, t, db, 500)
	seedTokenPlanPeriod(ctx, t, db, 501)
	seedTokenPlanPeriod(ctx, t, db, 502)
	seedTokenPlanPeriod(ctx, t, db, 503)
	seedTokenPlanPeriod(ctx, t, db, 504)
	seedTokenPlanPeriod(ctx, t, db, 505)
	seedTokenPlanPeriod(ctx, t, db, 506)

	store := database.NewAccountStatementStoreWithDB(db)

	seedAccount := func(t *testing.T, ns string) {
		au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
		_, err := db.Core.NewInsert().Model(au).Exec(ctx)
		require.NoError(t, err)
	}

	t.Run("token-metred usage mirrors onto the key quota", func(t *testing.T) {
		ns := "foo-tokenkey-token-" + uuid.NewString()
		seedAccount(t, ns)

		quota := &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 500, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000,
		}
		_, err := db.Core.NewInsert().Model(quota).Exec(ctx)
		require.NoError(t, err)
		keyQuota := &database.AccountTokenPlanPeriodKeyQuota{
			PeriodID: 500, TokenID: 7, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 500, UsedTokens: 50, QuotaRequestCount: 5, UsedRequestCount: 1,
		}
		_, err = db.Core.NewInsert().Model(keyQuota).Exec(ctx)
		require.NoError(t, err)

		err = store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneModelServerless, Value: -10, Consumption: 300,
			ValueType: types.TokenNumberType, CustomerID: "model-a",
			TokenPlanPeriodID: 500, TokenID: 7,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		// the model quota drives the charge split: fully covered, fee zero
		var stored database.AccountStatement
		err = db.Core.NewSelect().Model(&stored).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 0, stored.Value, 1e-9)

		var storedQuota database.AccountTokenPlanPeriodModelQuota
		err = db.Core.NewSelect().Model(&storedQuota).Where("id = ?", quota.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(300), storedQuota.UsedTokens)
		require.Equal(t, int64(1), storedQuota.UsedRequestCount)

		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).Where("id = ?", keyQuota.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(350), storedKey.UsedTokens)
		require.Equal(t, int64(2), storedKey.UsedRequestCount)
		require.Positive(t, storedKey.LatestUsedAt)
	})

	t.Run("count-metred usage mirrors one request unit onto the key quota", func(t *testing.T) {
		ns := "foo-tokenkey-count-" + uuid.NewString()
		seedAccount(t, ns)

		quota := &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 501, ModelID: "model-b", TaskType: string(types.Text2Video),
			QuotaRequestCount: 10,
		}
		_, err := db.Core.NewInsert().Model(quota).Exec(ctx)
		require.NoError(t, err)
		keyQuota := &database.AccountTokenPlanPeriodKeyQuota{
			PeriodID: 501, TokenID: 8, ModelID: "model-b", TaskType: string(types.Text2Video),
			QuotaRequestCount: 3,
		}
		_, err = db.Core.NewInsert().Model(keyQuota).Exec(ctx)
		require.NoError(t, err)

		err = store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneMultiModalServerless, Value: -8, Consumption: 4,
			ValueType: types.CountNumberType, CustomerID: "model-b",
			TokenPlanPeriodID: 501, TokenID: 8,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		var stored database.AccountStatement
		err = db.Core.NewSelect().Model(&stored).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 0, stored.Value, 1e-9)

		var storedQuota database.AccountTokenPlanPeriodModelQuota
		err = db.Core.NewSelect().Model(&storedQuota).Where("id = ?", quota.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(4), storedQuota.UsedTokens)
		require.Equal(t, int64(1), storedQuota.UsedRequestCount)

		// the count dimension decides coverage; the metered consumption (4)
		// is still recorded as token usage and mirrored onto the key quota
		// together with one request unit
		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).Where("id = ?", keyQuota.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(4), storedKey.UsedTokens)
		require.Equal(t, int64(1), storedKey.UsedRequestCount)
		require.Positive(t, storedKey.LatestUsedAt)
	})

	t.Run("a missing key quota row is created inheriting the model allowances", func(t *testing.T) {
		ns := "foo-tokenkey-nokey-" + uuid.NewString()
		seedAccount(t, ns)

		quota := &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 502, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000,
		}
		_, err := db.Core.NewInsert().Model(quota).Exec(ctx)
		require.NoError(t, err)

		err = store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneModelServerless, Value: -10, Consumption: 500,
			ValueType: types.TokenNumberType, CustomerID: "model-a",
			TokenPlanPeriodID: 502, TokenID: 9,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		var storedQuota database.AccountTokenPlanPeriodModelQuota
		err = db.Core.NewSelect().Model(&storedQuota).Where("id = ?", quota.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(500), storedQuota.UsedTokens)

		// the key quota row is created on the fly: task type and allowances
		// inherited from the model quota row, usage recorded
		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).
			Where("period_id = ?", 502).Where("token_id = ?", 9).Where("model_id = ?", "model-a").
			Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, string(types.TextGeneration), storedKey.TaskType)
		require.Equal(t, int64(1000), storedKey.QuotaTokens)
		require.Zero(t, storedKey.QuotaRequestCount)
		require.Equal(t, int64(500), storedKey.UsedTokens)
		require.Equal(t, int64(1), storedKey.UsedRequestCount)
		require.Positive(t, storedKey.LatestUsedAt)

		// a second statement accumulates onto the same row instead of
		// inserting another one
		err = store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneModelServerless, Value: -10, Consumption: 200,
			ValueType: types.TokenNumberType, CustomerID: "model-a",
			TokenPlanPeriodID: 502, TokenID: 9,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		keyCount, err := db.Core.NewSelect().Model(&database.AccountTokenPlanPeriodKeyQuota{}).
			Where("period_id = ?", 502).Where("token_id = ?", 9).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, keyCount)

		err = db.Core.NewSelect().Model(&storedKey).Where("id = ?", storedKey.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(700), storedKey.UsedTokens)
		require.Equal(t, int64(2), storedKey.UsedRequestCount)
	})

	t.Run("statement without api key id stays pay-as-you-go", func(t *testing.T) {
		ns := "foo-tokenkey-notoken-" + uuid.NewString()
		seedAccount(t, ns)

		quota := &database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 503, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000,
		}
		_, err := db.Core.NewInsert().Model(quota).Exec(ctx)
		require.NoError(t, err)
		keyQuota := &database.AccountTokenPlanPeriodKeyQuota{
			PeriodID: 503, TokenID: 7, ModelID: "model-a", TaskType: string(types.TextGeneration),
			QuotaTokens: 500, UsedTokens: 50, QuotaRequestCount: 5, UsedRequestCount: 1,
		}
		_, err = db.Core.NewInsert().Model(keyQuota).Exec(ctx)
		require.NoError(t, err)

		err = store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneModelServerless, Value: -10, Consumption: 500,
			ValueType: types.TokenNumberType, CustomerID: "model-a",
			TokenPlanPeriodID: 503, TokenID: 0, CostAmount: 25,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		var storedStatement database.AccountStatement
		err = db.Core.NewSelect().Model(&storedStatement).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), storedStatement.TokenID)
		require.InDelta(t, -10, storedStatement.Value, 1e-9)

		var acctUser database.AccountUser
		err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 90, acctUser.CashBalance, 1e-9)

		var period database.AccountTokenPlanInstancePeriod
		err = db.Core.NewSelect().Model(&period).Where("id = ?", 503).Scan(ctx)
		require.NoError(t, err)
		require.Zero(t, period.CostAmount)

		var bills []database.AccountBill
		err = db.Core.NewSelect().Model(&bills).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.Len(t, bills, 1)
		require.Zero(t, bills[0].PeriodID)
		require.InDelta(t, -10, bills[0].Value, 1e-9)

		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).Where("id = ?", keyQuota.ID).Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(50), storedKey.UsedTokens)
		require.Equal(t, int64(1), storedKey.UsedRequestCount)
		require.Zero(t, storedKey.LatestUsedAt)

		// no row is auto-created for an unresolved api key id either
		keyCount, err := db.Core.NewSelect().Model(&database.AccountTokenPlanPeriodKeyQuota{}).
			Where("period_id = ?", 503).Where("token_id = ?", 0).Count(ctx)
		require.NoError(t, err)
		require.Zero(t, keyCount)
	})

	t.Run("unrestricted period still records the key usage", func(t *testing.T) {
		ns := "foo-tokenkey-unrestricted-" + uuid.NewString()
		seedAccount(t, ns)

		// period 504 defines no model quota at all: the call is fully covered
		// and the key usage is recorded with an empty task type
		err := store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneModelServerless, Value: -10, Consumption: 500,
			ValueType: types.TokenNumberType, CustomerID: "model-a",
			TokenPlanPeriodID: 504, TokenID: 9,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		var stored database.AccountStatement
		err = db.Core.NewSelect().Model(&stored).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 0, stored.Value, 1e-9)

		modelCount, err := db.Core.NewSelect().Model(&database.AccountTokenPlanPeriodModelQuota{}).
			Where("period_id = ?", 504).Count(ctx)
		require.NoError(t, err)
		require.Zero(t, modelCount)

		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).
			Where("period_id = ?", 504).Where("token_id = ?", 9).Where("model_id = ?", "model-a").
			Scan(ctx)
		require.NoError(t, err)
		require.Empty(t, storedKey.TaskType)
		require.Zero(t, storedKey.QuotaTokens)
		require.Equal(t, int64(500), storedKey.UsedTokens)
		require.Equal(t, int64(1), storedKey.UsedRequestCount)
		require.Positive(t, storedKey.LatestUsedAt)
	})

	t.Run("unrestricted count-metred call records one request unit", func(t *testing.T) {
		ns := "foo-tokenkey-unrestricted-count-" + uuid.NewString()
		seedAccount(t, ns)

		err := store.Create(ctx, database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneMultiModalServerless, Value: -8, Consumption: 4,
			ValueType: types.CountNumberType, CustomerID: "model-b",
			TokenPlanPeriodID: 505, TokenID: 9,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}, types.AcctStatementExtra{})
		require.NoError(t, err)

		var stored database.AccountStatement
		err = db.Core.NewSelect().Model(&stored).Where("user_uuid = ?", ns).Scan(ctx)
		require.NoError(t, err)
		require.InDelta(t, 0, stored.Value, 1e-9)

		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).
			Where("period_id = ?", 505).Where("token_id = ?", 9).Where("model_id = ?", "model-b").
			Scan(ctx)
		require.NoError(t, err)
		// the unrestricted period covers the whole call and records the
		// metered consumption as token usage alongside one request unit
		require.Equal(t, int64(4), storedKey.UsedTokens)
		require.Equal(t, int64(1), storedKey.UsedRequestCount)
	})

	t.Run("unrestricted period records a redelivered event once", func(t *testing.T) {
		ns := "foo-tokenkey-unrestricted-dup-" + uuid.NewString()
		seedAccount(t, ns)

		statement := database.AccountStatement{
			EventUUID: uuid.New(), UserUUID: ns,
			Scene: types.SceneModelServerless, Value: -10, Consumption: 500,
			ValueType: types.TokenNumberType, CustomerID: "model-a",
			TokenPlanPeriodID: 506, TokenID: 9,
			EventDate: time.Now(), RecordedAt: time.Now(),
		}
		require.NoError(t, store.Create(ctx, statement, types.AcctStatementExtra{}))
		require.NoError(t, store.Create(ctx, statement, types.AcctStatementExtra{}))

		statementCount, err := db.Core.NewSelect().Model(&database.AccountStatement{}).
			Where("user_uuid = ?", ns).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, statementCount)

		var storedKey database.AccountTokenPlanPeriodKeyQuota
		err = db.Core.NewSelect().Model(&storedKey).
			Where("period_id = ?", 506).Where("token_id = ?", 9).Where("model_id = ?", "model-a").
			Scan(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(500), storedKey.UsedTokens)
		require.Equal(t, int64(1), storedKey.UsedRequestCount)
	})
}

func TestAccountStatementStore_Create_TokenPlanParallelQuota(t *testing.T) {
	ctx := context.TODO()
	db := tests.InitTransactionTestDB()
	defer db.Close()

	// Every billing period this test references must exist: deduct
	// accumulates the covered cost onto the period row and fails when the
	// update matches no row.
	seedTokenPlanPeriod(ctx, t, db, 300)

	ns := "foo-tokenpar-" + uuid.NewString()
	au := &database.AccountUser{UserUUID: ns, Balance: 0, CashBalance: 100}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.NoError(t, err)

	quota := &database.AccountTokenPlanPeriodModelQuota{
		PeriodID: 300, ModelID: "model-a", TaskType: string(types.TextGeneration), QuotaTokens: 1000,
	}
	_, err = db.Core.NewInsert().Model(quota).Exec(ctx)
	require.NoError(t, err)

	store := database.NewAccountStatementStoreWithDB(db)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := store.Create(ctx, database.AccountStatement{
				EventUUID:         uuid.New(),
				UserUUID:          ns,
				Scene:             types.SceneModelServerless,
				Value:             -60,
				EventDate:         time.Now(),
				Consumption:       600,
				ValueType:         types.TokenNumberType,
				CustomerID:        "model-a",
				TokenID:           9,
				TokenPlanPeriodID: 300,
				RecordedAt:        time.Now(),
			}, types.AcctStatementExtra{})
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	// the FOR UPDATE quota lock serializes the two events: whichever runs
	// first is fully covered (600 of 1000 free), the second finds 400 free of
	// 600 consumed and pays the uncovered 200 tokens => 20 of the 60 fee.
	// Only the covered portions are recorded, so UsedTokens stops at the 1000
	// allowance. Without the lock both could read used=0 and both would
	// charge zero.
	var statements []database.AccountStatement
	err = db.Core.NewSelect().Model(&statements).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.Len(t, statements, 2)
	total := statements[0].Value + statements[1].Value
	require.InDelta(t, -20, total, 1e-9)

	var stored database.AccountTokenPlanPeriodModelQuota
	err = db.Core.NewSelect().Model(&stored).Where("id = ?", quota.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1000), stored.UsedTokens)
	require.Equal(t, int64(2), stored.UsedRequestCount)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).Where("user_uuid = ?", ns).Scan(ctx)
	require.NoError(t, err)
	require.InDelta(t, 80, acctUser.CashBalance, 1e-9)
}

func TestAccountStatementStore_ListByUserIDAndTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 1, 12, 12, 12, 0, 0, time.UTC)
	statements := []database.AccountStatement{
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "a",
			CustomerID: "bar", CreatedAt: dt.Add(-5 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "b",
			CustomerID: "bar", CreatedAt: dt.Add(-2 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "c",
			CustomerID: "bar", CreatedAt: dt.Add(2 * time.Hour), Consumption: -1,
		},
		// not match
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "d",
			CustomerID: "bar", CreatedAt: dt.Add(7 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "bar", Scene: types.SceneCashCharge, Value: 1, OpUID: "e",
			CustomerID: "bar", CreatedAt: dt.Add(7 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneEvaluation, Value: 1, OpUID: "f",
			CustomerID: "bar", CreatedAt: dt.Add(-1 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneEvaluation, Value: 1, OpUID: "g",
			CustomerID: "bar", CreatedAt: dt.Add(1 * time.Hour), Consumption: -1,
		},
	}

	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)
	result, err := store.ListByUserIDAndTime(ctx, types.ActStatementsReq{
		UserUUID:     "foo",
		Scene:        3,
		InstanceName: "bar",
		StartTime:    dt.Add(-6 * time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(6 * time.Hour).Format(time.RFC3339),
	})
	require.Nil(t, err)
	require.Equal(t, 3, result.Total)
	require.Equal(t, float64(3), result.TotalValue)
	require.Equal(t, float64(-3), result.TotalConsumption)

	uids := []string{}
	for _, d := range result.Data {
		uids = append(uids, d.OpUID)
	}
	require.ElementsMatch(t, []string{"a", "b", "c"}, uids)

}

func TestAccountStatementStore_ListByUserIDAndTimeVoucherFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 1, 12, 12, 12, 0, 0, time.UTC)
	statements := []database.AccountStatement{
		{UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "a", CustomerID: "bar", CreatedAt: dt, VoucherNo: "VC-2022-000001", TokenPlanPeriodID: 11},
		{UserUUID: "foo", Scene: types.SceneCashCharge, Value: 2, OpUID: "b", CustomerID: "bar", CreatedAt: dt, VoucherNo: "VC-2022-000002", TokenPlanPeriodID: 22},
		{UserUUID: "foo", Scene: types.SceneCashCharge, Value: 3, OpUID: "c", CustomerID: "bar", CreatedAt: dt, VoucherNo: "VC-2022-000001", TokenPlanPeriodID: 22},
		// not match: another user
		{UserUUID: "bar", Scene: types.SceneCashCharge, Value: 4, OpUID: "d", CustomerID: "bar", CreatedAt: dt, VoucherNo: "VC-2022-000001", TokenPlanPeriodID: 11},
	}
	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)
	baseReq := types.ActStatementsReq{
		UserUUID:  "foo",
		Scene:     types.SceneCashCharge,
		StartTime: dt.Add(-time.Hour).Format(time.RFC3339),
		EndTime:   dt.Add(time.Hour).Format(time.RFC3339),
		Per:       10,
		Page:      1,
	}

	t.Run("without voucher filters", func(t *testing.T) {
		result, err := store.ListByUserIDAndTime(ctx, baseReq)
		require.Nil(t, err)
		require.Equal(t, 3, result.Total)
	})

	t.Run("filters by voucher number", func(t *testing.T) {
		req := baseReq
		req.VoucherNo = "VC-2022-000001"
		result, err := store.ListByUserIDAndTime(ctx, req)
		require.Nil(t, err)
		require.Equal(t, 2, result.Total)
		opUIDs := []string{}
		for _, d := range result.Data {
			opUIDs = append(opUIDs, d.OpUID)
		}
		require.ElementsMatch(t, []string{"a", "c"}, opUIDs)
	})

	t.Run("filters by token plan period", func(t *testing.T) {
		req := baseReq
		req.PeriodID = 22
		result, err := store.ListByUserIDAndTime(ctx, req)
		require.Nil(t, err)
		require.Equal(t, 2, result.Total)
		opUIDs := []string{}
		for _, d := range result.Data {
			opUIDs = append(opUIDs, d.OpUID)
		}
		require.ElementsMatch(t, []string{"b", "c"}, opUIDs)
	})

	t.Run("combines voucher number and period", func(t *testing.T) {
		req := baseReq
		req.VoucherNo = "VC-2022-000001"
		req.PeriodID = 22
		result, err := store.ListByUserIDAndTime(ctx, req)
		require.Nil(t, err)
		require.Equal(t, 1, result.Total)
		require.Len(t, result.Data, 1)
		require.Equal(t, "c", result.Data[0].OpUID)
	})
}

func TestAccountStatementStore_GetByEventID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	uid := uuid.New()
	_, err := db.Core.NewInsert().Model(&database.AccountStatement{
		EventUUID: uid,
		OpUID:     "foo",
	}).Exec(ctx)
	require.Nil(t, err)

	ev, err := database.NewAccountStatementStoreWithDB(db).GetByEventID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, "foo", ev.OpUID)
}

func TestAccountStatementStore_MinCreatedAt(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountStatementStoreWithDB(db)

	// empty table -> zero time
	minTime, err := store.MinCreatedAt(ctx)
	require.Nil(t, err)
	require.True(t, minTime.IsZero())

	// insert two statements with different created_at
	earliest := time.Date(2024, 6, 18, 10, 0, 0, 0, time.UTC)
	later := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	_, err = db.Core.NewInsert().Model(&database.AccountStatement{
		EventUUID: uuid.New(), UserUUID: "u1", CreatedAt: later,
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.AccountStatement{
		EventUUID: uuid.New(), UserUUID: "u2", CreatedAt: earliest,
	}).Exec(ctx)
	require.Nil(t, err)

	minTime, err = store.MinCreatedAt(ctx)
	require.Nil(t, err)
	require.Equal(t, earliest.UTC(), minTime.UTC())
}

func TestAccountStatementStore_ListRechargeByUserIDAndTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	uuid1 := uuid.New()
	uuid2 := uuid.New()

	dt := time.Date(2022, 1, 12, 12, 12, 0, 0, time.UTC)
	statements := []database.AccountStatement{
		{
			EventUUID: uuid1,
			UserUUID:  "foo", Scene: types.ScenePortalCharge, Value: 1, OpUID: "a",
			CustomerID: "", CreatedAt: dt.Add(-5 * time.Hour), Consumption: -1,
		},
		{
			EventUUID: uuid2,
			UserUUID:  "foo", Scene: types.ScenePortalCharge, Value: 1, OpUID: "b",
			CustomerID: "", CreatedAt: dt.Add(-2 * time.Hour), Consumption: -1,
		},
	}

	presents := []database.AccountPresent{
		{
			EventUUID:  uuid1,
			UserUUID:   "foo",
			ActivityID: 1001,
			Value:      100,
		}, {
			EventUUID:  uuid2,
			UserUUID:   "foo",
			ActivityID: 1002,
			Value:      200,
		},
	}

	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	for _, p := range presents {
		_, err := db.Core.NewInsert().Model(&p).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)
	result, err := store.ListRechargeByUserIDAndTime(ctx,
		types.AcctRechargeListReq{
			TargetUUID: "foo",
			Scene:      int(types.ScenePortalCharge),
			ActivityID: 0,
			StartTime:  dt.Add(-6 * time.Hour).Format(time.RFC3339),
			EndTime:    dt.Add(6 * time.Hour).Format(time.RFC3339),
		},
	)
	require.Nil(t, err)
	require.Equal(t, 2, result.Total)
	require.Equal(t, float64(2), result.TotalValue)

	opids := []string{}
	aids := []int64{}
	for _, d := range result.Data {
		opids = append(opids, d.OpUID)
		aids = append(aids, d.Present.ActivityID)
	}
	require.ElementsMatch(t, []string{"a", "b"}, opids)
	require.ElementsMatch(t, []int64{int64(1001), int64(1002)}, aids)
}

func TestAccountStatementStore_ListGroupedByUserAndSku(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 1, 12, 12, 0, 0, 0, time.UTC)

	statements := []database.AccountStatement{
		{
			UserUUID: "user1", SkuID: 1001, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 10, Consumption: -2,
		},
		{
			UserUUID: "user1", SkuID: 1001, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 20, Consumption: -3,
		},
		{
			UserUUID: "user1", SkuID: 1002, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 30, Consumption: -4,
		},
		{
			UserUUID: "user2", SkuID: 1001, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 999, Consumption: -999,
		},
	}

	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.NoError(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)

	res, total, err := store.ListStatementByUserAndSku(ctx, types.ActStatementsReq{
		UserUUID:     "user1",
		Scene:        types.SceneCashCharge,
		InstanceName: "cust1",
		StartTime:    dt.Add(-time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(time.Hour).Format(time.RFC3339),
		Per:          0,
	})
	require.NoError(t, err)

	require.Equal(t, 2, total)

	groupMap := map[int64]struct {
		TotalValue       float64
		TotalConsumption float64
	}{}
	for _, r := range res {
		groupMap[r.SkuID] = struct {
			TotalValue       float64
			TotalConsumption float64
		}{
			TotalValue:       r.TotalValue,
			TotalConsumption: r.TotalConsumption,
		}
	}

	require.Equal(t, float64(30), groupMap[1001].TotalValue)
	require.Equal(t, float64(-5), groupMap[1001].TotalConsumption)

	require.Equal(t, float64(30), groupMap[1002].TotalValue)
	require.Equal(t, float64(-4), groupMap[1002].TotalConsumption)
}

func TestGetConsumeResource(t *testing.T) {
	cases := []struct {
		name              string
		scene             types.SceneType
		spaceResource     *database.SpaceResource
		expectClusterID   string
		expectHardwareTyp string
		expectErr         bool
	}{
		{
			name:              "scene not need extract hardware",
			scene:             types.SceneCashCharge,
			spaceResource:     nil,
			expectClusterID:   "",
			expectHardwareTyp: "",
			expectErr:         false,
		},
		{
			name:  "scene need extract hardware with valid resource",
			scene: types.SceneSpace,
			spaceResource: &database.SpaceResource{
				Name:      "test-resource",
				Resources: `{"gpu": {"num": "1", "type": "A10"}}`,
				ClusterID: "test-cluster",
			},
			expectClusterID:   "test-cluster",
			expectHardwareTyp: "A10",
			expectErr:         false,
		},
		{
			name:  "scene need extract hardware with invalid resource id",
			scene: types.SceneModelInference,
			spaceResource: &database.SpaceResource{
				Name:      "test-resource",
				Resources: `{"gpu": {"num": "1", "type": "A10"}}`,
				ClusterID: "test-cluster",
			},
			expectClusterID:   "",
			expectHardwareTyp: "",
			expectErr:         true,
		},
		{
			name:  "scene need extract hardware with missing resource record",
			scene: types.SceneEvaluation,
			spaceResource: &database.SpaceResource{
				Name:      "other-resource",
				Resources: `{"gpu": {"num": "1", "type": "A10"}}`,
				ClusterID: "other-cluster",
			},
			expectClusterID:   "",
			expectHardwareTyp: "",
			expectErr:         true,
		},
		{
			name:  "scene need extract hardware with cpu only resource",
			scene: types.SceneModelFinetune,
			spaceResource: &database.SpaceResource{
				Name:      "cpu-resource",
				Resources: `{"cpu": {"num": "2", "type": "Intel"}}`,
				ClusterID: "cpu-cluster",
			},
			expectClusterID:   "cpu-cluster",
			expectHardwareTyp: "Intel",
			expectErr:         false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			resourceID := "not-a-number"
			if c.spaceResource != nil {
				_, err := db.Core.NewInsert().Model(c.spaceResource).Exec(ctx, c.spaceResource)
				require.Nil(t, err)
				resourceID = fmt.Sprintf("%d", c.spaceResource.ID)
			}

			err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				input := database.AccountStatement{
					Scene:      c.scene,
					ResourceID: resourceID,
				}

				if c.name == "scene need extract hardware with invalid resource id" {
					input.ResourceID = "not-a-number"
				}
				if c.name == "scene need extract hardware with missing resource record" {
					input.ResourceID = "99999"
				}

				result, err := database.GetConsumeResource(ctx, tx, input)
				if c.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, c.expectClusterID, result.ClusterID)
					require.Equal(t, c.expectHardwareTyp, result.HardwareType)
				}
				return nil
			})
			require.Nil(t, err)
		})
	}
}

func TestAccountStatementStore_GetDeductionSummaryByUserSceneCustomer(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountStatementStoreWithDB(db)

	t.Run("returns aggregated value for multiple records", func(t *testing.T) {
		uid := uuid.New()
		_, err := db.Core.NewInsert().Model(&database.AccountStatement{
			EventUUID: uid, UserUUID: "sum_u1", Scene: types.SceneSpace, CustomerID: "sum_c1", Value: -6,
		}).Exec(ctx)
		require.Nil(t, err)
		_, err = db.Core.NewInsert().Model(&database.AccountStatement{
			EventUUID: uid, UserUUID: "sum_u1", Scene: types.SceneSpace, CustomerID: "sum_c1", Value: -4,
		}).Exec(ctx)
		require.Nil(t, err)

		result, err := store.GetDeductionSummaryByUserSceneCustomer(ctx, database.AccountStatement{
			UserUUID: "sum_u1", Scene: types.SceneSpace, CustomerID: "sum_c1",
		})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, -10.0, result.TotalValue)
		require.Equal(t, "sum_u1", result.UserUUID)
		require.Equal(t, types.SceneSpace, result.Scene)
		require.Equal(t, "sum_c1", result.CustomerID)
		require.Equal(t, uid, result.EventUUID)
	})

	t.Run("no matching record returns error", func(t *testing.T) {
		_, err := store.GetDeductionSummaryByUserSceneCustomer(ctx, database.AccountStatement{
			UserUUID: "nonexistent_sum", Scene: types.SceneSpace, CustomerID: "nope",
		})
		require.Error(t, err)
	})

	t.Run("does not match different customer_id", func(t *testing.T) {
		uid := uuid.New()
		_, err := db.Core.NewInsert().Model(&database.AccountStatement{
			EventUUID: uid, UserUUID: "sum_u2", Scene: types.SceneSpace, CustomerID: "sum_c2", Value: -5,
		}).Exec(ctx)
		require.Nil(t, err)

		_, err = store.GetDeductionSummaryByUserSceneCustomer(ctx, database.AccountStatement{
			UserUUID: "sum_u2", Scene: types.SceneSpace, CustomerID: "other_sum",
		})
		require.Error(t, err)
	})
}

func TestAccountStatementStore_DeductFeeStatement_AcademyIdempotent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountStatementStoreWithDB(db)

	// Setup account user with sufficient balance
	au := database.AccountUser{
		UserUUID:    "academy_idem_user",
		Balance:     0,
		CashBalance: 100,
	}
	_, err := db.Core.NewInsert().Model(&au).Exec(ctx)
	require.Nil(t, err)

	input := database.AccountStatement{
		EventUUID:  uuid.New(),
		UserUUID:   "academy_idem_user",
		Scene:      types.SceneAcademyPurchase,
		Value:      -10,
		CustomerID: "academy_order_1",
		EventDate:  time.Now().UTC(),
		RecordedAt: time.Now().UTC(),
	}

	// First call: should succeed and deduct balance
	err = store.Create(ctx, input, types.AcctStatementExtra{CheckBalance: true})
	require.Nil(t, err)

	// Verify balance was deducted
	acctUser := &database.AccountUser{}
	err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "academy_idem_user").Scan(ctx)
	require.Nil(t, err)
	firstBalance := acctUser.CashBalance
	require.Equal(t, 90.0, firstBalance)

	// Verify statement was created
	var stmtCount int
	stmtCount, err = db.Core.NewSelect().Model((*database.AccountStatement)(nil)).
		Where("user_uuid = ?", "academy_idem_user").
		Where("scene = ?", types.SceneAcademyPurchase).
		Where("customer_id = ?", "academy_order_1").
		Count(ctx)
	require.Nil(t, err)
	firstStmtCount := stmtCount

	// Second call (duplicate customer_id): should succeed (idempotent) but NOT change balance, statement, or bill
	dupInput := database.AccountStatement{
		EventUUID:  uuid.New(), // different event_uuid, same user+scene+customer_id
		UserUUID:   "academy_idem_user",
		Scene:      types.SceneAcademyPurchase,
		Value:      -10,
		CustomerID: "academy_order_1",
		EventDate:  time.Now().UTC(),
		RecordedAt: time.Now().UTC(),
	}
	err = store.Create(ctx, dupInput, types.AcctStatementExtra{CheckBalance: true})
	require.Nil(t, err)

	// Balance should not change after duplicate call
	err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "academy_idem_user").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, firstBalance, acctUser.CashBalance, "balance should not change after duplicate academy order")

	// Statement count should not increase after duplicate call
	stmtCount, err = db.Core.NewSelect().Model((*database.AccountStatement)(nil)).
		Where("user_uuid = ?", "academy_idem_user").
		Where("scene = ?", types.SceneAcademyPurchase).
		Where("customer_id = ?", "academy_order_1").
		Count(ctx)
	require.Nil(t, err)
	require.Equal(t, firstStmtCount, stmtCount, "statement count should not increase after duplicate academy order")
}

// A non-numeric api_key (a legacy raw key value from an in-flight event) must
// not interrupt the deduction: the store persists the statement with the
// degraded token_id 0 and the bill inherits it. The api_key parse and its
// failure log live in the accounting component (see the component test
// TestStatementComponent_AddNewStatement_NonNumericAPIKeyDegradesToZeroTokenID).
func TestAccountStatementStore_CreateNonNumericAPIKeyDegradesToZeroTokenID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	au := &database.AccountUser{
		UserUUID:    "user-nonnumeric-key",
		Balance:     100,
		CashBalance: 0,
	}
	_, err := db.Core.NewInsert().Model(au).Exec(ctx)
	require.Nil(t, err)

	const rawLegacyKey = "sk-legacy-do-not-log-0123456789"
	eventDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	store := database.NewAccountStatementStoreWithDB(db)
	input := database.AccountStatement{
		EventUUID:   uuid.New(),
		UserUUID:    "user-nonnumeric-key",
		Scene:       types.SceneModelServerless,
		Value:       -10,
		Consumption: 100,
		ValueType:   types.TokenNumberType,
		EventDate:   eventDate,
		APIKey:      rawLegacyKey,
	}
	err = store.Create(ctx, input, types.AcctStatementExtra{})
	require.Nil(t, err)

	// The deduction continues: balance 100 - 10 = 90.
	acctUser := &database.AccountUser{}
	err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "user-nonnumeric-key").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, float64(90), acctUser.Balance)

	// The statement degrades to token_id 0.
	as := &database.AccountStatement{}
	err = db.Core.NewSelect().Model(as).Where("user_uuid = ?", "user-nonnumeric-key").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(0), as.TokenID)
	require.Equal(t, types.ChargeBalance, as.BalanceType)

	// The bill inherits the degraded token_id 0.
	bill := &database.AccountBill{}
	err = db.Core.NewSelect().Model(bill).Where("user_uuid = ?", "user-nonnumeric-key").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(0), bill.TokenID)
	require.Equal(t, float64(-10), bill.Value)
}
