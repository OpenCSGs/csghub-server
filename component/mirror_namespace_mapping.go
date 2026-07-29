package component

import (
	"context"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type mirrorNamespaceMappingComponentImpl struct {
	mirrorNamespaceMappingStore database.MirrorNamespaceMappingStore
	userStore                   database.UserStore
	namespaceStore              database.NamespaceStore
}

type MirrorNamespaceMappingComponent interface {
	Create(ctx context.Context, req types.CreateMirrorNamespaceMappingReq) (*database.MirrorNamespaceMapping, error)
	Get(ctx context.Context, id int64) (*database.MirrorNamespaceMapping, error)
	Index(ctx context.Context, search string) ([]database.MirrorNamespaceMapping, error)
	Update(ctx context.Context, req types.UpdateMirrorNamespaceMappingReq) (*database.MirrorNamespaceMapping, error)
	Delete(ctx context.Context, id int64) error
}

func NewMirrorNamespaceMappingComponent(config *config.Config) (MirrorNamespaceMappingComponent, error) {
	return &mirrorNamespaceMappingComponentImpl{
		mirrorNamespaceMappingStore: database.NewMirrorNamespaceMappingStore(),
		userStore:                   database.NewUserStore(),
		namespaceStore:              database.NewNamespaceStore(),
	}, nil
}

// Create validates and persists a namespace mapping with a canonical local target.
func (c *mirrorNamespaceMappingComponentImpl) Create(ctx context.Context, req types.CreateMirrorNamespaceMappingReq) (*database.MirrorNamespaceMapping, error) {
	targetNamespace := strings.ToLower(strings.TrimSpace(req.TargetNamespace))
	exists, err := c.namespaceStore.Exists(ctx, targetNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check target namespace existence, error: %w", err)
	}
	if !exists {
		return nil, errorx.TargetNamespaceNotFound(targetNamespace)
	}

	var mnm database.MirrorNamespaceMapping
	mnm.SourceNamespace = req.SourceNamespace
	mnm.TargetNamespace = targetNamespace
	if req.Enabled != nil {
		mnm.Enabled = req.Enabled
	}
	res, err := c.mirrorNamespaceMappingStore.Create(ctx, &mnm)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror namespace mapping, error: %w", err)
	}
	return res, nil
}

func (c *mirrorNamespaceMappingComponentImpl) Get(ctx context.Context, id int64) (*database.MirrorNamespaceMapping, error) {
	mnm, err := c.mirrorNamespaceMappingStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror namespace mapping, error: %w", err)
	}
	return mnm, nil
}

func (c *mirrorNamespaceMappingComponentImpl) Index(ctx context.Context, search string) ([]database.MirrorNamespaceMapping, error) {
	mnm, err := c.mirrorNamespaceMappingStore.Index(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror namespace mapping, error: %w", err)
	}
	return mnm, nil
}

// Update validates and persists changes to an existing namespace mapping.
func (c *mirrorNamespaceMappingComponentImpl) Update(ctx context.Context, req types.UpdateMirrorNamespaceMappingReq) (*database.MirrorNamespaceMapping, error) {
	var targetNamespace string
	if req.TargetNamespace != nil {
		targetNamespace = strings.ToLower(strings.TrimSpace(*req.TargetNamespace))
		exists, err := c.namespaceStore.Exists(ctx, targetNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to check target namespace existence, error: %w", err)
		}
		if !exists {
			return nil, errorx.TargetNamespaceNotFound(targetNamespace)
		}
	}

	var mnm database.MirrorNamespaceMapping
	mnm.ID = req.ID
	if req.SourceNamespace != nil {
		mnm.SourceNamespace = *req.SourceNamespace
	}
	if req.TargetNamespace != nil {
		mnm.TargetNamespace = targetNamespace
	}
	if req.Enabled != nil {
		mnm.Enabled = req.Enabled
	}
	mnm, err := c.mirrorNamespaceMappingStore.Update(ctx, &mnm)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror namespace mapping, error: %w", err)
	}
	return &mnm, nil
}

func (c *mirrorNamespaceMappingComponentImpl) Delete(ctx context.Context, id int64) error {
	mnm, err := c.mirrorNamespaceMappingStore.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find mirror namespace mapping, error: %w", err)
	}
	err = c.mirrorNamespaceMappingStore.Delete(ctx, mnm)
	if err != nil {
		return fmt.Errorf("failed to delete mirror namespace mapping, error: %w", err)
	}
	return nil
}
