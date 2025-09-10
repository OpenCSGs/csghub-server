package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	mock_comp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
)

func TestMCPScannerComponent_Scan(t *testing.T) {
	tests := []struct {
		name          string
		repoNameSpace string
		repoName      string
		files         []*types.File
		fileData      map[string]string
		issues        []types.ScannerIssue
		wantErr       bool
		wantCount     int
	}{
		{
			name:          "successful scan with critical issues",
			repoNameSpace: "mcpservers_test-namespace",
			repoName:      "test-repo",
			files: []*types.File{
				{Path: "file1.go", Name: "file1.go"},
				{Path: "file2.go", Name: "file2.go"},
			},
			fileData: map[string]string{
				"file1.go": "content1",
				"file2.go": "content2",
			},
			issues: []types.ScannerIssue{
				{Level: types.LevelCritical, Title: "Critical issue 1"},
				{Level: types.LevelCritical, Title: "Critical issue 2"},
			},

			wantCount: 2,
		},
	}
	ctx := context.TODO()
	scanner := initializeTestMCPScannerComponent(ctx, t)
	mockPlugin := mock_comp.NewMockMCPScannerPlugin(t)
	scanner.plugins = []MCPScannerPlugin{mockPlugin}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner.mocks.gitServer.EXPECT().GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
				Namespace: tt.repoNameSpace,
				Name:      tt.repoName,
				Ref:       "main",
				RepoType:  types.MCPServerRepo,
			}).Return(tt.files, nil)
			for _, file := range tt.files {
				req := gitserver.GetRepoInfoByPathReq{
					Namespace: tt.repoNameSpace,
					Name:      tt.repoName,
					Ref:       "main",
					Path:      file.Path,
					RepoType:  types.MCPServerRepo,
				}
				scanner.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, req).Return(tt.fileData[file.Path], nil)
			}
			mockPlugin.EXPECT().Check(ctx, tt.files).Return(tt.issues, nil)

			_, err := scanner.Scan(ctx, tt.repoNameSpace, tt.repoName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(tt.issues))
		})
	}
}
