package parquet_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"opencsg.com/csghub-server/builder/parquet"
)

func initReader() *parquet.ParquetGoReader {
	return parquet.NewParquetGoReader(&parquet.OSFileClient{}, otel.Tracer("test"), "")
}

// test data: 10 parquet files, each file contains 20 rows of data, row id start from 0
func TestGoReader_All(t *testing.T) {
	cases := []struct {
		limit         int64
		offset        int64
		expectedRange string
	}{
		{10, 0, "0-9"},
		{10, 10, "10-19"},
		{10, 18, "18-27"},
		{30, 18, "18-47"},
		{60, 185, "185-199"},
		{100, 75, "75-174"},
	}

	paths := []string{}
	for i := 0; i < 10; i++ {
		paths = append(paths, fmt.Sprintf("test_data/%d.parquet", i))
	}

	reader := initReader()
	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			columns, columnTypes, data, total, err := reader.RowsWithCount(
				context.TODO(),
				paths,
				c.limit,
				c.offset,
			)
			require.NoError(t, err)
			require.Equal(t, []string{"Id", "Name"}, columns)
			require.Equal(t, []string{"INT64", "INT64"}, columnTypes)
			require.Equal(t, int64(200), total)

			rg := strings.Split(c.expectedRange, "-")
			start := cast.ToInt64(rg[0])
			end := cast.ToInt64(rg[1])
			current := start
			for _, row := range data {
				id := cast.ToInt64(row[0])
				name := cast.ToInt64(row[1])
				require.Equal(t, current, id)
				require.Equal(t, current, name)
				current += 1
			}
			require.Equal(t, end+1, current)
		})
	}

}

func TestGoReader_UUID(t *testing.T) {
	paths := []string{}
	paths = append(paths, "test_data/uuid.parquet")

	reader := initReader()
	t.Run("test-uuid-column", func(t *testing.T) {
		columns, columnTypes, data, total, err := reader.RowsWithCount(
			context.TODO(),
			paths,
			2,
			0,
		)
		require.NoError(t, err)
		require.Equal(t, []string{"id", "image"}, columns)
		require.Equal(t, []string{"FIXED_LEN_BYTE_ARRAY", "BYTE_ARRAY"}, columnTypes)
		require.Equal(t, int64(2), total)

		realData := [][]string{
			{
				"0ad08b5c-35ba-4304-9eaa-1afb0554e459",
				"image.png",
			},
			{
				"1e4fe944-78ef-4fd5-abf5-cb4a4a20ac86",
				"21000.png",
			},
		}

		for idx, row := range data {
			id := cast.ToString(row[0])
			name := cast.ToString(row[1])
			require.Equal(t, realData[idx][0], id)
			require.Equal(t, realData[idx][1], name)
		}

	})

}
