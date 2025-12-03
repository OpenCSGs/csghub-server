package multisync

import (
	"testing"

	"opencsg.com/csghub-server/common/types"
)

func Test_repoTypeToURLPath(t *testing.T) {
	tests := []struct {
		name     string
		repoType types.RepositoryType
		want     string
	}{
		{name: "model", repoType: types.ModelRepo, want: "model"},
		{name: "dataset", repoType: types.DatasetRepo, want: "dataset"},
		{name: "code", repoType: types.CodeRepo, want: "code"},
		{name: "space", repoType: types.SpaceRepo, want: "space"},
		{name: "mcp", repoType: types.MCPServerRepo, want: "mcp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoTypeToURLPath(tt.repoType)
			if got != tt.want {
				t.Errorf("repoTypeToURLPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
