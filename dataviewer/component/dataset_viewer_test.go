package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func TestDatasetViewerComponent_ViewParquetFile(t *testing.T) {

	ctx := context.TODO()
	dc := initializeTestDatasetViewerComponent(ctx, t)

	repo := &database.Repository{DefaultBranch: "main"}
	dc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(
		repo, nil,
	)
	dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)

	dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      "foo",
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
	}, nil)

	dc.mocks.preader.EXPECT().RowCount(mock.Anything, []string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"}, types.QueryReq{
		PageSize:  10,
		PageIndex: 1,
	}, true).Return(100, nil)

	dc.mocks.preader.EXPECT().FetchRows(mock.Anything, []string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"}, types.QueryReq{
		PageSize:  10,
		PageIndex: 1,
	}, true).Return([]string{"a", "b"}, []string{"c", "d"}, [][]interface{}{
		{1, 2, 3},
	}, nil)

	resp, err := dc.ViewParquetFile(ctx, &dvCom.ViewParquetFileReq{
		Namespace:   "ns",
		RepoName:    "repo",
		Branch:      "main",
		Path:        "foo",
		Per:         10,
		Page:        1,
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, &dvCom.ViewParquetFileResp{
		Columns:     []string{"a", "b"},
		ColumnsType: []string{"c", "d"},
		Rows: [][]interface{}{
			{1, 2, 3},
		},
		Total: 100,
	}, resp)

}

func TestDatasetViewerComponent_Rows(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetViewerComponent(ctx, t)

	repo := &database.Repository{DefaultBranch: "main"}

	dc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(
		repo, nil,
	)
	dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)

	dc.mocks.stores.ViewerMock().EXPECT().GetViewerByRepoID(ctx, int64(0)).Return(nil, nil)

	dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
		Content:         "LS0tCmNvbmZpZ3M6Ci0gY29uZmlnX25hbWU6ICJmb28iCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgogICAgcGF0aDogZm9vLy4qCi0gY29uZmlnX25hbWU6ICJiYXIiCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgpwYXRoOiBiYXIvLioKLS0tCg==",
	}, nil)
	dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      "foo/foobar.parquet",
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
	}, nil)
	dc.mocks.preader.EXPECT().RowCount(mock.Anything, []string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"}, types.QueryReq{
		PageSize:  10,
		PageIndex: 1,
	}, true).Return(100, nil)
	dc.mocks.preader.EXPECT().FetchRows(mock.Anything, []string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"}, types.QueryReq{
		PageSize:  10,
		PageIndex: 1,
	}, true).Return([]string{"a", "b"}, []string{"c", "d"}, [][]interface{}{
		{1, 2, 3},
	}, nil)
	dc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "ns", Name: "repo", Ref: "main", Limit: 500, RepoType: "dataset", Recursive: true},
	).Return(&types.GetRepoFileTreeResp{Files: []*types.File{
		{
			Name:      "foobar.parquet",
			Path:      "foo/foobar.parquet",
			LfsSHA256: "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
		},
	}, Cursor: ""}, nil)

	resp, err := dc.Rows(ctx, &dvCom.ViewParquetFileReq{
		Namespace:   "ns",
		RepoName:    "repo",
		Branch:      "main",
		Path:        "foo",
		Per:         10,
		Page:        1,
		CurrentUser: "user",
	}, types.DataViewerReq{Split: "train", Config: "foo"})
	require.Nil(t, err)
	require.Equal(t, &dvCom.ViewParquetFileResp{
		Columns:     []string{"a", "b"},
		ColumnsType: []string{"c", "d"},
		Rows: [][]interface{}{
			{1, 2, 3},
		},
		Total: 100,
	}, resp)

}

