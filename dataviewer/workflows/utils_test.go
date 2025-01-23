package workflows

import (
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
	splitFiles := []string{"a/1.parquet", "b/2.parquet"}
	sortKeys := []string{"a", "b"}

	targetFiles := map[string]*types.File{
		"a/1.parquet": {
			Path: "a/1.parquet",
		},
		"b/2.parquet": {
			Path: "b/2.parquet",
		},
		"c/3.parquet": {
			Path: "c/3.parquet",
		},
	}

	res := ConvertRealFiles(splitFiles, sortKeys, targetFiles, "default", "train")
	require.Equal(t, 2, len(res))
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
