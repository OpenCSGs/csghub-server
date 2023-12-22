package database

import (
	"database/sql"
	"fmt"
	"time"
)

type times struct {
	CreatedAt time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
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
		err = fmt.Errorf("retriving affected row count: %w", err)
		return err
	}
	if affected != X {
		err = fmt.Errorf("affected %d row(s), want %d", affected, X)
		return err
	}

	return nil
}
