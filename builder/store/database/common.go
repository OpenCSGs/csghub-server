package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

type times struct {
	CreatedAt time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

func (t *times) BeforeAppendModel(ctx context.Context, query schema.Query) error {
	switch query.(type) {
	case *bun.UpdateQuery:
		q := query.(*bun.UpdateQuery)
		m := q.GetModel().Value()
		//skip update for Repository, and related assets
		if _, ok := m.(*Repository); ok {
			return nil
		}
		if _, ok := m.(*Model); ok {
			return nil
		}
		if _, ok := m.(*Dataset); ok {
			return nil
		}
		if _, ok := m.(*Code); ok {
			return nil
		}
		if _, ok := m.(*Space); ok {
			return nil
		}
		t.UpdatedAt = time.Now()
	}

	return nil
}

func assertAffectedOneRow(result sql.Result, err error) error {
	return assertAffectedXRows(1, result, err)
}

func assertAffectedXRows(X int64, result sql.Result, err error) error {
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("retrieving affected row count: %w", err)
		return err
	}
	if affected != X {
		err = fmt.Errorf("affected %d row(s), want %d", affected, X)
		return err
	}

	return nil
}