func TestDatasetViewerComponent_LimitOffsetRowsNoCard(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetViewerComponent(ctx, t)

	repo := &database.Repository{DefaultBranch: "main"}

	dc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(
		repo, nil,
	)
	dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)

	dc.mocks.stores.ViewerMock().EXPECT().GetViewerByRepoID(ctx, int64(0)).Return(nil, nil)

	dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		Content:         "xxx",
		LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
	}, nil)
	dc.mocks.gitServer.EXPECT().GetTree(mock.Anything, types.GetTreeRequest{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      "foo",
		RepoType:  types.DatasetRepo,
		Limit:     500,
		Recursive: true,
	}).Return(
		&types.GetRepoFileTreeResp{Files: []*types.File{
			{
				Name:            "foobar.parquet",
				Path:            "train/foobar.parquet",
				LfsRelativePath: "a/b",
				LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
			},
		}}, nil,
	)

	dc.mocks.limitOffsetReader.EXPECT().RowsWithCount(ctx,
		[]string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"},
		int64(16), int64(48)).Return(
		[]string{"a", "b"},
		[]string{"c", "d"},
		[][]any{
			{1, 2, 3},
		}, 12000, nil,
	)

	resp, err := dc.LimitOffsetRows(ctx, &dvCom.ViewParquetFileReq{
		Namespace:   "ns",
		RepoName:    "repo",
		Branch:      "main",
		Path:        "foo",
		Per:         16,
		Page:        4,
		CurrentUser: "user",
	}, types.DataViewerReq{Split: "train", Config: "foo"})
	require.Nil(t, err)
	require.Equal(t, &dvCom.ViewParquetFileResp{
		Columns:     []string{"a", "b"},
		ColumnsType: []string{"c", "d"},
		Rows: [][]interface{}{
			{1, 2, 3},
		},
		Total: 12000,
	}, resp)
}

func TestDatasetViewerComponent_LimitOffsetRowsWithCard(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetViewerComponent(ctx, t)

	repo := &database.Repository{DefaultBranch: "main"}

	dc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(
		repo, nil,
	)
	dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)

	dc.mocks.stores.ViewerMock().EXPECT().GetViewerByRepoID(ctx, int64(0)).Return(nil, nil)

	dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		Content:         "LS0tCmNvbmZpZ3M6Ci0gY29uZmlnX25hbWU6ICJmb28iCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgogICAgcGF0aDogZm9vLy4qCi0gY29uZmlnX25hbWU6ICJiYXIiCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgpwYXRoOiBiYXIvLioKLS0tCg==",
		LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
	}, nil)
	dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "repo",
		Ref:       "main",
		Path:      "foo/foobar.parquet",
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		LfsRelativePath: "a/b",
		LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
	}, nil)

	dc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "ns", Name: "repo", RepoType: "dataset", Ref: "main", Limit: 500, Recursive: true},
	).Return(&types.GetRepoFileTreeResp{Files: []*types.File{
		{Name: "foobar.parquet", Path: "foo/foobar.parquet"},
	}, Cursor: ""}, nil)

	dc.mocks.limitOffsetReader.EXPECT().RowsWithCount(ctx,
		[]string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"},
		int64(16),
		int64(48)).Return(
		[]string{"a", "b"},
		[]string{"c", "d"},
		[][]any{
			{1, 2, 3},
		}, 12000, nil,
	)

	resp, err := dc.LimitOffsetRows(ctx, &dvCom.ViewParquetFileReq{
		Namespace:   "ns",
		RepoName:    "repo",
		Branch:      "main",
		Path:        "foo",
		Per:         16,
		Page:        4,
		CurrentUser: "user",
	}, types.DataViewerReq{Split: "train", Config: "foo"})
	require.Nil(t, err)
	require.Equal(t, &dvCom.ViewParquetFileResp{
		Columns:     []string{"a", "b"},
		ColumnsType: []string{"c", "d"},
		Rows: [][]interface{}{
			{1, 2, 3},
		},
		Total: 12000,
	}, resp)
}

