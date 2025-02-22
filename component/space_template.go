package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SpaceTemplateComponent interface {
	Index(ctx context.Context) ([]database.SpaceTemplate, error)
	Create(ctx context.Context, req *types.SpaceTemplateReq) (*database.SpaceTemplate, error)
	Update(ctx context.Context, req *types.UpdateSpaceTemplateReq) (*database.SpaceTemplate, error)
	Delete(ctx context.Context, id int64) error
	FindAllByType(ctx context.Context, templateType string) ([]database.SpaceTemplate, error)
	FindByName(ctx context.Context, templateType, templateName string) (*database.SpaceTemplate, error)
}

func NewSpaceTemplateComponent(config *config.Config) (SpaceTemplateComponent, error) {
	c := &spaceTemplateComponentImpl{}
	c.spaceTemplateStore = database.NewSpaceTemplateStore()
	return c, nil
}

type spaceTemplateComponentImpl struct {
	spaceTemplateStore database.SpaceTemplateStore
}

func (c *spaceTemplateComponentImpl) Index(ctx context.Context) ([]database.SpaceTemplate, error) {
	templates, err := c.spaceTemplateStore.Index(ctx)
	if err != nil {
		return nil, fmt.Errorf("fail to get space template list error: %w", err)
	}
	return templates, nil
}

func (c *spaceTemplateComponentImpl) Create(ctx context.Context, req *types.SpaceTemplateReq) (*database.SpaceTemplate, error) {
	st := database.SpaceTemplate{
		Type:        req.Type,
		Name:        req.Name,
		ShowName:    req.ShowName,
		Enable:      req.Enable,
		Path:        req.Path,
		DevMode:     req.DevMode,
		Port:        req.Port,
		Secrets:     req.Secrets,
		Variables:   req.Variables,
		Description: req.Description,
	}
	res, err := c.spaceTemplateStore.Create(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("fail to create space template error: %w", err)
	}

	return res, nil
}

func (c *spaceTemplateComponentImpl) Update(ctx context.Context, req *types.UpdateSpaceTemplateReq) (*database.SpaceTemplate, error) {
	st, err := c.spaceTemplateStore.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("getting space template by id %d error: %w", req.ID, err)
	}

	if req.Type != nil {
		st.Type = *req.Type
	}
	if req.Name != nil {
		st.Name = *req.Name
	}
	if req.ShowName != nil {
		st.ShowName = *req.ShowName
	}
	if req.Enable != nil {
		st.Enable = *req.Enable
	}
	if req.Path != nil {
		st.Path = *req.Path
	}
	if req.DevMode != nil {
		st.DevMode = *req.DevMode
	}
	if req.Port != nil {
		st.Port = *req.Port
	}
	if req.Secrets != nil {
		st.Secrets = *req.Secrets
	}
	if req.Variables != nil {
		st.Variables = *req.Variables
	}
	if req.Description != nil {
		st.Description = *req.Description
	}

	res, err := c.spaceTemplateStore.Update(ctx, *st)
	if err != nil {
		return nil, fmt.Errorf("fail to update space template error: %w", err)
	}

	return res, nil
}

func (c *spaceTemplateComponentImpl) Delete(ctx context.Context, id int64) error {
	err := c.spaceTemplateStore.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("fail to delete space template by %d error: %w", id, err)
	}
	return nil
}

func (c *spaceTemplateComponentImpl) FindAllByType(ctx context.Context, templateType string) ([]database.SpaceTemplate, error) {
	res, err := c.spaceTemplateStore.FindAllByType(ctx, templateType)
	if err != nil {
		return nil, fmt.Errorf("fail to find templates by type %s error: %w", templateType, err)
	}
	return res, nil
}

func (c *spaceTemplateComponentImpl) FindByName(ctx context.Context, templateType, templateName string) (*database.SpaceTemplate, error) {
	res, err := c.spaceTemplateStore.FindByName(ctx, templateType, templateName)
	if err != nil {
		return nil, fmt.Errorf("fail to get %s template by %s error: %w", templateType, templateName, err)
	}
	return res, nil
}
