package database

import (
	"context"
	"regexp"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/types"
)

// InferenceArch represents the allowed inference architectures configuration in database
type InferenceArch struct {
	ID        int       `bun:",pk,autoincrement" json:"id"`
	Patterns  string    `bun:",notnull,default:''" json:"patterns"` // Multiple regex patterns separated by newlines
	CreatedAt time.Time `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",notnull,default:current_timestamp" json:"updated_at"`
}

type inferenceArchStoreImpl struct {
	db *DB
}

type InferenceArchStore interface {
	GetInferenceArch(ctx context.Context) (*InferenceArch, error)
	UpdateInferenceArch(ctx context.Context, req *types.CreateInferenceArchReq) (*InferenceArch, error)
	IsAllowed(ctx context.Context, archs []string) (bool, error)
}

func NewInferenceArchStore() InferenceArchStore {
	return &inferenceArchStoreImpl{
		db: defaultDB,
	}
}

func NewInferenceArchStoreWithDB(db *DB) InferenceArchStore {
	return &inferenceArchStoreImpl{
		db: db,
	}
}

// GetInferenceArch gets the inference arch configuration
func (s *inferenceArchStoreImpl) GetInferenceArch(ctx context.Context) (*InferenceArch, error) {
	var arch InferenceArch
	err := s.db.Core.NewSelect().Model(&arch).OrderExpr("id DESC").Limit(1).Scan(ctx, &arch)
	if err != nil {
		// If no record found, return a default one
		return &InferenceArch{
			Patterns: "",
		}, nil
	}

	// If no record found, return a default one
	if arch.ID == 0 {
		return &InferenceArch{
			Patterns: "",
		}, nil
	}

	return &arch, nil
}

// UpdateInferenceArch updates the inference arch configuration
func (s *inferenceArchStoreImpl) UpdateInferenceArch(ctx context.Context, req *types.CreateInferenceArchReq) (*InferenceArch, error) {
	// First, check if there's already a record
	existing, err := s.GetInferenceArch(ctx)
	if err != nil {
		return nil, err
	}

	if existing.ID > 0 {
		// Update existing record
		_, err = s.db.Core.NewUpdate().Model((*InferenceArch)(nil)).Set("patterns = ?", req.Patterns).Where("id = ?", existing.ID).Exec(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		// Create new record
		arch := &InferenceArch{
			Patterns: req.Patterns,
		}
		_, err = s.db.Core.NewInsert().Model(arch).Exec(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Get the updated record
	updated, err := s.GetInferenceArch(ctx)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// IsAllowed checks if the given architecture is allowed
func (s *inferenceArchStoreImpl) IsAllowed(ctx context.Context, archs []string) (bool, error) {
	arch, err := s.GetInferenceArch(ctx)
	if err != nil {
		return false, err
	}

	// If no patterns, all architectures are allowed
	if arch.Patterns == "" {
		return true, nil
	}

	// Split patterns by newlines
	patterns := strings.Split(arch.Patterns, "\n")

	for _, arch := range archs {
		// Check each pattern
		for _, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				// Skip invalid regex
				continue
			}

			if re.MatchString(arch) {
				// If any pattern matches, return false (not allowed)
				return false, nil
			}
		}
	}

	// If no patterns match, return true (allowed)
	return true, nil
}

// ToTypes converts database.InferenceArch to types.InferenceArch
func (arch *InferenceArch) ToTypes() *types.InferenceArch {
	return &types.InferenceArch{
		ID:        arch.ID,
		Patterns:  arch.Patterns,
		CreatedAt: arch.CreatedAt.Format(time.RFC3339),
		UpdatedAt: arch.UpdatedAt.Format(time.RFC3339),
	}
}