func TestDatasetViewerComponent_GetCatalog(t *testing.T) {
	t.Run("has card data", func(t *testing.T) {

		ctx := context.TODO()
		dc := initializeTestDatasetViewerComponent(ctx, t)

		repo := &database.Repository{DefaultBranch: "main"}
		dc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(
			repo, nil,
		)
		dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)
		dc.mocks.stores.ViewerMock().EXPECT().GetViewerByRepoID(ctx, int64(0)).Return(nil, nil)
		dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
			Namespace: "ns",
			Name:      "repo",
			Ref:       "main",
			Path:      types.REPOCARD_FILENAME,
			RepoType:  types.DatasetRepo,
		}).Return(&types.File{
			LfsRelativePath: "a/b",
			Content:         "LS0tCmNvbmZpZ3M6Ci0gY29uZmlnX25hbWU6ICJmb28iCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgogICAgcGF0aDogZm9vLy4qCi0gY29uZmlnX25hbWU6ICJiYXIiCiAgZGF0YV9maWxlczoKICAtIHNwbGl0OiB0cmFpbgpwYXRoOiBiYXIvLioKLS0tCg==",
			LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
		}, nil)
		dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
			Namespace: "ns",
			Name:      "repo",
			Ref:       "main",
			Path:      "foo/foobar.parquet",
			RepoType:  types.DatasetRepo,
		}).Return(&types.File{
			LfsRelativePath: "a/b",
			LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
		}, nil)
		dc.mocks.preader.EXPECT().RowCount(mock.Anything,
			[]string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"},
			types.QueryReq{
				PageSize:  10,
				PageIndex: 1,
			}, true).Return(100, nil)

		dc.mocks.gitServer.EXPECT().GetTree(
			mock.Anything, types.GetTreeRequest{Namespace: "ns", Name: "repo", RepoType: "dataset", Ref: "main", Limit: 500, Recursive: true},
		).Return(&types.GetRepoFileTreeResp{Files: []*types.File{
			{
				Name:      "foobar.parquet",
				Path:      "foo/foobar.parquet",
				LfsSHA256: "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
			},
		}, Cursor: ""}, nil)

		data, err := dc.GetCatalog(ctx, &dvCom.ViewParquetFileReq{
			Namespace:   "ns",
			RepoName:    "repo",
			Branch:      "main",
			Path:        "foo",
			Per:         10,
			Page:        1,
			CurrentUser: "user",
		})
		require.Nil(t, err)
		require.Equal(t, &dvCom.CataLogRespone{
			Configs: []dvCom.ConfigData{
				{ConfigName: "foo", DataFiles: []dvCom.DataFiles{
					{Split: "train", Path: []string{"foo/.*"}},
				}}, {ConfigName: "bar", DataFiles: []dvCom.DataFiles{{Split: "train"}}}},
			DatasetInfos: []dvCom.DatasetInfo{
				{ConfigName: "foo", Splits: []dvCom.Split{{Name: "train", NumExamples: 100}}},
				{ConfigName: "bar", Splits: []dvCom.Split{{Name: "train", NumExamples: 0}}}},
		}, data)
	})

	t.Run("auto generate card data", func(t *testing.T) {

		ctx := context.TODO()
		dc := initializeTestDatasetViewerComponent(ctx, t)

		repo := &database.Repository{DefaultBranch: "main"}
		dc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "repo").Return(
			repo, nil,
		)
		dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)
		dc.mocks.stores.ViewerMock().EXPECT().GetViewerByRepoID(ctx, int64(0)).Return(nil, nil)
		dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
			Namespace: "ns",
			Name:      "repo",
			Ref:       "main",
			Path:      types.REPOCARD_FILENAME,
			RepoType:  types.DatasetRepo,
		}).Return(&types.File{
			LfsRelativePath: "a/b",
			Content:         "xxx",
			LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
		}, nil)
		dc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
			Namespace: "ns",
			Name:      "repo",
			Ref:       "main",
			Path:      "foo/train.parquet",
			RepoType:  types.DatasetRepo,
		}).Return(&types.File{
			LfsRelativePath: "a/b",
			LfsSHA256:       "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
		}, nil)
		dc.mocks.preader.EXPECT().RowCount(mock.Anything,
			[]string{"lfs/c5/18/5c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836"},
			types.QueryReq{
				PageSize:  10,
				PageIndex: 1,
			}, true).Return(100, nil)

		dc.mocks.gitServer.EXPECT().GetTree(
			mock.Anything, types.GetTreeRequest{Namespace: "ns", Name: "repo", RepoType: "dataset", Ref: "main", Limit: 500, Recursive: true},
		).Return(&types.GetRepoFileTreeResp{Files: []*types.File{
			{
				Name:      "foobar.parquet",
				Path:      "foo/train.parquet",
				LfsSHA256: "c5185c4794be2d8a9784d5753c9922db38df478ce11f9ed0b415b7304d896836",
			},
		}, Cursor: ""}, nil)

		data, err := dc.GetCatalog(ctx, &dvCom.ViewParquetFileReq{
			Namespace:   "ns",
			RepoName:    "repo",
			Branch:      "main",
			Path:        "foo",
			Per:         10,
			Page:        1,
			CurrentUser: "user",
		})
		require.Nil(t, err)
		require.Equal(t, &dvCom.CataLogRespone{Configs: []dvCom.ConfigData{{ConfigName: "default", DataFiles: []dvCom.DataFiles{{Split: "train", Path: []string{"foo/train.parquet"}}}}}, DatasetInfos: []dvCom.DatasetInfo{{ConfigName: "default", Splits: []dvCom.Split{{Name: "train", NumExamples: 100}}}}}, data)
	})
}
