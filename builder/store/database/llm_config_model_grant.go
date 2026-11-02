package database

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/config"
)

// LLMConfigModelGrant is one access whitelist grant on an LLM config, stored
// per namespace: Namespace is the namespace path (a username for personal
// namespaces, the organization path for organizations), which is also the
// identity an API key carries. Requests carrying the other side's credentials
// do not match — an organization grant only accepts requests made with the
// organization's own API key, and vice versa.
type LLMConfigModelGrant struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	LLMConfigID int64  `bun:",notnull" json:"llm_config_id"`
	// NamespaceType is either "user" or "organization".
	NamespaceType string `bun:",notnull" json:"namespace_type"`
	// Namespace is the granted namespace path (the llm caller identity).
	Namespace string `bun:",notnull" json:"namespace"`
	// NamespaceUUID is the uuid of that namespace; the gateway compares the
	// authenticated caller's namespace UUID against it.
	NamespaceUUID string `bun:",notnull" json:"namespace_uuid"`
	Note          string `bun:",nullzero" json:"note"`
	// CreatedBy records the admin (username or uuid) who granted the access.
	CreatedBy string `bun:",nullzero" json:"created_by"`
	times
}

type LLMConfigModelGrantStore interface {
	Index(ctx context.Context, llmConfigID int64) ([]LLMConfigModelGrant, error)
	Create(ctx context.Context, auth LLMConfigModelGrant) (*LLMConfigModelGrant, error)
	Delete(ctx context.Context, llmConfigID int64, namespaceType, namespace string) error
	DeleteByLLMConfigID(ctx context.Context, llmConfigID int64) error
}

type lLMConfigModelGrantStoreImpl struct {
	db *DB
}

func NewLLMConfigModelGrantStore(cfg *config.Config) LLMConfigModelGrantStore {
	return &lLMConfigModelGrantStoreImpl{db: defaultDB}
}

func NewLLMConfigModelGrantStoreWithDB(db *DB, cfg *config.Config) LLMConfigModelGrantStore {
	return &lLMConfigModelGrantStoreImpl{db: db}
}

func (s *lLMConfigModelGrantStoreImpl) Index(ctx context.Context, llmConfigID int64) ([]LLMConfigModelGrant, error) {
	var authorizations []LLMConfigModelGrant
	err := s.db.Operator.Core.NewSelect().
		Model(&authorizations).
		Where("llm_config_id = ?", llmConfigID).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("index llm config model grants by config id %d, %w", llmConfigID, err)
	}
	return authorizations, nil
}

func (s *lLMConfigModelGrantStoreImpl) Create(ctx context.Context, auth LLMConfigModelGrant) (*LLMConfigModelGrant, error) {
	if _, err := s.db.Core.NewInsert().Model(&auth).On("CONFLICT (llm_config_id, namespace) DO NOTHING").Exec(ctx, &auth); err != nil {
		return nil, fmt.Errorf("create llm config model grant, %w", err)
	}
	return &auth, nil
}

func (s *lLMConfigModelGrantStoreImpl) Delete(ctx context.Context, llmConfigID int64, namespaceType, namespace string) error {
	_, err := s.db.Core.NewDelete().
		Model((*LLMConfigModelGrant)(nil)).
		Where("llm_config_id = ? AND namespace_type = ? AND namespace = ?", llmConfigID, namespaceType, namespace).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete llm config model grant %d/%s/%s, %w", llmConfigID, namespaceType, namespace, err)
	}
	return nil
}

func (s *lLMConfigModelGrantStoreImpl) DeleteByLLMConfigID(ctx context.Context, llmConfigID int64) error {
	_, err := s.db.Core.NewDelete().
		Model((*LLMConfigModelGrant)(nil)).
		Where("llm_config_id = ?", llmConfigID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete llm config model grants by config id %d, %w", llmConfigID, err)
	}
	return nil
}
