package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
)

type scenarioConstraintStoreImpl struct {
	db *DB
}

type ScenarioConstraintStore interface {
	// FindByScenario returns the constraint configured for a single scenario,
	// or nil if no row matches.
	FindByScenario(ctx context.Context, scenario string) (*ScenarioConstraint, error)
	// FindByCode returns the scenario row for the given code (bit position), or
	// nil if no row matches. Used by the Index API to resolve the deploy_type
	// int into a scenario mask (1<<code) and name.
	FindByCode(ctx context.Context, code int) (*ScenarioConstraint, error)
	// FindAll returns every configured scenario constraint.
	FindAll(ctx context.Context) ([]ScenarioConstraint, error)
	// FindAllOrdered returns every scenario ordered by code (ascending). Used by
	// the /scenarios listing API and by the mask<->name conversion helpers.
	FindAllOrdered(ctx context.Context) ([]ScenarioConstraint, error)
	// Upsert inserts or updates the constraint for a scenario (scenario is the
	// unique key).
	Upsert(ctx context.Context, input ScenarioConstraint) (*ScenarioConstraint, error)
	// Delete removes the constraint for a scenario.
	Delete(ctx context.Context, scenario string) error
}

func NewScenarioConstraintStore() ScenarioConstraintStore {
	return &scenarioConstraintStoreImpl{db: defaultDB}
}

func NewScenarioConstraintStoreWithDB(db *DB) ScenarioConstraintStore {
	return &scenarioConstraintStoreImpl{db: db}
}

// ScenarioConstraint is both the scenario catalog and the per-scenario filtering
// constraint. It drives the dynamic filtering in the space resource Index query,
// replacing the previously hardcoded deployAvailable / replica checks, and is the
// single source of truth for the scenario list (no Go-side scenarioBit table).
//
//   - Scenario: scenario name (e.g. "finetune", "sandbox", "wf_evaluation").
//   - Code: bit position. Deploy scenarios use the DeployType int values 0-7 so
//     the deploy_type passed to Index doubles as a scenario code; workflow
//     scenarios use 32-39. The bitmask is `1 << Code`. Unique.
//   - Category: "deploy" or "workflow".
//   - DisplayName: i18n key (e.g. "scenario.finetune") resolved by the frontend
//   - I18nKey: i18n key (e.g. "scenario.finetune") resolved by the frontend
//     via $t; the localized text lives in the frontend locale files.
//   - RequiredHardware: bitmask of hardware types the resource MUST have at
//     least one of ("at least" semantics, tested with
//     `hardwareMask & required != 0`). 0 means no "must have" requirement.
//     e.g. finetune = HardwareMaskGraphic (must have a graphic accelerator).
//   - ExcludeHardware: bitmask of hardware types the resource MUST NOT have
//     ("none of" semantics, tested with `hardwareMask & exclude == 0`). 0 means
//     nothing is excluded. e.g. sandbox = HardwareMaskGraphic (must be pure CPU,
//     no accelerator) — this is how "pure CPU" is expressed, since required_hardware
//     can only say "has CPU", not "only CPU".
//   - MaxReplica: max replicas allowed for this scenario. 0 means unlimited.
type ScenarioConstraint struct {
	bun.BaseModel    `bun:"table:space_resource_scenario_constraints"`
	ID               int64  `bun:",pk,autoincrement" json:"id"`
	Scenario         string `bun:",notnull,unique" json:"scenario"`
	Code             int    `bun:",notnull,unique" json:"code"`
	Category         string `bun:",notnull,default:'deploy'" json:"category"`
	I18nKey          string `bun:",notnull,default:''" json:"i18n_key"`
	RequiredHardware int64  `bun:",notnull,default:0" json:"required_hardware"`
	ExcludeHardware  int64  `bun:",notnull,default:0" json:"exclude_hardware"`
	MaxReplica       int    `bun:",notnull,default:0" json:"max_replica"`
	times
}

func (s *scenarioConstraintStoreImpl) FindByScenario(ctx context.Context, scenario string) (*ScenarioConstraint, error) {
	var result ScenarioConstraint
	err := s.db.Operator.Core.NewSelect().Model(&result).
		Where("scenario = ?", scenario).
		Scan(ctx)
	if err != nil {
		// no row for this scenario is not an error: it means the scenario has
		// no configured constraint, so callers treat it as "no rule" (nil).
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &result, nil
}

func (s *scenarioConstraintStoreImpl) FindByCode(ctx context.Context, code int) (*ScenarioConstraint, error) {
	var result ScenarioConstraint
	err := s.db.Operator.Core.NewSelect().Model(&result).
		Where("code = ?", code).
		Scan(ctx)
	if err != nil {
		// no row for this code is not an error: it means the code is not a known
		// scenario, so callers treat it as "no scenario" (nil) and skip
		// scenario filtering.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &result, nil
}

func (s *scenarioConstraintStoreImpl) FindAll(ctx context.Context) ([]ScenarioConstraint, error) {
	var result []ScenarioConstraint
	err := s.db.Operator.Core.NewSelect().Model(&result).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (s *scenarioConstraintStoreImpl) FindAllOrdered(ctx context.Context) ([]ScenarioConstraint, error) {
	var result []ScenarioConstraint
	err := s.db.Operator.Core.NewSelect().Model(&result).Order("code ASC").Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (s *scenarioConstraintStoreImpl) Upsert(ctx context.Context, input ScenarioConstraint) (*ScenarioConstraint, error) {
	_, err := s.db.Core.NewInsert().Model(&input).
		On("CONFLICT (scenario) DO UPDATE SET code = EXCLUDED.code, category = EXCLUDED.category, i18n_key = EXCLUDED.i18n_key, required_hardware = EXCLUDED.required_hardware, exclude_hardware = EXCLUDED.exclude_hardware, max_replica = EXCLUDED.max_replica, updated_at = CURRENT_TIMESTAMP").
		Exec(ctx, &input)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("scenario", input.Scenario))
	}

	return &input, nil
}

func (s *scenarioConstraintStoreImpl) Delete(ctx context.Context, scenario string) error {
	_, err := s.db.Core.NewDelete().Model((*ScenarioConstraint)(nil)).Where("scenario = ?", scenario).Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("scenario", scenario))
	}
	return nil
}
