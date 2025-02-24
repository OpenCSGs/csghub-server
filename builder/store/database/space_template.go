package database

import (
	"context"
	"fmt"
)

type SpaceTemplate struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	Type        string `bun:",notnull" json:"type"`
	Name        string `bun:",notnull" json:"name"`
	ShowName    string `bun:",notnull" json:"show_name"`
	Enable      bool   `bun:",notnull,default:false" json:"enable"`
	Path        string `bun:",notnull" json:"path"`
	DevMode     bool   `bun:",notnull,default:false" json:"dev_mode"`
	Port        int    `bun:",notnull" json:"port"`
	Secrets     string `bun:",nullzero" json:"secrets"`
	Variables   string `bun:",nullzero" json:"variables"`
	Description string `bun:",nullzero" json:"description"`
	times
}

type SpaceTemplateStore interface {
	Index(ctx context.Context) ([]SpaceTemplate, error)
	Create(ctx context.Context, input SpaceTemplate) (*SpaceTemplate, error)
	Update(ctx context.Context, input SpaceTemplate) (*SpaceTemplate, error)
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*SpaceTemplate, error)
	FindAllByType(ctx context.Context, templateType string) ([]SpaceTemplate, error)
	FindByName(ctx context.Context, templateType, templateName string) (*SpaceTemplate, error)
}

type spaceTemplateStoreImpl struct {
	db *DB
}

func NewSpaceTemplateStore() SpaceTemplateStore {
	return &spaceTemplateStoreImpl{db: defaultDB}
}

func NewSpaceTemplateStoreWithDB(db *DB) SpaceTemplateStore {
	return &spaceTemplateStoreImpl{db: db}
}

func (s *spaceTemplateStoreImpl) Index(ctx context.Context) ([]SpaceTemplate, error) {
	var result []SpaceTemplate
	_, err := s.db.Operator.Core.NewSelect().Model(&result).
		Order("type", "enable", "name").Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("select all space templates error: %w", err)
	}
	return result, nil
}

func (s *spaceTemplateStoreImpl) Create(ctx context.Context, input SpaceTemplate) (*SpaceTemplate, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create space template error: %w", err)
	}

	return &input, nil
}

func (s *spaceTemplateStoreImpl) Update(ctx context.Context, input SpaceTemplate) (*SpaceTemplate, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update space template by id %d error: %w", input.ID, err)
	}
	return &input, nil
}

func (s *spaceTemplateStoreImpl) Delete(ctx context.Context, id int64) error {
	var input SpaceTemplate
	input.ID = id
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete space template by id %d error: %w", id, err)
	}
	return nil
}

func (s *spaceTemplateStoreImpl) FindByID(ctx context.Context, id int64) (*SpaceTemplate, error) {
	var res SpaceTemplate
	res.ID = id
	_, err := s.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)
	if err != nil {
		return nil, fmt.Errorf("select space template by id %d error: %w", id, err)
	}
	return &res, err
}

func (s *spaceTemplateStoreImpl) FindAllByType(ctx context.Context, templateType string) ([]SpaceTemplate, error) {
	var result []SpaceTemplate
	_, err := s.db.Operator.Core.NewSelect().Model(&result).
		Where("type = ?", templateType).Where("enable = ?", true).
		Order("name").Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("select space %s enabled templates error: %w", templateType, err)
	}
	return result, nil
}

func (s *spaceTemplateStoreImpl) FindByName(ctx context.Context, templateType, templateName string) (*SpaceTemplate, error) {
	var result SpaceTemplate
	_, err := s.db.Core.NewSelect().Model(&result).
		Where("type = ? and name = ?", templateType, templateName).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("select space %s template %s error: %w", templateType, templateName, err)
	}
	return &result, err
}
