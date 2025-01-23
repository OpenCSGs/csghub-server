package workflows

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func TestRepoFiles_appendFile(t *testing.T) {
	file := &types.File{
		Name: "test.jsonl",
		Size: 101,
	}

	fileClass := dvCom.RepoFilesClass{
		AllFiles:     make(map[string]*types.File),
		ParquetFiles: make(map[string]*types.File),
		JsonlFiles:   make(map[string]*types.File),
		CsvFiles:     make(map[string]*types.File),
	}

	appendFile(file, &fileClass, 100)

	require.Equal(t, 1, len(fileClass.AllFiles))
	require.Equal(t, 0, len(fileClass.JsonlFiles))
}
