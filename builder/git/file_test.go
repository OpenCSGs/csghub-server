package git

import (
	"testing"
)

// FIXME: this test should be done in gitea/gitaly package separately as unit test
// or start a real gitea/gitaly server as a e2e test
func TestDeleteRepoFile(t *testing.T) {

	// cfg, err := config.LoadConfig()
	// if err != nil {
	// 	t.Fatalf("failed to load config: %v", err)
	// }
	// cfg.GitServer.Type = types.GitServerTypeGitaly
	// git, err := NewGitServer(cfg)
	// if err != nil {
	// 	t.Fatalf("failed to create git server: %v", err)
	// }
	// req := types.DeleteFileReq{
	// 	Namespace:   "wanghh2003",
	// 	Name:        "gp2",
	// 	Branch:      types.MainBranch,
	// 	FilePath:    "aaa.jsonl",
	// 	Content:     "",
	// 	RepoType:    types.PromptRepo,
	// 	CurrentUser: "wanghh2003",
	// 	Username:    "wanghh2003",
	// 	Email:       "wanghh2003@163.com",
	// 	Message:     fmt.Sprintf("delete prompt %s", "aaa.jsonl"),
	// 	OriginPath:  "",
	// }
	// err = git.DeleteRepoFile(&req)
	// if err != nil {
	// 	t.Fatalf("failed to delete repo file: %v", err)
	// }
}
