package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type RuntimeArchitecturesStore struct {
	db *DB
}

func NewRuntimeArchitecturesStore() *RuntimeArchitecturesStore {
	return &RuntimeArchitecturesStore{
		db: defaultDB,
	}
}

type RuntimeArchitecture struct {
	ID                 int64  `bun:",pk,autoincrement" json:"id"`
	RuntimeFrameworkID int64  `bun:",notnull" json:"runtime_framework_id"`
	ArchitectureName   string `bun:",notnull" json:"architecture_name"`
}

func (ra *RuntimeArchitecturesStore) ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("runtime_framework_id = ?", id).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}

func (ra *RuntimeArchitecturesStore) Add(ctx context.Context, arch RuntimeArchitecture) error {
	res, err := ra.db.Core.NewInsert().Model(&arch).Exec(ctx, &arch)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("creating runtime architecture in the db failed,error:%w", err)
	}
	return nil
}

func (ra *RuntimeArchitecturesStore) DeleteByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) error {
	var arch RuntimeArchitecture
	_, err := ra.db.Core.NewDelete().Model(&arch).Where("runtime_framework_id = ? and architecture_name = ?", id, archName).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleteing runtime architecture in the db failed, error:%w", err)
	}
	return nil
}

func (ra *RuntimeArchitecturesStore) FindByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) (*RuntimeArchitecture, error) {
	var arch RuntimeArchitecture
	_, err := ra.db.Core.NewSelect().Model(&arch).Where("runtime_framework_id = ? and architecture_name = ?", id, archName).Exec(ctx, &arch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting runtime architecture in the db failed, error:%w", err)
	}
	return &arch, nil
}

func (ra *RuntimeArchitecturesStore) ListByRArchName(ctx context.Context, archName string) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("architecture_name = ?", archName).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}

func (ra *RuntimeArchitecturesStore) ListByRArchNameAndModel(ctx context.Context, archName, modelName string) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("architecture_name = ?", archName).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	result2, err := ra.GetRuntimeByModelName(ctx, archName, modelName)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture by model name, %w", err)
	}
	result = append(result, result2...)
	return result, nil
}

/**
@Description: get runtime architecture by model name
@param ctx
@param archName
@param modelName
@return []RuntimeArchitecture
------------      table resource_models         -----------------
resource_name | engine_name |            model_name
---------------+-------------+-----------------------------------
ascend        | mindie      | Baichuan-7B
ascend        | mindie      | Baichuan-13B
nvidia        | nim         | Llama-3.1-8B-Instruct
nvidia        | nim         | Llama-3-8B-Instruct

------------      table runtime_frameworks         -----------------
        frame_name         |                          frame_image                           |    frame_npu_image
---------------------------+----------------------------------------------------------------+------------------------
 TGI                       | tgi:2.1                                                        |
 VLLM                      | vllm-local:2.7                                                 | vllm-cpu:2.3
 MindIE                    |                                                                | mindie:1.8-csg-1.0.RC2
 nim-llama-3.1-8b-instruct | nvcr.io/nim/meta/llama-3.1-8b-instruct:latest                  |
 nim-llama-2-13b-chat      | nvcr.io/nim/meta/llama-2-13b-chat:latest                       |
 nim-llama3-8b-instruct    | nvcr.io/nim/meta/llama3-8b-instruct:latest                     |
case 1: mindie
all models share same runtime framework mindie, so we only need to get the runtime framework for mindie when the image contains mindie
case 2: nim
every llama model has its own runtime framework, so we need to get the runtime framework for each model
Meta-Llama-3-8B-Instruct --> llama3-8b-instruct
Llama-2-13b-chat --> llama-2-13b-chat
*/

func (ra *RuntimeArchitecturesStore) GetRuntimeByModelName(ctx context.Context, archName, modelName string) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	var resModel []ResourceModel
	err := ra.db.Core.NewSelect().Model(&resModel).Where("LOWER(model_name) like ? and engine_name != ?", fmt.Sprintf("%%%s%%", strings.ToLower(modelName)), "nim").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting resource model, %w", err)
	}
	var resNIMModel []ResourceModel
	nimModel := strings.Replace(strings.ToLower(modelName), "meta-", "", 1)
	err = ra.db.Core.NewSelect().Model(&resNIMModel).Where("LOWER(model_name) = ? and engine_name = ?", nimModel, "nim").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting resource model, %w", err)
	}
	resModel = append(resModel, resNIMModel...)
	var runtime_frameworks []RuntimeFramework
	// select all runtime_frameworks
	err = ra.db.Core.NewSelect().Model(&runtime_frameworks).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime frameworks, %w", err)
	}
	nimMatchModel := strings.ReplaceAll(nimModel, "-", "")
	for _, r := range resModel {
		//append to result if result dont' have the runtime_framework_id
		engineName := strings.ToLower(r.EngineName)

		for _, rf := range runtime_frameworks {
			image := strings.ToLower(rf.FrameImage)
			if strings.Contains(image, "/") {
				parts := strings.Split(image, "/")
				image = parts[len(parts)-1]
			}
			if strings.Contains(image, engineName) && !contains(result, rf.ID) {
				result = append(result, RuntimeArchitecture{
					RuntimeFrameworkID: rf.ID,
					ArchitectureName:   archName,
				})
				continue
			}
			// special handling for nim models
			nimImage := strings.ReplaceAll(image, "-", "")
			if strings.Contains(nimImage, nimMatchModel) && !contains(result, rf.ID) {
				result = append(result, RuntimeArchitecture{
					RuntimeFrameworkID: rf.ID,
					ArchitectureName:   archName,
				})
			}
		}

	}
	return result, nil
}

func contains(architectures []RuntimeArchitecture, id int64) bool {
	for _, arch := range architectures {
		if arch.RuntimeFrameworkID == id {
			return true
		}
	}
	return false
}
