package database

import (
	"context"
	"encoding/json"
	"fmt"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type spaceResourceStoreImpl struct {
	db *DB
}

type SpaceResourceStore interface {
	Index(ctx context.Context, filter types.SpaceResourceFilter, per, page int) ([]SpaceResource, int, error)
	Create(ctx context.Context, input SpaceResource) (*SpaceResource, error)
	Update(ctx context.Context, input SpaceResource) (*SpaceResource, error)
	Delete(ctx context.Context, input SpaceResource) error
	FindByID(ctx context.Context, id int64) (*SpaceResource, error)
	FindByName(ctx context.Context, name string) (*SpaceResource, error)
	FindAll(ctx context.Context) ([]SpaceResource, error)
	FindAllResourceTypes(ctx context.Context, clusterId string) ([]string, error)
}

func NewSpaceResourceStore() SpaceResourceStore {
	return &spaceResourceStoreImpl{db: defaultDB}
}

func NewSpaceResourceStoreWithDB(db *DB) SpaceResourceStore {
	return &spaceResourceStoreImpl{db: db}
}

type SpaceResource struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	Name      string `bun:",notnull" json:"name"`
	Resources string `bun:",notnull" json:"resources"`
	ClusterID string `bun:",notnull" json:"cluster_id"`
	times
}

func (s *spaceResourceStoreImpl) Index(ctx context.Context, filter types.SpaceResourceFilter, per, page int) ([]SpaceResource, int, error) {
	var result []SpaceResource
	query := s.db.Operator.Core.NewSelect().Model(&result)
	if filter.ClusterID != "" {
		query = query.Where("cluster_id = ?", filter.ClusterID)
	}
	if filter.HardwareType != "" {
		query = query.Where("EXISTS (SELECT 1 FROM jsonb_each(resources::jsonb) WHERE value->>'type' = ?)", filter.HardwareType)
	}
	if filter.ResourceType != "" {
		query = query.Where("EXISTS (SELECT 1 FROM jsonb_each(resources::jsonb) WHERE key = ?)", filter.ResourceType)
	}
	query = query.Order("name asc").
		Limit(per).
		Offset((page - 1) * per)
	err := query.Scan(ctx, &result)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	err = errorx.HandleDBError(err, nil)
	return result, total, err
}

func (s *spaceResourceStoreImpl) Create(ctx context.Context, input SpaceResource) (*SpaceResource, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().
			Set("resource_name", "input.Name"))
		return nil, fmt.Errorf("create space resource in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *spaceResourceStoreImpl) Update(ctx context.Context, input SpaceResource) (*SpaceResource, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *spaceResourceStoreImpl) Delete(ctx context.Context, input SpaceResource) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *spaceResourceStoreImpl) FindByID(ctx context.Context, id int64) (*SpaceResource, error) {
	var res SpaceResource
	res.ID = id
	_, err := s.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)

	return &res, err
}

func (s *spaceResourceStoreImpl) FindByName(ctx context.Context, name string) (*SpaceResource, error) {
	var res SpaceResource
	err := s.db.Core.NewSelect().Model(&res).Where("name = ?", name).Scan(ctx)

	return &res, err
}

func (s *spaceResourceStoreImpl) FindAll(ctx context.Context) ([]SpaceResource, error) {
	var result []SpaceResource
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *spaceResourceStoreImpl) FindAllResourceTypes(ctx context.Context, clusterId string) ([]string, error) {
	typeSet := make(map[string]bool)
	var hardWareTypes []string
	filter := types.SpaceResourceFilter{ClusterID: clusterId}
	// Use pagination to query resources
	page := 1
	per := 100 // Set a reasonable page size

	for {
		// Get resources for current page
		resources, _, err := s.Index(ctx, filter, per, page)
		if err != nil {
			return nil, err
		}

		// If no resources returned, we've reached the end
		if len(resources) == 0 {
			break
		}

		// Process each resource
		for _, resource := range resources {
			var hw types.HardWare
			if err := json.Unmarshal([]byte(resource.Resources), &hw); err != nil {
				continue
			}

			// Extract type from each processor and CPU
			processors := []types.Processor{
				hw.Gpu,
				hw.Npu,
				hw.Gcu,
				hw.Mlu,
				hw.Dcu,
				hw.GPGpu,
			}

			for _, p := range processors {
				if p.Type != "" && !typeSet[p.Type] {
					typeSet[p.Type] = true
					hardWareTypes = append(hardWareTypes, p.Type)
				}
			}

			if hw.Cpu.Type != "" && !typeSet[hw.Cpu.Type] {
				typeSet[hw.Cpu.Type] = true
				hardWareTypes = append(hardWareTypes, hw.Cpu.Type)
			}
		}

		// Move to next page
		page++
	}

	return hardWareTypes, nil
}
