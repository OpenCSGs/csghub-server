package parquet

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetColumnsAndValuesWithSqlmock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.Nil(t, err)

	defer db.Close()

	uuid1 := uuid.New()
	uuid2 := uuid.New()

	columns := []string{"uuid", "name", "age"}
	rows := sqlmock.NewRows(columns).
		AddRow(uuid1, "Alice", 25).
		AddRow(uuid2, "Bob", 30)

	mock.ExpectQuery("SELECT.*").WillReturnRows(rows)

	realRows, err := db.Query("SELECT uuid, name, age FROM users")
	require.Nil(t, err)

	defer realRows.Close()

	reader := &duckdbReader{}

	resultColumns, resultTypes, values, err := reader.getColumnsAndValues(realRows)
	require.Nil(t, err)
	require.Equal(t, len(resultColumns), 3)
	require.Equal(t, len(values), 2)
	require.Equal(t, len(resultTypes), 3)
	require.Equal(t, values[0][0], uuid1.String())
	require.Equal(t, values[1][0], uuid2.String())
}
