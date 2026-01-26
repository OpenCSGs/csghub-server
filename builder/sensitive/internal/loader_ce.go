//go:build !saas && !ee

package internal

import (
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
)

// Database loads sensitive words from database
type databaseImpl struct {
	Base
}

// FromDatabase creates a new database-based loader
func FromDatabase() Loader {
	return &databaseImpl{}
}

func FromDatabaseWithParam(db database.SensitiveWordSetStore, interval time.Duration) *databaseImpl {
	return &databaseImpl{}
}

// Load loads sensitive words from database
func (dl *databaseImpl) Load() (*SensitiveWordData, error) {
	slog.Info("no implement for loading sensitive words from database")
	data := &SensitiveWordData{
		TagMap: make(map[int]string),
		Words:  make([]string, 0),
	}
	return data, nil
}
