package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type tagRuleStoreImpl struct {
	db *DB
}

type TagRuleStore interface {
	// find dataset tag by name and related tag with tag name
	FindByRepo(ctx context.Context, category, namespace, repoName, repoType string) (*TagRule, error)
	FindAllByRepo(ctx context.Context, category, namespace, repoName, repoType string) ([]TagRule, error)
	SyncEvaluationDatasets(
		ctx context.Context,
		runtimeFramework string,
		datasets []types.EvaluationDatasetConfig,
		prune bool,
	) error
}

func NewTagRuleStore() TagRuleStore {
	return &tagRuleStoreImpl{db: defaultDB}
}

func NewTagRuleStoreWithDB(db *DB) TagRuleStore {
	return &tagRuleStoreImpl{db: db}
}

// internal use for the relation between dataset and tag
type TagRule struct {
	ID               int64     `bun:",pk,autoincrement" json:"id"`
	Namespace        string    `bun:",notnull" json:"namespace"`
	RepoName         string    `bun:",notnull" json:"repo_name"`
	RepoType         string    `bun:",notnull" json:"repo_type"`
	Category         string    `bun:",notnull" json:"category"`
	TagName          string    `bun:",notnull" json:"tag_name"`
	RuntimeFramework string    `bun:"," json:"runtime_framework"`
	Source           string    `bun:"," json:"source"`
	Tag              Tag       `bun:",rel:has-one,join:tag_name=name,join:category=category"`
	CreatedAt        time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
}

// find dataset tag by name and related tag with tag name
func (s *tagRuleStoreImpl) FindByRepo(ctx context.Context, category, namespace, repoName, repoType string) (*TagRule, error) {
	var TagRule TagRule
	err := s.db.Core.NewSelect().
		Model(&TagRule).
		Relation("Tag").
		Where("LOWER(tag_rule.repo_name) = LOWER(?) AND tag_rule.namespace =? AND tag_rule.repo_type = ? AND tag_rule.category = ?", repoName, namespace, repoType, category).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().
			Set("category", category).
			Set("namespace", namespace).
			Set("repo_name", repoName).
			Set("repo_type", repoType))
	}
	return &TagRule, nil
}

func (s *tagRuleStoreImpl) FindAllByRepo(
	ctx context.Context,
	category, namespace, repoName, repoType string,
) ([]TagRule, error) {
	var tagRules []TagRule
	err := s.db.Core.NewSelect().
		Model(&tagRules).
		Relation("Tag").
		Where(
			"LOWER(tag_rule.repo_name) = LOWER(?) AND tag_rule.namespace = ? AND tag_rule.repo_type = ? AND tag_rule.category = ?",
			repoName,
			namespace,
			repoType,
			category,
		).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().
			Set("category", category).
			Set("namespace", namespace).
			Set("repo_name", repoName).
			Set("repo_type", repoType))
	}
	return tagRules, nil
}

func (s *tagRuleStoreImpl) SyncEvaluationDatasets(
	ctx context.Context,
	runtimeFramework string,
	datasets []types.EvaluationDatasetConfig,
	prune bool,
) error {
	if runtimeFramework == "" {
		return fmt.Errorf("runtime framework is required")
	}
	if len(datasets) == 0 {
		return fmt.Errorf("evaluation dataset rules for %s must not be empty", runtimeFramework)
	}
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		configured := make(map[string]struct{}, len(datasets))
		for _, dataset := range datasets {
			rule := &TagRule{
				Namespace:        dataset.Namespace,
				RepoName:         dataset.RepoName,
				RepoType:         dataset.RepoType,
				Category:         dataset.Category,
				TagName:          dataset.TagName,
				RuntimeFramework: runtimeFramework,
				Source:           dataset.Source,
			}
			_, err := tx.NewInsert().
				Model(rule).
				On("CONFLICT (namespace, repo_name, category, runtime_framework) DO UPDATE").
				Set("repo_type = EXCLUDED.repo_type").
				Set("tag_name = EXCLUDED.tag_name").
				Set("source = EXCLUDED.source").
				Exec(ctx)
			if err != nil {
				return err
			}
			configured[tagRuleConfigKey(rule.Namespace, rule.RepoName, rule.Category)] = struct{}{}
		}

		if !prune {
			return nil
		}

		var existing []TagRule
		err := tx.NewSelect().
			Model(&existing).
			Where("runtime_framework = ?", runtimeFramework).
			Scan(ctx)
		if err != nil {
			return err
		}
		for _, rule := range existing {
			if _, ok := configured[tagRuleConfigKey(rule.Namespace, rule.RepoName, rule.Category)]; ok {
				continue
			}
			if _, err = tx.NewDelete().Model((*TagRule)(nil)).Where("id = ?", rule.ID).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	return errorx.HandleDBError(err, errorx.Ctx().Set("runtime_framework", runtimeFramework))
}

func tagRuleConfigKey(namespace, repoName, category string) string {
	return namespace + "/" + repoName + "/" + category
}
