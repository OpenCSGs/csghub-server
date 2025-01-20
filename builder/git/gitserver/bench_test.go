package gitserver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/git/gitserver/gitea"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// This benchmark evaluates the performance of the new `tree` and `logs_tree` APIs compared to
// the existing repository file tree API. The current API becomes inefficient with large file lists
// (e.g., hundreds of files).
// To run this benchmark, you must first set up the Gitaly/Gitea environment and add the SQLite repository.

var gitalyBench = false
var gitbenchNamespace = "sqlite"
var gitbenchName = ""

func newGitClient(b *testing.B) gitserver.GitServer {
	b.Skip("this benchmark needs gitaly/gitea")
	if gitalyBench {
		config := &config.Config{}
		config.GitalyServer.Address = "tcp://localhost:9999"
		config.GitalyServer.Storge = "default"
		client, err := gitaly.NewClient(config)
		require.NoError(b, err)
		return client
	} else {
		gitbenchNamespace = "giteatea"
		gitbenchName = "sqlite"
		config := &config.Config{}
		config.GitServer.Host = "http://localhost:3030"
		client, err := gitea.NewClient(config)
		require.NoError(b, err)
		return client
	}
}

func BenchmarkGitServerTree_GetRepoFile(b *testing.B) {
	client := newGitClient(b)

	cases := []string{"ext/fts3", "tool", "test"}

	for _, c := range cases {
		b.Run(c, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := client.GetRepoFileTree(context.TODO(), gitserver.GetRepoInfoByPathReq{
					Namespace: gitbenchNamespace,
					Name:      gitbenchName,
					Ref:       "master",
					Path:      c,
				})
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkGitServerTree_GetTree(b *testing.B) {
	client := newGitClient(b)

	cases := []string{"ext/fts3", "tool", "test"}

	for _, c := range cases {
		b.Run(c, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := client.GetTree(context.TODO(), types.GetTreeRequest{
					Namespace: gitbenchNamespace,
					Name:      gitbenchName,
					Ref:       "master",
					Path:      c,
					Limit:     500,
				})
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkGitServerTree_LogsTree(b *testing.B) {
	client := newGitClient(b)

	cases := []string{"ext/fts3", "tool", "test"}

	for _, c := range cases {
		b.Run(c, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := client.GetLogsTree(context.TODO(), types.GetLogsTreeRequest{
					Namespace: gitbenchNamespace,
					Name:      gitbenchName,
					Ref:       "master",
					Path:      c,
					Limit:     30,
				})
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkGitServerTree_GetTreeAndLogsTree(b *testing.B) {
	client := newGitClient(b)

	cases := []string{"ext/fts3", "tool", "test"}

	for _, c := range cases {
		b.Run(c, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := client.GetTree(context.TODO(), types.GetTreeRequest{
					Namespace: gitbenchNamespace,
					Name:      gitbenchName,
					Ref:       "master",
					Path:      c,
					Limit:     500,
				})
				require.NoError(b, err)
				_, err = client.GetLogsTree(context.TODO(), types.GetLogsTreeRequest{
					Namespace: gitbenchNamespace,
					Name:      gitbenchName,
					Ref:       "master",
					Path:      c,
					Limit:     30,
				})
				require.NoError(b, err)
			}
		})
	}
}
