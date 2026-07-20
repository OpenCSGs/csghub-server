package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// spaceResourceScenarioConstraint mirrors builder/store/database.ScenarioConstraint.
// It is declared here (in the migrations package) because createTables/dropTables
// build the DDL from a struct value via reflection, and the migrations package
// must not import the store package. The bun table name is pinned with the
// `bun:table` tag so it matches the store struct exactly.
//
// This table is the single source of truth for the scenario catalog (name, code,
// category, display_name) AND the per-scenario filtering constraints
// (required_hardware, max_replica). Adding a scenario only requires inserting a
// row here / at runtime via Upsert; no Go code change needed.
//
//   - Scenario: scenario name (matches common/types scenarioBit names, e.g.
//     "finetune", "sandbox", "wf_evaluation").
//   - Code: bit position. Deploy scenarios use the DeployType int values 0-7
//     (see common/types/model.go) so the deploy_type passed by callers doubles as
//     both a deploy task type and a scenario code. Workflow scenarios use 32-38.
//     The bitmask is `1 << Code`. Unique so one bit maps to one scenario.
//   - Category: "deploy" or "workflow" (see types.ScenarioCategory*).
//   - I18nKey: i18n key (e.g. "scenario.finetune") resolved by the frontend
//     via $t; the localized text lives in the frontend locale files.
//   - RequiredHardware: bitmask of hardware types the resource MUST have at
//     least one of ("at least" semantics). 0 means no "must have" requirement.
//   - ExcludeHardware: bitmask of hardware types the resource MUST NOT have
//     ("none of" semantics). 0 means nothing excluded. sandbox uses
//     HardwareMaskGraphic here to express "pure CPU, no accelerator" (since
//     required_hardware can only say "has CPU", not "only CPU").
//   - MaxReplica: max replicas allowed for this scenario. 0 means unlimited.
type spaceResourceScenarioConstraint struct {
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

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, spaceResourceScenarioConstraint{})
		if err != nil {
			return fmt.Errorf("create space_resource_scenario_constraints table failed: %w", err)
		}

		// Seed the full scenario catalog (8 deploy + 7 workflow = 15 rows).
		// Deploy codes mirror the DeployType constants (SpaceType=0 ... SandboxType=7)
		// so the deploy_type passed to the Index API doubles as a scenario code.
		// Workflow scenarios occupy bits 32-38. required_hardware/max_replica
		// mirror the previously hardcoded deployAvailable logic:
		//   - finetune/inference require a graphic accelerator (required = HardwareMaskGraphic);
		//   - sandbox must be pure CPU, expressed by excluding all graphic
		//     accelerators (exclude = HardwareMaskGraphic). required_hardware can
		//     only say "has CPU", not "only CPU", so exclude is used instead.
		//   - the rest have no hardware requirement (0 = any).
		seeds := []spaceResourceScenarioConstraint{
			// deploy scenarios (bits 0-7). I18nKey is an i18n key (scenario.<name>)
			// that the frontend resolves via $t; the localized text lives in the
			// frontend locale files, not here.
			{Scenario: types.ScenarioName(types.ScenarioSpace), Code: types.SpaceType, Category: string(types.ScenarioCategoryDeploy), I18nKey: "scenario.space", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioInference), Code: types.InferenceType, Category: string(types.ScenarioCategoryDeploy), I18nKey: "scenario.inference", RequiredHardware: int64(types.HardwareMaskGraphic), ExcludeHardware: 0, MaxReplica: 0},
			{Scenario: types.ScenarioName(types.ScenarioFinetune), Code: types.FinetuneType, Category: string(types.ScenarioCategoryDeploy), I18nKey: "scenario.finetune", RequiredHardware: int64(types.HardwareMaskGraphic), ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioServerless), Code: types.ServerlessType, Category: string(types.ScenarioCategoryDeploy), I18nKey: "scenario.serverless", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioNotebook), Code: types.NotebookType, Category: string(types.ScenarioCategoryDeploy), I18nKey: "scenario.notebook", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioSandbox), Code: types.SandboxType, Category: string(types.ScenarioCategoryDeploy), I18nKey: "scenario.sandbox", RequiredHardware: 0, ExcludeHardware: int64(types.HardwareMaskGraphic), MaxReplica: 1},
			// workflow scenarios (bits 32-38)
			{Scenario: types.ScenarioName(types.ScenarioWfEvaluation), Code: 32, Category: string(types.ScenarioCategoryWorkflow), I18nKey: "scenario.wf_evaluation", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioWfClawEval), Code: 33, Category: string(types.ScenarioCategoryWorkflow), I18nKey: "scenario.wf_claw_eval", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioWfFinetune), Code: 37, Category: string(types.ScenarioCategoryWorkflow), I18nKey: "scenario.wf_finetune", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
			{Scenario: types.ScenarioName(types.ScenarioWfDataflow), Code: 38, Category: string(types.ScenarioCategoryWorkflow), I18nKey: "scenario.wf_dataflow", RequiredHardware: 0, ExcludeHardware: 0, MaxReplica: 1},
		}
		_, err = db.NewInsert().Model(&seeds).Exec(ctx)
		if err != nil {
			return fmt.Errorf("seed space_resource_scenario_constraints failed: %w", err)
		}
		fmt.Println("Insert data successfully")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		err := dropTables(ctx, db, spaceResourceScenarioConstraint{})
		if err != nil {
			return fmt.Errorf("drop space_resource_scenario_constraints table failed: %w", err)
		}
		return nil
	})
}
