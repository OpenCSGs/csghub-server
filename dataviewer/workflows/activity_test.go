package workflows

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"

	mockGit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockS3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
)

func NewTestDataViewerActivity(cfg *config.Config, gs gitserver.GitServer, s3Client s3.Client, viewerStore database.DataviewerStore) (DataViewerActivity, error) {
	return &dataViewerActivityImpl{
		gitServer:   gs,
		s3Client:    s3Client,
		cfg:         cfg,
		viewerStore: viewerStore,
	}, nil
}

func TestActivity_GetCardFromReadme(t *testing.T) {
	ctx := context.TODO()

	config := &config.Config{}
	dvstore := mockdb.NewMockDataviewerStore(t)
	mockGitServer := mockGit.NewMockGitServer(t)
	s3Client := mockS3.NewMockClient(t)

	req := types.UpdateViewerReq{
		Namespace: "testns",
		Name:      "testname",
		Branch:    "main",
		RepoType:  types.DatasetRepo,
		RepoID:    int64(1),
	}

	// configs:
	// - config_name: "foo"
	//   data_files:
	//   - split: train
	// 	   path: foo/..
	// - config_name: "bar"
	//   data_files:
	//   - split: train
	// 	   path: bar/..

	mockGitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Branch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  req.RepoType,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		Content:         "LS0tCmNvbmZpZ3M6Ci0gY29uZmlnX25hbWU6ICJmb28iCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgogICAgcGF0aDogZm9vLy4qCi0gY29uZmlnX25hbWU6ICJiYXIiCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgpwYXRoOiBiYXIvLioKLS0tCg==",
	}, nil)

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	card, err := dvActivity.GetCardFromReadme(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, card)
	require.Equal(t, "foo", card.Configs[0].ConfigName)
	require.Equal(t, "bar", card.Configs[1].ConfigName)
}

func TestActivity_ScanRepoFiles(t *testing.T) {
	ctx := context.TODO()

	config := &config.Config{}
	dvstore := mockdb.NewMockDataviewerStore(t)
	mockGitServer := mockGit.NewMockGitServer(t)
	s3Client := mockS3.NewMockClient(t)

	req := types.UpdateViewerReq{
		Namespace: "testns",
		Name:      "testname",
		Branch:    "main",
		RepoType:  types.DatasetRepo,
		RepoID:    int64(1),
	}

	mockGitServer.EXPECT().GetRepoFileTree(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Branch,
		RepoType:  req.RepoType,
	}).Return(
		[]*types.File{
			{Name: "foobar.parquet", Path: "foo/foobar.parquet"},
		}, nil,
	)

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	cls, err := dvActivity.ScanRepoFiles(ctx, dvCom.ScanRepoFileReq{
		Req:         req,
		MaxFileSize: config.DataViewer.MaxFileSize,
	})
	require.Nil(t, err)
	require.NotNil(t, cls)
	require.Equal(t, 1, len(cls.AllFiles))
	require.Equal(t, 1, len(cls.ParquetFiles))
	require.Equal(t, 0, len(cls.JsonlFiles))
	require.Equal(t, 0, len(cls.CsvFiles))
}

func TestActivity_DetermineCardData(t *testing.T) {
	ctx := context.TODO()

	config := &config.Config{}
	dvstore := mockdb.NewMockDataviewerStore(t)
	mockGitServer := mockGit.NewMockGitServer(t)
	s3Client := mockS3.NewMockClient(t)

	card := dvCom.CardData{}

	repoFileClass := dvCom.RepoFilesClass{
		AllFiles: map[string]*types.File{
			"foo.parquet": {Name: "foo.parquet", Path: "train/foo.parquet"},
		},
		ParquetFiles: map[string]*types.File{
			"foo.parquet": {Name: "foo.parquet", Path: "train/foo.parquet"},
		},
		JsonlFiles: map[string]*types.File{},
		CsvFiles:   map[string]*types.File{},
	}

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	res, err := dvActivity.DetermineCardData(ctx,
		dvCom.DetermineCardReq{
			Card:         card,
			Class:        repoFileClass,
			RepoDataType: RepoParquetData,
		})
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, 1, len(res.Configs))
	require.Equal(t, "default", res.Configs[0].ConfigName)
}

func TestActivity_CheckIfNeedRebuild(t *testing.T) {
	ctx := context.TODO()

	config := &config.Config{}
	dvstore := mockdb.NewMockDataviewerStore(t)
	mockGitServer := mockGit.NewMockGitServer(t)
	s3Client := mockS3.NewMockClient(t)

	req := types.UpdateViewerReq{
		Namespace: "testns",
		Name:      "testname",
		Branch:    "main",
		RepoType:  types.DatasetRepo,
		RepoID:    int64(1),
	}

	card := dvCom.CardData{
		Configs: []dvCom.ConfigData{
			{
				ConfigName: "default",
				DataFiles: []dvCom.DataFiles{
					{
						Split: "train",
						Path:  "train/foo.parquet",
					},
				},
			},
		},
		DatasetInfos: []dvCom.DatasetInfo{
			{
				ConfigName: "default",
				Splits: []dvCom.Split{
					{
						Name: "train",
						Origins: []dvCom.FileObject{
							{
								RepoFile:   "foo.parquet",
								LastCommit: "abcdefg",
							},
						},
					},
				},
			},
		},
	}

	dvstore.EXPECT().GetViewerByRepoID(ctx, int64(1)).Return(&database.Dataviewer{
		DataviewerJob: &database.DataviewerJob{
			CardMD5: "123456",
		},
	}, nil)

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	res, err := dvActivity.CheckIfNeedRebuild(ctx,
		dvCom.CheckBuildReq{
			Req:  req,
			Card: card,
		})
	require.Nil(t, err)
	require.True(t, res)
}

func TestActivity_CreateParquetBranch(t *testing.T) {
	ctx := context.TODO()

	config := &config.Config{}
	dvstore := mockdb.NewMockDataviewerStore(t)
	mockGitServer := mockGit.NewMockGitServer(t)
	s3Client := mockS3.NewMockClient(t)

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	req := types.UpdateViewerReq{
		Namespace: "testns",
		Name:      "testname",
		Branch:    "main",
		RepoType:  types.DatasetRepo,
		RepoID:    int64(1),
	}

	mockGitServer.EXPECT().GetRepoBranchByName(ctx, gitserver.GetBranchReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       dvCom.ParquetBranch,
		RepoType:  req.RepoType,
	},
	).Return(nil, nil)

	mockGitServer.EXPECT().CreateRepoFile(&types.CreateFileReq{
		Username:  GitDefaultUserName,
		Email:     GitDefaultUserEmail,
		Message:   "create branch",
		Content:   base64.StdEncoding.EncodeToString([]byte(types.DatasetGitattributesContent)),
		NewBranch: dvCom.ParquetBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.GitattributesFileName,
		RepoType:  req.RepoType,
	}).Return(nil)

	res, err := dvActivity.CreateParquetBranch(ctx, req)
	require.Nil(t, err)
	require.Equal(t, dvCom.ParquetBranch, res)
}
