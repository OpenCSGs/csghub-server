package workflows

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func TestUtils_GetPatternFileList(t *testing.T) {
	path := interface{}([]string{"a", "b"})

	output := GetPatternFileList(path)

	require.Equal(t, 2, len(output))

	path = interface{}("c")

	output = GetPatternFileList(path)

	require.Equal(t, 1, len(output))
}

func TestUtils_ConvertRealFiles(t *testing.T) {
	exists := map[string]*dvCom.RepoFile{}
	paths := []string{
		"foo/a.csv",
		"foo/b.csv",
		"foo/a.json",
		"bar/c.csv",
		"bar/d.csv",
		"bar/a.json",
		"foo/v1/e.csv",
		"foo/v2/f.csv",
		"foo/v1/t1/g.csv",
	}
	for _, path := range paths {
		exists[path] = &dvCom.RepoFile{File: &types.File{Path: path}}
	}
	// not exists files
	paths = append(paths, "foo/zz.csv")
	paths = append(paths, "bar/qq.csv")

	cases := []struct {
		split    string
		expected []string
	}{
		{split: "foobar/a.csv", expected: []string{}},
		{split: "foo/a.csv", expected: []string{"foo/a.csv"}},
		{split: "foo/*.csv", expected: []string{"foo/a.csv", "foo/b.csv"}},
		{split: "bar/*.csv", expected: []string{"bar/c.csv", "bar/d.csv"}},
		{split: "foo/**/*.csv", expected: []string{
			"foo/a.csv", "foo/b.csv", "foo/v1/e.csv", "foo/v2/f.csv",
			"foo/v1/t1/g.csv",
		}},
		{split: "bar/**/*.csv", expected: []string{"bar/c.csv", "bar/d.csv"}},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			match := ConvertRealFiles([]string{c.split}, paths, exists, "default", "test")
			paths := []string{}
			for _, f := range match {
				paths = append(paths, f.RepoFile)
			}
			require.Equal(t, c.expected, paths)
		})
	}
}

func TestUtils_GetCardDataMD5(t *testing.T) {
	card := dvCom.CardData{
		DatasetInfos: []dvCom.DatasetInfo{
			{
				ConfigName: "train",
				Splits: []dvCom.Split{
					{
						Origins: []dvCom.FileObject{
							{
								RepoFile:   "a/1.parquet",
								LastCommit: "abc",
							},
						},
					},
				},
			},
		},
	}

	out := GetCardDataMD5(card)

	require.Equal(t, "e096cecdbd943760b46ac073b6fd8d24", out)
}

func TestUtils_GetThreadNum(t *testing.T) {
	t.Run("min thread num", func(t *testing.T) {
		num := GetThreadNum(0, 2)
		require.Equal(t, 1, num)
	})

	t.Run("normal thread num", func(t *testing.T) {
		num := GetThreadNum(DataSizePerThread*4, 8)
		require.Equal(t, 4, num)
	})

	t.Run("max thread num", func(t *testing.T) {
		num := GetThreadNum(DataSizePerThread*10, 8)
		require.Equal(t, 8, num)
	})

}

func TestUtils_CopyFileContext(t *testing.T) {
	tempFile := "/tmp/write_tmp_line_file"

	t.Run("whole file", func(t *testing.T) {
		writeFile, err := os.Create(tempFile)
		require.Nil(t, err)

		defer os.Remove(tempFile)

		reader := io.NopCloser(strings.NewReader("it is not possible.\nIf it is odd."))

		len, err := CopyFileContext(writeFile, reader, 100)
		require.Nil(t, err)
		require.Equal(t, int64(34), len)

		data, err := os.ReadFile(tempFile)

		require.Nil(t, err)
		require.Equal(t, "it is not possible.\nIf it is odd.\n", string(data))
	})

	t.Run("partial file", func(t *testing.T) {
		writeFile, err := os.Create(tempFile)
		require.Nil(t, err)

		defer os.Remove(tempFile)

		reader := io.NopCloser(strings.NewReader("it is not possible.\nIf it is odd."))

		len, err := CopyFileContext(writeFile, reader, 20)
		require.Nil(t, err)
		require.Equal(t, int64(20), len)

		data, err := os.ReadFile(tempFile)

		require.Nil(t, err)
		require.Equal(t, "it is not possible.\n", string(data))
	})

}

func TestUtils_CopyJsonArray(t *testing.T) {
	tempFile := "/tmp/write_tmp_json_file"

	t.Run("whole file", func(t *testing.T) {
		writeFile, err := os.Create(tempFile)
		require.Nil(t, err)

		defer os.Remove(tempFile)

		reader := io.NopCloser(strings.NewReader("[{\"a\": 1}, {\"b\": 2}]"))

		len, err := CopyJsonArray(writeFile, reader, 100)
		require.Nil(t, err)
		require.Equal(t, int64(16), len)

		data, err := os.ReadFile(tempFile)
		require.Nil(t, err)
		require.Equal(t, "[{\"a\":1}\n,{\"b\":2}\n]", string(data))
	})

	t.Run("partial file", func(t *testing.T) {
		writeFile, err := os.Create(tempFile)
		require.Nil(t, err)

		defer os.Remove(tempFile)

		reader := io.NopCloser(strings.NewReader("[{\"a\": 1}, {\"b\": 2}]"))

		len, err := CopyJsonArray(writeFile, reader, 5)
		require.Nil(t, err)
		require.Equal(t, int64(8), len)

		data, err := os.ReadFile(tempFile)
		require.Nil(t, err)
		require.Equal(t, "[{\"a\":1}\n]", string(data))
	})

}
