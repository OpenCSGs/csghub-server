package gitea

import (
	"testing"

	"opencsg.com/csghub-server/common/types"
)

func Test_repoPrefixByType(t *testing.T) {
	testData := map[types.RepositoryType]string{
		types.CodeRepo:    CodeOrgPrefix,
		types.SpaceRepo:   SpaceOrgPrefix,
		types.ModelRepo:   ModelOrgPrefix,
		types.DatasetRepo: DatasetOrgPrefix,
	}

	for repoType, prefix := range testData {
		if prefix != repoPrefixByType(repoType) {
			t.Fail()
		}
	}
}
