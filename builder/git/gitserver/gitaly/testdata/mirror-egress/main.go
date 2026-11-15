// mirror-egress is an isolated Git HTTP origin and proxy trap for container tests.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

func main() {
	const root = "/tmp/source"
	const repo = root + "/repo.git"
	if err := os.MkdirAll(root, 0755); err != nil {
		log.Fatal(err)
	}
	git("init", "--bare", repo)
	tree := git("--git-dir="+repo, "mktree")
	commit := git("-c", "user.name=Test", "-c", "user.email=test@example.com", "--git-dir="+repo, "commit-tree", tree, "-m", "initial")
	git("--git-dir="+repo, "update-ref", "refs/heads/main", commit)
	git("--git-dir="+repo, "symbolic-ref", "HEAD", "refs/heads/main")
	git("--git-dir="+repo, "update-server-info")
	var proxyRequests, originRequests atomic.Int64
	go func() {
		files := http.FileServer(http.Dir(root))
		log.Fatal(http.ListenAndServe("127.0.0.1:80", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			originRequests.Add(1)
			files.ServeHTTP(w, r)
		})))
	}()
	go func() {
		log.Fatal(http.ListenAndServe("127.0.0.1:8080", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			proxyRequests.Add(1)
			http.Error(w, "unexpected proxy connection", http.StatusBadGateway)
		})))
	}()
	log.Fatal(http.ListenAndServe(":8081", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int64{"proxy": proxyRequests.Load(), "origin": originRequests.Load()})
	})))
}

func git(args ...string) string {
	cmd := exec.Command("/usr/local/bin/gitaly-git-v2.42", args...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("git %v: %v: %s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}
