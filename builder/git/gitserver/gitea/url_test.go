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

func test_portalCloneUrl(t *testing.T) {
	httpCloneUrl := "https://gitdomain.com/datasets_2652/2652_dataset_02.git"
	expectedCloneUrl := "http://portaldomain.com/datasets/2652/2652_dataset_02.git"
	if portalCloneUrl(httpCloneUrl, types.DatasetRepo, "gitdomain.com", "portaldomain.com") != expectedCloneUrl {
		t.Fail()
	}
}
