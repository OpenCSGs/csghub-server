package workflows

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/mcuadros/go-defaults"
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
	defaults.SetDefaults(config)

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

	mockGitServer.EXPECT().GetTree(mock.Anything, types.GetTreeRequest{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Branch,
		RepoType:  req.RepoType,
		Limit:     500,
		Recursive: true,
	}).Return(
		&types.GetRepoFileTreeResp{
			Files: []*types.File{
				{Name: "foobar.parquet", Path: "foo/foobar.parquet"},
			}}, nil,
	)

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	cls, err := dvActivity.ScanRepoFiles(ctx, dvCom.ScanRepoFileReq{
		Req:              req,
		ConvertLimitSize: config.DataViewer.ConvertLimitSize,
		MaxFileNum:       config.DataViewer.ScanFileNumLimit,
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
		AllFiles: map[string]*dvCom.RepoFile{
			"foo.parquet": {
				File: &types.File{
					Name: "foo.parquet",
					Path: "train/foo.parquet",
				},
			},
		},
		ParquetFiles: map[string]*dvCom.RepoFile{
			"foo.parquet": {
				File: &types.File{
					Name: "foo.parquet",
					Path: "train/foo.parquet",
				},
			},
		},
		JsonlFiles: map[string]*dvCom.RepoFile{},
		CsvFiles:   map[string]*dvCom.RepoFile{},
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

	mockGitServer.EXPECT().RepositoryExists(ctx, gitserver.CheckRepoReq{
		RepoType:  types.RepositoryType(config.RepoTemplate.EmptyRepoType),
		Namespace: config.RepoTemplate.EmptyNameSpace,
		Name:      config.RepoTemplate.EmptyRepoName,
	}).Return(true, nil)

	mockGitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: config.RepoTemplate.EmptyNameSpace,
		Name:      config.RepoTemplate.EmptyRepoName,
		RepoType:  types.RepositoryType(config.RepoTemplate.EmptyRepoType),
		Ref:       types.MainBranch,
	}).Return(&types.Commit{}, nil)

	mockGitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Username:    GitDefaultUserName,
		Email:       GitDefaultUserEmail,
		Message:     "update gitattributes file in new branch refs-convert-parquet",
		Content:     base64.StdEncoding.EncodeToString([]byte(types.DatasetGitattributesContent)),
		NewBranch:   dvCom.ParquetBranch,
		Branch:      dvCom.ParquetBranch,
		Namespace:   req.Namespace,
		Name:        req.Name,
		FilePath:    types.GitattributesFileName,
		RepoType:    req.RepoType,
		StartBranch: "main",
	}).Return(nil)

	res, err := dvActivity.CreateParquetBranch(ctx, req)
	require.Nil(t, err)
	require.Equal(t, dvCom.ParquetBranch, res)
}

