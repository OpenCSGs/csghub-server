package database

import (
	"context"
)

type SensitiveWordSet struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	Name       string `bun:",notnull" json:"name"`
	ShowName   string `bun:",notnull" json:"show_name"`
	WordList   string `bun:",notnull" json:"word_list"`
	Enabled    bool   `bun:",notnull" json:"enabled"`
	CategoryID int64  `bun:",notnull" json:"category_id"`
	// many to one relation
	Category *SensitiveWordSetCategory `bun:"rel:belongs-to,join:category_id=id" json:"category"`

	times
}

type SensitiveWordSetCategory struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	Name     string `bun:",notnull" json:"name"`
	ShowName string `bun:",notnull" json:"show_name"`
}

type SensitiveWordSetStore interface {
	Create(ctx context.Context, input SensitiveWordSet) error
	Get(ctx context.Context, id int64) (*SensitiveWordSet, error)
	Update(ctx context.Context, input SensitiveWordSet) error
	Delete(ctx context.Context, id int64) error
	// List returns sensitive word sets matching the filter.
	// When per > 0 and page > 0, results are paginated by limit=per and
	// offset=(page-1)*per. total always reflects the count of all matching
	// rows (via SELECT count(*)), regardless of whether pagination is applied.
	// When per <= 0 or page <= 0, all matching rows are returned without
	// pagination.
	List(ctx context.Context, filter *SensitiveWordSetFilter, per, page int) ([]SensitiveWordSet, int, error)
	GetByName(ctx context.Context, name string) (*SensitiveWordSet, error)
}

type SensitiveWordSetFilter struct {
	search  *string
	enabled *bool
}

func NewSensitiveWordSetFilter() *SensitiveWordSetFilter {
	return &SensitiveWordSetFilter{}
}

func (f *SensitiveWordSetFilter) GetSearch() (string, bool) {
	if f.search == nil {
		return "", false
	}
	return *f.search, true
}

func (f *SensitiveWordSetFilter) GetEnabled() (bool, bool) {
	if f.enabled == nil {
		return false, false
	}
	return *f.enabled, true
}

func (f *SensitiveWordSetFilter) Search(s string) *SensitiveWordSetFilter {
	if len(s) == 0 {
		return f
	}
	f.search = &s
	return f
}

func (f *SensitiveWordSetFilter) Enabled(b bool) *SensitiveWordSetFilter {
	f.enabled = &b
	return f
}