func TestActivity_CalcStatistics(t *testing.T) {
	config := &config.Config{}
	dvstore := mockdb.NewMockDataviewerStore(t)
	mockGitServer := mockGit.NewMockGitServer(t)
	s3Client := mockS3.NewMockClient(t)

	dvActivity, err := NewTestDataViewerActivity(config, mockGitServer, s3Client, dvstore)
	require.Nil(t, err)

	t.Run("basic statistics calculation", func(t *testing.T) {
		cardReq := &dvCom.UpdateCardReq{
			FinalCardData: dvCom.CardData{
				DatasetInfos: []dvCom.DatasetInfo{
					{
						ConfigName: "config1",
						Splits: []dvCom.Split{
							{
								Name:        "train",
								NumExamples: 1000,
								Files: []dvCom.FileObject{
									{Size: 1024},
									{Size: 2048},
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 4096},
									{DownloadSize: 8192},
								},
							},
							{
								Name:        "test",
								NumExamples: 500,
								Files: []dvCom.FileObject{
									{Size: 512},
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 1024},
								},
							},
						},
					},
					{
						ConfigName: "config2",
						Splits: []dvCom.Split{
							{
								Name:        "validation",
								NumExamples: 300,
								Files: []dvCom.FileObject{
									{Size: 256},
									{Size: 128},
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 512},
									{DownloadSize: 256},
								},
							},
						},
					},
				},
			},
		}

		dvActivity.CalcStatistics(cardReq)

		require.Equal(t, int64(1800), cardReq.FinalCardData.Rows_Num)         // 1000 + 500 + 300
		require.Equal(t, int64(3968), cardReq.FinalCardData.Converted_Size)   // 1024+2048+512+256+128
		require.Equal(t, int64(14080), cardReq.FinalCardData.Downloaded_Size) // 4096+8192+1024+512+256
	})

	t.Run("empty dataset", func(t *testing.T) {
		cardReq := &dvCom.UpdateCardReq{
			FinalCardData: dvCom.CardData{
				DatasetInfos: []dvCom.DatasetInfo{},
			},
		}

		dvActivity.CalcStatistics(cardReq)

		require.Equal(t, int64(0), cardReq.FinalCardData.Rows_Num)
		require.Equal(t, int64(0), cardReq.FinalCardData.Converted_Size)
		require.Equal(t, int64(0), cardReq.FinalCardData.Downloaded_Size)
	})

	t.Run("only files without origins", func(t *testing.T) {
		cardReq := &dvCom.UpdateCardReq{
			FinalCardData: dvCom.CardData{
				DatasetInfos: []dvCom.DatasetInfo{
					{
						ConfigName: "config1",
						Splits: []dvCom.Split{
							{
								Name:        "train",
								NumExamples: 100,
								Files: []dvCom.FileObject{
									{Size: 1024},
									{Size: 2048},
								},
								Origins: []dvCom.FileObject{},
							},
						},
					},
				},
			},
		}

		dvActivity.CalcStatistics(cardReq)

		require.Equal(t, int64(100), cardReq.FinalCardData.Rows_Num)
		require.Equal(t, int64(3072), cardReq.FinalCardData.Converted_Size) // 1024+2048
		require.Equal(t, int64(0), cardReq.FinalCardData.Downloaded_Size)
	})

	t.Run("only origins without files", func(t *testing.T) {
		cardReq := &dvCom.UpdateCardReq{
			FinalCardData: dvCom.CardData{
				DatasetInfos: []dvCom.DatasetInfo{
					{
						ConfigName: "config1",
						Splits: []dvCom.Split{
							{
								Name:        "train",
								NumExamples: 200,
								Files:       []dvCom.FileObject{},
								Origins: []dvCom.FileObject{
									{DownloadSize: 4096},
									{DownloadSize: 8192},
								},
							},
						},
					},
				},
			},
		}

		dvActivity.CalcStatistics(cardReq)

		require.Equal(t, int64(200), cardReq.FinalCardData.Rows_Num)
		require.Equal(t, int64(0), cardReq.FinalCardData.Converted_Size)
		require.Equal(t, int64(12288), cardReq.FinalCardData.Downloaded_Size)
	})

	t.Run("large numbers calculation", func(t *testing.T) {
		cardReq := &dvCom.UpdateCardReq{
			FinalCardData: dvCom.CardData{
				DatasetInfos: []dvCom.DatasetInfo{
					{
						ConfigName: "large_config",
						Splits: []dvCom.Split{
							{
								Name:        "train",
								NumExamples: 1000000,
								Files: []dvCom.FileObject{
									{Size: 1024 * 1024 * 1024}, // 1GB
									{Size: 512 * 1024 * 1024},  // 512MB
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 2 * 1024 * 1024 * 1024}, // 2GB
									{DownloadSize: 1 * 1024 * 1024 * 1024}, // 1GB
								},
							},
						},
					},
				},
			},
		}

		dvActivity.CalcStatistics(cardReq)

		expectedConvertedSize := int64(1024*1024*1024 + 512*1024*1024)     // 1GB + 512MB = 1.5GB
		expectedDownloadSize := int64(2*1024*1024*1024 + 1*1024*1024*1024) // 2GB + 1GB = 3GB

		require.Equal(t, int64(1000000), cardReq.FinalCardData.Rows_Num)
		require.Equal(t, expectedConvertedSize, cardReq.FinalCardData.Converted_Size)
		require.Equal(t, expectedDownloadSize, cardReq.FinalCardData.Downloaded_Size)
	})

	t.Run("complex scenario with multiple configs and splits", func(t *testing.T) {
		cardReq := &dvCom.UpdateCardReq{
			FinalCardData: dvCom.CardData{
				DatasetInfos: []dvCom.DatasetInfo{
					{
						ConfigName: "english",
						Splits: []dvCom.Split{
							{
								Name:        "train",
								NumExamples: 50000,
								Files: []dvCom.FileObject{
									{Size: 100 * 1024 * 1024}, // 100MB
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 200 * 1024 * 1024}, // 200MB
								},
							},
							{
								Name:        "test",
								NumExamples: 10000,
								Files: []dvCom.FileObject{
									{Size: 20 * 1024 * 1024}, // 20MB
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 40 * 1024 * 1024}, // 40MB
								},
							},
						},
					},
					{
						ConfigName: "chinese",
						Splits: []dvCom.Split{
							{
								Name:        "train",
								NumExamples: 80000,
								Files: []dvCom.FileObject{
									{Size: 150 * 1024 * 1024}, // 150MB
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 300 * 1024 * 1024}, // 300MB
								},
							},
							{
								Name:        "validation",
								NumExamples: 20000,
								Files: []dvCom.FileObject{
									{Size: 30 * 1024 * 1024}, // 30MB
								},
								Origins: []dvCom.FileObject{
									{DownloadSize: 60 * 1024 * 1024}, // 60MB
								},
							},
						},
					},
				},
			},
		}

		dvActivity.CalcStatistics(cardReq)

		expectedRows := int64(50000 + 10000 + 80000 + 20000)                // 160000
		expectedConvertedSize := int64((100 + 20 + 150 + 30) * 1024 * 1024) // 300MB
		expectedDownloadSize := int64((200 + 40 + 300 + 60) * 1024 * 1024)  // 600MB

		require.Equal(t, expectedRows, cardReq.FinalCardData.Rows_Num)
		require.Equal(t, expectedConvertedSize, cardReq.FinalCardData.Converted_Size)
		require.Equal(t, expectedDownloadSize, cardReq.FinalCardData.Downloaded_Size)
	})
}
